package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AuditLogRepository struct {
	tx pgx.Tx
}

func NewAuditLogRepository(tx pgx.Tx) *AuditLogRepository {
	return &AuditLogRepository{tx: tx}
}

func (r *AuditLogRepository) Create(
	ctx context.Context,
	accountID string,
	actionType string,
	targetID string,
	reason string,
	previousState any,
	newState any,
) error {
	previousJSON, err := json.Marshal(previousState)
	if err != nil {
		return fmt.Errorf("marshal previous state: %w", err)
	}

	newJSON, err := json.Marshal(newState)
	if err != nil {
		return fmt.Errorf("marshal new state: %w", err)
	}

	_, err = r.tx.Exec(ctx, `
		INSERT INTO audit_logs (
			account_id,
			action_type,
			target_id,
			reason,
			previous_state,
			new_state
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		accountID,
		actionType,
		targetID,
		reason,
		previousJSON,
		newJSON,
	)

	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}

	return nil
}
