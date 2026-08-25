-- ============================================================
-- Audit Logs
-- ============================================================

CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(account_id),
    action_type     TEXT NOT NULL,
    target_id       TEXT NOT NULL,
    reason          TEXT NOT NULL,
    previous_state  JSONB,
    new_state       JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_account_id
    ON audit_logs(account_id);

CREATE INDEX idx_audit_logs_target_id
    ON audit_logs(target_id);

CREATE INDEX idx_audit_logs_created_at
    ON audit_logs(created_at);
