package action

import (
	"context"
	"testing"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/agent"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDatabaseURL = "postgres://parcelpilot:parcelpilot@localhost:5432/parcelpilot_test"

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatalf("failed to create test pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func resetTestData(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		DELETE FROM audit_logs
		WHERE target_id IN (
			'TEST-CANCEL',
			'TEST-RTO',
			'TEST-RTO-DONE'
		)
	`)
	if err != nil {
		t.Fatalf("failed to clean audit logs: %v", err)
	}

	_, err = pool.Exec(ctx, `
		UPDATE orders
		SET status = CASE order_id
			WHEN 'TEST-CANCEL' THEN 'BOOKED'
			WHEN 'TEST-RTO' THEN 'PICKED_UP'
			WHEN 'TEST-RTO-DONE' THEN 'RETURN_TO_ORIGIN'
		END,
		updated_at = NOW()
		WHERE order_id IN (
			'TEST-CANCEL',
			'TEST-RTO',
			'TEST-RTO-DONE'
		)
	`)
	if err != nil {
		t.Fatalf("failed to reset orders: %v", err)
	}
}

func TestExecutorCancelBookedOrder(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "CANCEL_ORDER",
			Target: "TEST-CANCEL",
			Reason: "test cancellation",
		},
	)

	if err != nil {
		t.Fatalf("expected cancellation to succeed: %v", err)
	}

	var status string

	err = pool.QueryRow(
		context.Background(),
		`SELECT status FROM orders WHERE order_id = 'TEST-CANCEL'`,
	).Scan(&status)

	if err != nil {
		t.Fatalf("failed to read order status: %v", err)
	}

	if status != "CANCELLED" {
		t.Fatalf("expected CANCELLED, got %s", status)
	}
}

func TestExecutorReturnToOrigin(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "RETURN_TO_ORIGIN",
			Target: "TEST-RTO",
			Reason: "customer requested return",
		},
	)

	if err != nil {
		t.Fatalf("expected return-to-origin to succeed: %v", err)
	}

	var status string

	err = pool.QueryRow(
		context.Background(),
		`SELECT status FROM orders WHERE order_id = 'TEST-RTO'`,
	).Scan(&status)

	if err != nil {
		t.Fatalf("failed to read order status: %v", err)
	}

	if status != "RETURN_TO_ORIGIN" {
		t.Fatalf("expected RETURN_TO_ORIGIN, got %s", status)
	}
}

func TestExecutorReturnToOriginIsIdempotent(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "RETURN_TO_ORIGIN",
			Target: "TEST-RTO-DONE",
			Reason: "already returned",
		},
	)

	if err != nil {
		t.Fatalf("expected idempotent action to succeed: %v", err)
	}

	var status string

	err = pool.QueryRow(
		context.Background(),
		`SELECT status FROM orders WHERE order_id = 'TEST-RTO-DONE'`,
	).Scan(&status)

	if err != nil {
		t.Fatalf("failed to read order status: %v", err)
	}

	if status != "RETURN_TO_ORIGIN" {
		t.Fatalf("expected RETURN_TO_ORIGIN, got %s", status)
	}
}

func TestExecutorRejectsInvalidCancellationState(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "CANCEL_ORDER",
			Target: "TEST-RTO",
			Reason: "invalid cancellation",
		},
	)

	if err == nil {
		t.Fatal("expected cancellation of PICKED_UP order to fail")
	}
}

func TestExecutorRejectsInvalidReturnState(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "RETURN_TO_ORIGIN",
			Target: "TEST-CANCEL",
			Reason: "invalid return",
		},
	)

	if err == nil {
		t.Fatal("expected return-to-origin of CANCELLED order to fail")
	}
}

func TestExecutorCancelCreatesAuditLog(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "CANCEL_ORDER",
			Target: "TEST-CANCEL",
			Reason: "audit test cancellation",
		},
	)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	var (
		actionType    string
		targetID      string
		previousState string
		newState      string
		reason        string
	)

	err = pool.QueryRow(context.Background(), `
		SELECT
			action_type,
			target_id,
			previous_state::text,
			new_state::text,
			reason
		FROM audit_logs
		WHERE target_id = 'TEST-CANCEL'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(
		&actionType,
		&targetID,
		&previousState,
		&newState,
		&reason,
	)

	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if actionType != "CANCEL_ORDER" {
		t.Fatalf("expected CANCEL_ORDER, got %s", actionType)
	}

	if targetID != "TEST-CANCEL" {
		t.Fatalf("expected TEST-CANCEL, got %s", targetID)
	}

	if previousState != `{"status": "BOOKED"}` {
		t.Fatalf("unexpected previous state: %s", previousState)
	}

	if newState != `{"status": "CANCELLED"}` {
		t.Fatalf("unexpected new state: %s", newState)
	}

	if reason != "audit test cancellation" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestExecutorReturnToOriginCreatesAuditLog(t *testing.T) {
	pool := setupTestDB(t)
	resetTestData(t, pool)

	executor := NewExecutor(pool)

	err := executor.Execute(
		context.Background(),
		&agent.Action{
			Type:   "RETURN_TO_ORIGIN",
			Target: "TEST-RTO",
			Reason: "audit test return",
		},
	)
	if err != nil {
		t.Fatalf("return-to-origin failed: %v", err)
	}

	var (
		actionType    string
		targetID      string
		previousState string
		newState      string
		reason        string
	)

	err = pool.QueryRow(context.Background(), `
		SELECT
			action_type,
			target_id,
			previous_state::text,
			new_state::text,
			reason
		FROM audit_logs
		WHERE target_id = 'TEST-RTO'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(
		&actionType,
		&targetID,
		&previousState,
		&newState,
		&reason,
	)

	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if actionType != "RETURN_TO_ORIGIN" {
		t.Fatalf("expected RETURN_TO_ORIGIN, got %s", actionType)
	}

	if targetID != "TEST-RTO" {
		t.Fatalf("expected TEST-RTO, got %s", targetID)
	}

	if previousState != `{"status": "PICKED_UP"}` {
		t.Fatalf("unexpected previous state: %s", previousState)
	}

	if newState != `{"status": "RETURN_TO_ORIGIN"}` {
		t.Fatalf("unexpected new state: %s", newState)
	}

	if reason != "audit test return" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestExecutorEscalateCreatesEscalation(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		DELETE FROM escalations;
		DELETE FROM tickets WHERE ticket_id = 'TEST-TICKET';
	`)

	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO tickets (
			ticket_id,
			account_id,
			created_at,
			status,
			subject,
			description
		)
		VALUES (
			'TEST-TICKET',
			'TEST-001',
			NOW(),
			'open',
			'Test escalation',
			'Test escalation description'
		)
	`)

	if err != nil {
		t.Fatal(err)
	}

	e := NewExecutor(pool)

	err = e.Execute(ctx, &agent.Action{
		Type:   "ESCALATE",
		Target: "TEST-TICKET",
		Reason: "test escalation",
	})

	if err != nil {
		t.Fatalf("expected escalation to succeed: %v", err)
	}

	var (
		priority    string
		status      string
		requestedBy string
	)

	err = pool.QueryRow(ctx, `
		SELECT priority, status, requested_by
		FROM escalations
		WHERE ticket_id = 'TEST-TICKET'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&priority, &status, &requestedBy)

	if err != nil {
		t.Fatal(err)
	}

	if priority != "P1" {
		t.Fatalf("expected P1, got %s", priority)
	}

	if status != "created" {
		t.Fatalf("expected created, got %s", status)
	}

	if requestedBy != "agent" {
		t.Fatalf("expected agent, got %s", requestedBy)
	}
}

func TestExecutorEscalateCreatesAuditLog(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		DELETE FROM audit_logs
		WHERE target_id = 'TEST-TICKET'
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
		DELETE FROM escalations
		WHERE ticket_id = 'TEST-TICKET'
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO tickets (
			ticket_id,
			account_id,
			created_at,
			status,
			subject,
			description
		)
		VALUES (
			'TEST-TICKET',
			'TEST-001',
			NOW(),
			'open',
			'Test escalation',
			'Test escalation description'
		)
		ON CONFLICT (ticket_id) DO NOTHING
	`)
	if err != nil {
		t.Fatal(err)
	}

	executor := NewExecutor(pool)

	err = executor.Execute(ctx, &agent.Action{
		Type:   "ESCALATE",
		Target: "TEST-TICKET",
		Reason: "audit test escalation",
	})
	if err != nil {
		t.Fatalf("escalation failed: %v", err)
	}

	var (
		actionType    string
		targetID      string
		reason        string
		previousState string
		newState      string
	)

	err = pool.QueryRow(ctx, `
		SELECT
			action_type,
			target_id,
			reason,
			previous_state::text,
			new_state::text
		FROM audit_logs
		WHERE target_id = 'TEST-TICKET'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(
		&actionType,
		&targetID,
		&reason,
		&previousState,
		&newState,
	)
	if err != nil {
		t.Fatal(err)
	}

	if actionType != "ESCALATE" {
		t.Fatalf("expected ESCALATE, got %s", actionType)
	}

	if targetID != "TEST-TICKET" {
		t.Fatalf("expected TEST-TICKET, got %s", targetID)
	}

	if reason != "audit test escalation" {
		t.Fatalf("unexpected reason: %s", reason)
	}

	if previousState != `{}` {
		t.Fatalf("unexpected previous state: %s", previousState)
	}

	if newState != `{"status": "created", "priority": "P1", "requested_by": "agent"}` {
		t.Fatalf("unexpected new state: %s", newState)
	}
}

func TestExecutorServiceCreditCreatesCredit(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()

	// Clean previous test credit.
	_, err := pool.Exec(ctx, `
		DELETE FROM service_credits
		WHERE order_id = 'TEST-CREDIT'
	`)
	if err != nil {
		t.Fatalf("failed to clean service credit: %v", err)
	}

	// Ensure test order exists.
	_, err = pool.Exec(ctx, `
INSERT INTO orders (
    order_id,
    account_id,
    status,
    carrier,
    shipment_fee_inr,
    created_at,
    updated_at
)
VALUES (
    'TEST-CREDIT',
    'TEST-001',
    'BOOKED',
    'TEST-CARRIER',
    1200,
    NOW(),
    NOW()
)
ON CONFLICT (order_id) DO UPDATE
SET
    account_id = EXCLUDED.account_id,
    status = 'BOOKED',
    carrier = EXCLUDED.carrier,
    shipment_fee_inr = EXCLUDED.shipment_fee_inr,
    updated_at = NOW()
`)
	if err != nil {
		t.Fatalf("failed to prepare test order: %v", err)
	}

	executor := NewExecutor(pool)

	err = executor.Execute(ctx, &agent.Action{
		Type:   "SERVICE_CREDIT",
		Target: "TEST-CREDIT",
		Reason: "carrier delayed pickup",
	})
	if err != nil {
		t.Fatalf("expected service credit to succeed: %v", err)
	}

	var (
		accountID   string
		amount      float64
		reason      string
		status      string
		requestedBy string
	)

	err = pool.QueryRow(ctx, `
		SELECT
			account_id,
			amount_inr,
			reason,
			status,
			requested_by
		FROM service_credits
		WHERE order_id = 'TEST-CREDIT'
		LIMIT 1
	`).Scan(
		&accountID,
		&amount,
		&reason,
		&status,
		&requestedBy,
	)

	if err != nil {
		t.Fatalf("failed to read service credit: %v", err)
	}

	if accountID != "TEST-001" {
		t.Fatalf("expected account TEST-001, got %s", accountID)
	}

	if amount <= 0 {
		t.Fatalf("expected positive credit amount, got %.2f", amount)
	}

	if reason != "carrier delayed pickup" {
		t.Fatalf("unexpected reason: %s", reason)
	}

	if status != "ISSUED" {
		t.Fatalf("expected ISSUED, got %s", status)
	}

	if requestedBy != "agent" {
		t.Fatalf("expected agent, got %s", requestedBy)
	}
}
