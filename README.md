# ParcelPilot AI

ParcelPilot AI is an agent-driven logistics support and operations system built in Go.

It combines:

- Structured PostgreSQL operational data
- Document retrieval with embeddings
- Customer-specific policy evaluation
- Deterministic decision making
- Safe state-changing actions
- Audit logging
- REST API evaluation flow

The system is designed around a simple principle:

> Use AI/retrieval for context, but use deterministic business rules for actions.

---

## Architecture

```text
                         ┌─────────────────────┐
                         │     Client / API     │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │     Orchestrator    │
                         └──────────┬──────────┘
                                    │
                 ┌──────────────────┼──────────────────┐
                 ▼                  ▼                  ▼
        ┌────────────────┐ ┌────────────────┐ ┌────────────────┐
        │   Retriever    │ │ Context Loader │ │ Decision Engine│
        │ PostgreSQL +   │ │ Orders/Account │ │ Deterministic  │
        │ embeddings     │ │ Tickets        │ │ business rules │
        └────────────────┘ └────────────────┘ └───────┬────────┘
                                                       │
                                                       ▼
                                              ┌─────────────────┐
                                              │ Action Executor │
                                              └────────┬────────┘
                                                       │
                                      ┌────────────────┼───────────────┐
                                      ▼                ▼               ▼
                                  Orders          Escalations      Audit Logs
```

## Current Capabilities

### Agent / Decision Engine

The agent evaluates operational requests using:

- Account context
- Order context
- Ticket context
- Retrieved policy documents
- Customer-specific agreements

Supported deterministic decisions include:

- Cancel BOOKED orders
- Reject invalid cancellation states
- Return PICKED_UP orders to origin
- Handle already-returned orders idempotently
- Reject cancellation of DELIVERED orders
- Detect already-cancelled orders
- Detect conflicting policy evidence
- Escalate suspected API key exposure as P1

### Action Execution

Supported actions:

#### CANCEL_ORDER

Cancels a valid BOOKED order and creates an audit record.

```text
BOOKED
   │
   ▼
CANCEL_ORDER
   │
   ▼
CANCELLED
```

#### RETURN_TO_ORIGIN

Moves a PICKED_UP order into the return-to-origin workflow.

```text
PICKED_UP
   │
   ▼
RETURN_TO_ORIGIN
   │
   ▼
RETURN_TO_ORIGIN
```

The action is idempotent when the order is already in the return-to-origin state.

#### ESCALATE

Creates an escalation record for a ticket.

Current P1 security escalation includes suspected API key exposure.

### Audit Logging

State-changing actions are recorded in PostgreSQL.

Audit records contain:

- Action type
- Target ID
- Previous state
- New state
- Reason
- Timestamp

Example:

```text
CANCEL_ORDER
ORD-2001

previous:
{"status": "BOOKED"}

new:
{"status": "CANCELLED"}
```

### Retrieval Pipeline

Documents are ingested and converted into searchable chunks.

```text
Document
   │
   ▼
Extraction
   │
   ▼
Chunking
   │
   ▼
Embedding
   │
   ▼
PostgreSQL
   │
   ▼
Similarity Retrieval
```

Ollama is currently used for local embeddings.

Default embedding model:

```text
nomic-embed-text
```

### Agent Evaluation Flow

For an incoming request:

```text
User Query
    │
    ▼
Retrieve relevant documents
    │
    ▼
Load account/order/ticket context
    │
    ▼
Build agent context
    │
    ▼
Run deterministic decision engine
    │
    ├── Answer only
    │
    ├── Escalate
    │
    └── Execute safe action
```

State-changing actions are only executed when the decision meets the configured confidence/safety conditions.

### REST API

The project exposes an evaluation endpoint:

```text
POST /evaluate
Content-Type: application/json
```

Example:

```json
{
  "query": "Can Northstar cancel ORD-2002?",
  "account_id": "ACCT-001",
  "order_id": "ORD-2002"
}
```

Example response:

```json
{
  "answer": "Northstar Logistics may cancel BOOKED order ORD-2002 before pickup with no cancellation fee under the active customer agreement",
  "confidence": 0.95,
  "action": {
    "Type": "CANCEL_ORDER",
    "Target": "ORD-2002",
    "Reason": "active customer agreement overrides the default cancellation fee policy"
  },
  "escalate": false
}
```

## Project Structure

```text
cmd/
├── embed/
├── ingest/
├── inspect/
├── pdfingest/
├── retrieve/
└── server/

internal/
├── action/
│   ├── executor.go
│   └── executor_test.go
│
├── agent/
│   ├── agent.go
│   ├── context.go
│   ├── context_builder.go
│   ├── decision.go
│   ├── decision_test.go
│   ├── orchestrator.go
│   ├── orchestrator_test.go
│   └── retriever.go
│
├── db/
│   ├── account_context.go
│   ├── audit_logs.go
│   ├── context_loader.go
│   ├── document_chunks.go
│   ├── documents.go
│   └── migrations/
│
├── document/
├── domain/
├── embeddings/
└── retrieval/
```

## Database

PostgreSQL stores:

- Accounts
- Orders
- Tickets
- Documents
- Document chunks
- Embeddings
- Audit logs
- Escalations

Migrations currently include:

```text
001_init.sql
002_embedding_dimension.sql
003_audit_logs.sql
004_return_to_origin.sql
```

## Local Development

### Requirements

- Go 1.25+
- Docker
- Docker Compose
- PostgreSQL
- Ollama

### Start infrastructure

```bash
docker compose up -d
```

### Configure environment

Create `.env` with the required database and Ollama configuration.

Example:

```text
DATABASE_URL=postgres://parcelpilot:parcelpilot@localhost:5432/parcelpilot

OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_EMBEDDING_MODEL=nomic-embed-text
```

### Run tests

```bash
go test ./...
```

### Run the API

```bash
go run ./cmd/server
```

### Run the evaluation CLI

```bash
go run ./cmd/retrieve
```

## Testing

The project currently includes tests for:

- Order cancellation
- Return-to-origin
- Idempotent return-to-origin
- Invalid state transitions
- Audit log creation
- Escalation creation
- Decision engine behavior
- Orchestrator behavior
- Empty query validation

Run the complete suite:

```bash
go test ./...
```

## Safety Model

ParcelPilot AI does not allow retrieved text alone to directly mutate operational state.

The system separates:

```text
Retrieval
    ↓
Context
    ↓
Deterministic Decision
    ↓
Validated Action
    ↓
Database Mutation
    ↓
Audit Log
```

This prevents an LLM/retrieval result from directly performing an unsafe state-changing operation.

## Example Scenarios

### Customer-specific cancellation

Northstar has an active agreement allowing cancellation of BOOKED shipments without a fee.

```text
BOOKED
  ↓
Customer agreement verified
  ↓
Confidence >= threshold
  ↓
CANCEL_ORDER
  ↓
CANCELLED
  ↓
Audit log
```

### Picked-up shipment

```text
PICKED_UP
  ↓
Cancellation rejected
  ↓
RETURN_TO_ORIGIN action
```

### Security incident

```text
Possible API key exposure
  ↓
P1 escalation
  ↓
ESCALATE
```

## Tech Stack

- Go
- PostgreSQL
- pgx
- Ollama
- Embeddings
- Docker
- REST API
- Deterministic rule-based decision engine

## Project Status

Core agent evaluation, retrieval, action execution, escalation, audit logging, and API evaluation flow are implemented.

Remaining work focuses on production hardening, broader test coverage, API polish, observability, and deployment readiness.