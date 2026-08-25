package action

import (
	"context"
	"fmt"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/agent"
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
		return e.cancelOrder(ctx, act.Target)

	case "RETURN_TO_ORIGIN":
		return fmt.Errorf("return-to-origin action is not implemented yet")

	case "ESCALATE":
		return fmt.Errorf("escalation action is not implemented yet")

	default:
		return fmt.Errorf("unsupported action type: %s", act.Type)
	}
}

func (e *Executor) cancelOrder(
	ctx context.Context,
	orderID string,
) error {
	if orderID == "" {
		return fmt.Errorf("order ID is required")
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cancellation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM orders
		WHERE order_id = $1
		FOR UPDATE
	`, orderID).Scan(&status)

	if err != nil {
		return fmt.Errorf("load order %s: %w", orderID, err)
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
		`, orderID)

		if err != nil {
			return fmt.Errorf("cancel order %s: %w", orderID, err)
		}

	case "CANCELLED":
		// Idempotent cancellation.
		return nil

	default:
		return fmt.Errorf(
			"cannot cancel order %s in status %s",
			orderID,
			status,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancellation: %w", err)
	}

	return nil
}
