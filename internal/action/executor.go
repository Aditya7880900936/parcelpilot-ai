package action

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/agent"
	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Executor struct {
	pool *pgxpool.Pool
}

func NewExecutor(pool *pgxpool.Pool) *Executor {
	return &Executor{pool: pool}
}

func (e *Executor) Execute(
	ctx context.Context,
	act *agent.Action,
) error {
	if act == nil {
		return fmt.Errorf("action is nil")
	}

	switch act.Type {
	case "CANCEL_ORDER":
		return e.cancelOrder(ctx, act)

	case "RETURN_TO_ORIGIN":
		return e.returnToOrigin(ctx, act)

	case "ESCALATE":
		return e.escalate(ctx, act)

	default:
		return fmt.Errorf("unsupported action type: %s", act.Type)
	}
}

func (e *Executor) cancelOrder(
	ctx context.Context,
	act *agent.Action,
) error {
	if act == nil {
		return fmt.Errorf("action is nil")
	}

	if act.Target == "" {
		return fmt.Errorf("order ID is required")
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cancellation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		status    string
		accountID string
	)

	err = tx.QueryRow(ctx, `
		SELECT status, account_id
		FROM orders
		WHERE order_id = $1
		FOR UPDATE
	`, act.Target).Scan(&status, &accountID)

	if err != nil {
		return fmt.Errorf("load order %s: %w", act.Target, err)
	}

	switch status {
	case "BOOKED":
		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET
				status = 'CANCELLED',
				cancellation_requested_at = NOW(),
				updated_at = NOW()
			WHERE order_id = $1
		`, act.Target)

		if err != nil {
			return fmt.Errorf("cancel order %s: %w", act.Target, err)
		}

		auditRepo := db.NewAuditLogRepository(tx)

		err = auditRepo.Create(
			ctx,
			accountID,
			act.Type,
			act.Target,
			act.Reason,
			map[string]any{
				"status": "BOOKED",
			},
			map[string]any{
				"status": "CANCELLED",
			},
		)
		if err != nil {
			return fmt.Errorf("audit cancellation: %w", err)
		}

	case "CANCELLED":
		// Idempotent cancellation.
		return nil

	default:
		return fmt.Errorf(
			"cannot cancel order %s in status %s",
			act.Target,
			status,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancellation: %w", err)
	}

	return nil
}

func (e *Executor) returnToOrigin(
	ctx context.Context,
	act *agent.Action,
) error {
	if act == nil {
		return fmt.Errorf("action is nil")
	}

	if act.Target == "" {
		return fmt.Errorf("order ID is required")
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin return-to-origin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		status    string
		accountID string
	)

	err = tx.QueryRow(ctx, `
        SELECT status, account_id
        FROM orders
        WHERE order_id = $1
        FOR UPDATE
    `, act.Target).Scan(&status, &accountID)

	if err != nil {
		return fmt.Errorf("load order %s: %w", act.Target, err)
	}

	switch status {
	case "PICKED_UP":
		_, err = tx.Exec(ctx, `
            UPDATE orders
            SET
                status = 'RETURN_TO_ORIGIN',
                updated_at = NOW()
            WHERE order_id = $1
        `, act.Target)

		if err != nil {
			return fmt.Errorf(
				"return order %s to origin: %w",
				act.Target,
				err,
			)
		}

		auditRepo := db.NewAuditLogRepository(tx)

		err = auditRepo.Create(
			ctx,
			accountID,
			act.Type,
			act.Target,
			act.Reason,
			map[string]any{
				"status": "PICKED_UP",
			},
			map[string]any{
				"status": "RETURN_TO_ORIGIN",
			},
		)
		if err != nil {
			return fmt.Errorf("audit return-to-origin: %w", err)
		}

	case "RETURN_TO_ORIGIN":
		// Idempotent return-to-origin.

	default:
		return fmt.Errorf(
			"cannot return order %s to origin in status %s",
			act.Target,
			status,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit return-to-origin: %w", err)
	}

	return nil
}

func (e *Executor) escalate(
	ctx context.Context,
	act *agent.Action,
) error {
	if act.Target == "" {
		return fmt.Errorf("ticket ID is required for escalation")
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin escalation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var accountID string

	err = tx.QueryRow(ctx, `
        SELECT account_id
        FROM tickets
        WHERE ticket_id = $1
    `, act.Target).Scan(&accountID)

	if err != nil {
		return fmt.Errorf("load ticket %s: %w", act.Target, err)
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO escalations (
            ticket_id,
            account_id,
            reason,
            priority,
            status,
            requested_by
        )
        VALUES ($1, $2, $3, 'P1', 'created', 'agent')
    `,
		act.Target,
		accountID,
		act.Reason,
	)

	auditRepo := db.NewAuditLogRepository(tx)

	err = auditRepo.Create(
		ctx,
		accountID,
		act.Type,
		act.Target,
		act.Reason,
		map[string]any{},
		map[string]any{
			"priority":     "P1",
			"status":       "created",
			"requested_by": "agent",
		},
	)
	if err != nil {
		return fmt.Errorf("audit escalation: %w", err)
	}

	if err != nil {
		return fmt.Errorf("create escalation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit escalation: %w", err)
	}

	return nil
}
