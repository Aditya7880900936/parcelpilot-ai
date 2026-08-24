CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================================
-- Accounts
-- ============================================================

CREATE TABLE accounts (
    account_id          TEXT PRIMARY KEY,
    account_name        TEXT NOT NULL,
    plan                TEXT NOT NULL CHECK (plan IN ('Standard', 'Growth', 'Enterprise')),
    status              TEXT NOT NULL,
    csm                 TEXT,
    contract_file       TEXT,
    premium_support     BOOLEAN NOT NULL DEFAULT FALSE,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Orders
-- ============================================================

CREATE TABLE orders (
    order_id                    TEXT PRIMARY KEY,
    account_id                  TEXT NOT NULL REFERENCES accounts(account_id),
    carrier                     TEXT NOT NULL,
    status                      TEXT NOT NULL CHECK (
        status IN ('DRAFT', 'BOOKED', 'PICKED_UP', 'DELIVERED')
    ),
    booked_at                   TIMESTAMPTZ,
    pickup_window_start         TIMESTAMPTZ,
    pickup_window_end           TIMESTAMPTZ,
    pickup_actual_at            TIMESTAMPTZ,
    shipment_fee_inr            NUMERIC(12,2) NOT NULL,
    carrier_fault               BOOLEAN NOT NULL DEFAULT FALSE,
    customer_fault              BOOLEAN NOT NULL DEFAULT FALSE,
    cancellation_requested_at   TIMESTAMPTZ,
    notes                       TEXT,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_account_id
    ON orders(account_id);

CREATE INDEX idx_orders_status
    ON orders(status);

-- ============================================================
-- Tickets
-- ============================================================

CREATE TABLE tickets (
    ticket_id               TEXT PRIMARY KEY,
    account_id              TEXT NOT NULL REFERENCES accounts(account_id),
    created_at              TIMESTAMPTZ NOT NULL,
    status                  TEXT NOT NULL,
    subject                 TEXT NOT NULL,
    description             TEXT NOT NULL,
    channel                 TEXT,
    assigned_to             TEXT,
    last_customer_message_at TIMESTAMPTZ,
    historical_resolution   TEXT,
    created_record_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_account_id
    ON tickets(account_id);

CREATE INDEX idx_tickets_status
    ON tickets(status);

CREATE INDEX idx_tickets_created_at
    ON tickets(created_at);

-- ============================================================
-- Documents
-- ============================================================

CREATE TABLE documents (
    id                  BIGSERIAL PRIMARY KEY,
    filename            TEXT NOT NULL UNIQUE,
    title               TEXT NOT NULL,
    document_type       TEXT NOT NULL,
    status              TEXT NOT NULL,
    authority_rank      INTEGER NOT NULL,
    account_id          TEXT REFERENCES accounts(account_id),
    effective_from      DATE,
    effective_to        DATE,
    source_updated_at   DATE,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_account_id
    ON documents(account_id);

CREATE INDEX idx_documents_authority
    ON documents(authority_rank);

-- ============================================================
-- Document chunks / RAG
-- ============================================================

CREATE TABLE document_chunks (
    id              BIGSERIAL PRIMARY KEY,
    document_id     BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index     INTEGER NOT NULL,
    content         TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    embedding       VECTOR(1536),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(document_id, chunk_index)
);

CREATE INDEX idx_document_chunks_document_id
    ON document_chunks(document_id);

-- Vector index will be added after embeddings are populated.
-- CREATE INDEX idx_document_chunks_embedding
-- ON document_chunks
-- USING hnsw (embedding vector_cosine_ops);

-- ============================================================
-- Escalations / State-changing actions
-- ============================================================

CREATE TABLE escalations (
    id                  BIGSERIAL PRIMARY KEY,
    ticket_id           TEXT NOT NULL REFERENCES tickets(ticket_id),
    account_id          TEXT NOT NULL REFERENCES accounts(account_id),
    reason              TEXT NOT NULL,
    priority            TEXT NOT NULL CHECK (
        priority IN ('P1', 'P2', 'P3')
    ),
    status              TEXT NOT NULL DEFAULT 'pending'
                        CHECK (
                            status IN (
                                'pending',
                                'confirmed',
                                'created',
                                'cancelled'
                            )
                        ),
    requested_by        TEXT,
    confirmed_by        TEXT,
    confirmed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_escalations_ticket_id
    ON escalations(ticket_id);

CREATE INDEX idx_escalations_account_id
    ON escalations(account_id);

CREATE INDEX idx_escalations_status
    ON escalations(status);