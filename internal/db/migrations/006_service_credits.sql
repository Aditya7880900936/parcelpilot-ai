-- internal/db/migrations/006_service_credits.sql

CREATE TABLE IF NOT EXISTS service_credits (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(order_id),
    account_id TEXT NOT NULL REFERENCES accounts(account_id),
    amount_inr NUMERIC(12, 2) NOT NULL CHECK (amount_inr > 0),
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ISSUED'
        CHECK (status IN ('PENDING_APPROVAL', 'APPROVED', 'ISSUED', 'REJECTED')),
    requested_by TEXT NOT NULL DEFAULT 'agent',
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    issued_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_service_credits_active_order
ON service_credits(order_id)
WHERE status IN ('PENDING_APPROVAL', 'APPROVED', 'ISSUED');

CREATE INDEX IF NOT EXISTS idx_service_credits_account
ON service_credits(account_id);

CREATE INDEX IF NOT EXISTS idx_service_credits_status
ON service_credits(status);