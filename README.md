# ParcelPilot AI

**An agent-driven logistics support and operations system built in Go.**

ParcelPilot AI combines retrieval-augmented context with deterministic business rules, validated state transitions, and explicit human confirmation to safely automate logistics workflows.

> **Core principle:** Use AI/retrieval for context, but use deterministic business rules and validated actions for state changes.

---

## Table of Contents

- [What It Combines](#what-it-combines)
- [Architecture](#architecture)
- [Core Design](#core-design)
- [Current Capabilities](#current-capabilities)
- [Supported Actions](#supported-actions)
- [Explicit Confirmation Flow](#explicit-confirmation-flow)
- [REST API](#rest-api)
- [Example API Flow](#example-api-flow)
- [Chat UI](#chat-ui)
- [Retrieval Pipeline](#retrieval-pipeline)
- [Policy-Aware Reasoning](#policy-aware-reasoning)
- [Safety Model](#safety-model)
- [Order State Validation](#order-state-validation)
- [Audit Logging](#audit-logging)
- [Database](#database)
- [Project Structure](#project-structure)
- [Local Development](#local-development)
- [Document Ingestion](#document-ingestion)
- [Testing](#testing)
- [Example Scenarios](#example-scenarios)
- [Confidence and Safety](#confidence-and-safety)
- [Tech Stack](#tech-stack)
- [Design Principles](#design-principles)
- [Project Status](#project-status)
- [Future Improvements](#future-improvements)
- [Verification](#verification)

---

## What It Combines

| Capability | Description |
| --- | --- |
| Structured operational data | PostgreSQL |
| Document retrieval | Embeddings-based similarity search |
| Policy evaluation | Customer-specific agreements |
| Decision making | Deterministic business rules |
| State changes | Safe, validated actions |
| Mutations | Explicit user confirmation |
| Traceability | Audit logging |
| Workflows | Service credits |
| Interfaces | REST API evaluation flow + browser-based chat UI |

---

## Architecture

```text
                         ┌─────────────────────┐
                         │   Chat UI / Client   │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │     REST API        │
                         │ /evaluate /confirm  │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │    Orchestrator     │
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
                                  ┌────────────────────┼──────────────────┐
                                  ▼                    ▼                  ▼
                              Orders             Service Credits     Escalations
                                  │                    │                  │
                                  └────────────────────┼──────────────────┘
                                                       ▼
                                               ┌──────────────┐
                                               │ Audit Logs   │
                                               └──────────────┘
```

---

## Core Design

ParcelPilot separates **reasoning** from **state mutation**.

```text
User Request
     │
     ▼
Document Retrieval
     │
     ▼
Operational Context
     │
     ▼
Deterministic Decision Engine
     │
     ├──────────────► Answer only
     │
     ├──────────────► Escalate
     │
     ▼
Validated Action
     │
     ▼
Explicit Confirmation
     │
     ▼
Action Executor
     │
     ▼
Database Mutation
     │
     ▼
Audit Log
```

Retrieved documents and AI-generated reasoning **never directly mutate operational state**.

---

## Current Capabilities

### Agent / Decision Engine

The agent evaluates operational requests using:

- Account context
- Order context
- Ticket context
- Retrieved policy documents
- Customer-specific agreements
- Current operational state

The decision engine handles:

- Customer-specific cancellation policies
- Order-state validation
- Return-to-origin workflows
- Service-credit eligibility
- Idempotent actions
- Conflicting policy evidence
- Security escalation
- Confidence-based action safety

---

## Supported Actions

### `CANCEL_ORDER`

Cancels a valid `BOOKED` order.

```text
BOOKED
   │
   ▼
Customer agreement / policy verified
   │
   ▼
High-confidence decision
   │
   ▼
Confirmation required
   │
   ▼
CANCEL_ORDER
   │
   ▼
CANCELLED
```

The action creates an audit record containing:

- Account ID
- Action type
- Target order
- Previous state
- New state
- Reason
- Timestamp

**Example**

Northstar Logistics has an active agreement allowing cancellation of any `BOOKED` shipment without a cancellation fee. The system can produce:

```json
{
  "action": {
    "Type": "CANCEL_ORDER",
    "Target": "ORD-1001",
    "Reason": "active customer agreement overrides the default cancellation fee policy"
  },
  "confidence": 0.95,
  "confirmation_required": true
}
```

---

### `RETURN_TO_ORIGIN`

Picked-up shipments cannot be cancelled. Instead, the system uses the return-to-origin workflow.

```text
PICKED_UP
   │
   ▼
Cancellation rejected
   │
   ▼
RETURN_TO_ORIGIN
   │
   ▼
RETURN_TO_ORIGIN
```

The action is **idempotent** when the order is already in the `RETURN_TO_ORIGIN` state.

Example decision:

```text
picked-up shipments cannot be cancelled; use return-to-origin
```

---

### `SERVICE_CREDIT`

ParcelPilot supports service credits for eligible failed-pickup situations.

The default policy allows a service credit when:

- Pickup is sufficiently delayed
- Carrier is at fault
- Customer is not at fault

The default credit is:

```text
min(INR 500, 10% of shipment fee)
```

Customer agreements can override the default policy. For example, **LumenWorks** has a customer-specific agreement defining:

```text
More than 4 hours past pickup window
+
Carrier at fault
+
Customer not at fault
=
INR 300 service credit
```

Service credits are persisted in PostgreSQL.

---

### `ESCALATE`

The system can create escalation records when a request requires human intervention.

```text
Possible API key exposure
        │
        ▼
       P1
        │
        ▼
    ESCALATE
```

The system avoids taking unsafe state-changing actions when important information is missing or conflicting.

---

## Explicit Confirmation Flow

State-changing actions require explicit confirmation.

```text
POST /evaluate
        │
        ▼
Decision Engine
        │
        ▼
CANCEL_ORDER
        │
        ▼
confirmation_required = true
        │
        ▼
confirmation_id generated
        │
        ▼
User confirms
        │
        ▼
POST /confirm
        │
        ▼
Action Executor
        │
        ▼
Database Mutation
        │
        ▼
Audit Log
```

Example response from `/evaluate`:

```json
{
  "answer": "Northstar Logistics may cancel BOOKED order ORD-1001 before pickup with no cancellation fee under the active customer agreement",
  "confidence": 0.95,
  "action": {
    "Type": "CANCEL_ORDER",
    "Target": "ORD-1001",
    "Reason": "active customer agreement overrides the default cancellation fee policy"
  },
  "escalate": false,
  "confirmation_required": true,
  "confirmation_id": "confirm-1787736987121248804-1"
}
```

The client can then confirm the action:

```http
POST /confirm
Content-Type: application/json

{
  "confirmation_id": "confirm-1787736987121248804-1",
  "confirm": true
}
```

Successful execution returns:

```json
{
  "status": "executed",
  "message": "action confirmed and executed successfully",
  "action": {
    "Type": "CANCEL_ORDER",
    "Target": "ORD-1001",
    "Reason": "active customer agreement overrides the default cancellation fee policy"
  }
}
```

Invalid or expired confirmation IDs are rejected.

---

## REST API

### Health Check

```http
GET /health
```

```json
{
  "status": "ok"
}
```

### Evaluate Request

```http
POST /evaluate
Content-Type: application/json
```

```json
{
  "query": "Northstar wants to cancel this booked shipment",
  "account_id": "ACCT-001",
  "order_id": "ORD-1001"
}
```

The endpoint:

1. Validates the request
2. Retrieves relevant documents
3. Loads account/order context
4. Builds agent context
5. Runs the decision engine
6. Returns the answer and decision
7. Requests confirmation for state-changing actions

### Confirm Action

```http
POST /confirm
Content-Type: application/json
```

```json
{
  "confirmation_id": "confirm-...",
  "confirm": true
}
```

The endpoint executes the pending action **only after explicit confirmation**.

---

## Example API Flow

### Step 1 — Evaluate

```bash
curl -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Northstar wants to cancel this booked shipment",
    "account_id": "ACCT-001",
    "order_id": "ORD-1001"
  }'
```

Response:

```json
{
  "answer": "Northstar Logistics may cancel BOOKED order ORD-1001 before pickup with no cancellation fee under the active customer agreement",
  "confidence": 0.95,
  "action": {
    "Type": "CANCEL_ORDER",
    "Target": "ORD-1001",
    "Reason": "active customer agreement overrides the default cancellation fee policy"
  },
  "confirmation_required": true,
  "confirmation_id": "confirm-..."
}
```

### Step 2 — Confirm

```bash
curl -X POST http://localhost:8080/confirm \
  -H "Content-Type: application/json" \
  -d '{
    "confirmation_id": "confirm-...",
    "confirm": true
  }'
```

### Step 3 — Result

```text
ORD-1001
BOOKED
   │
   ▼
CANCEL_ORDER
   │
   ▼
CANCELLED
```

An audit log is created automatically.

---

## Chat UI

ParcelPilot includes a browser-based chat interface for interacting with the agent.

The interface provides:

- Account ID input
- Order ID input
- Natural-language shipment queries
- Agent responses
- Confidence display
- Action execution status
- Explicit confirmation handling

Example interaction:

```text
User:
Northstar wants to cancel this booked shipment

Agent:
Northstar Logistics may cancel BOOKED order ORD-1001
before pickup with no cancellation fee under the
active customer agreement.

Confidence: 0.95

Action:
CANCEL_ORDER

Confirmation:
Required
```

After confirmation:

```text
Action executed successfully

action confirmed and executed successfully
```

---

## Retrieval Pipeline

Documents are ingested and converted into searchable chunks.

```text
PDF / Document
      │
      ▼
Extraction
      │
      ▼
Chunking
      │
      ▼
Embedding Generation
      │
      ▼
PostgreSQL
      │
      ▼
Similarity Retrieval
      │
      ▼
Agent Context
```

Ollama is currently used for local embeddings.

Default embedding model:

```text
nomic-embed-text
```

---

## Policy-Aware Reasoning

The system supports customer-specific agreements overriding default policies.

**Default SOP**

```text
BOOKED
   │
   ├── within 30 minutes → no fee
   │
   └── after 30 minutes → INR 250 fee
```

**Northstar agreement**

```text
BOOKED
   │
   ▼
No cancellation fee
regardless of booking age
```

The decision engine uses the customer-specific agreement when it has precedence over the default policy. This prevents applying generic policies when a signed customer agreement provides different terms.

---

## Safety Model

ParcelPilot follows a defense-in-depth approach for state-changing operations.

```text
Retrieved Documents
        │
        ▼
Account / Order Context
        │
        ▼
Deterministic Decision
        │
        ▼
Confidence / Safety Validation
        │
        ▼
Explicit User Confirmation
        │
        ▼
Action Executor
        │
        ▼
Database Mutation
        │
        ▼
Audit Log
```

Important principles:

- Retrieval does not directly mutate state.
- Invalid order transitions are rejected.
- High-risk or uncertain requests can be escalated.
- State-changing actions require confirmation.
- Database mutations are performed by deterministic executors.
- Every successful state-changing action is auditable.

---

## Order State Validation

The action executor validates the current database state before performing mutations.

**Cancellation**

```text
BOOKED
   ↓
CANCELLED
```

Allowed when policy and action validation succeed.

```text
PICKED_UP
   ↓
CANCEL_ORDER
```

Rejected. Instead:

```text
PICKED_UP
   ↓
RETURN_TO_ORIGIN
```

**Already Cancelled**

```text
CANCELLED
   ↓
CANCEL_ORDER
```

Detected as an already-completed state and does not blindly perform another mutation.

**Delivered**

```text
DELIVERED
   ↓
CANCEL_ORDER
```

Rejected.

---

## Audit Logging

All state-changing operations are recorded in PostgreSQL.

Audit records contain:

- Account ID
- Action type
- Target ID
- Reason
- Previous state
- New state
- Timestamp

Example:

```text
action_type: CANCEL_ORDER
target_id: ORD-1001

previous_state:
{"status": "BOOKED"}

new_state:
{"status": "CANCELLED"}

reason:
active customer agreement overrides the default cancellation fee policy
```

Audit logs provide an operational trail for every mutation.

---

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
- Service credits

### Database Migrations

Current migrations include:

```text
001_init.sql
002_embedding_dimension.sql
003_audit_logs.sql
004_return_to_origin.sql
005_...
006_service_credits.sql
```

The migration history covers:

- Initial schema
- Embedding support
- Audit logging
- Return-to-origin support
- Service-credit persistence

---

## Project Structure

```text
parcelpilot-ai/
│
├── cmd/
│   ├── embed/
│   │   └── main.go
│   ├── ingest/
│   │   └── main.go
│   ├── inspect/
│   │   └── main.go
│   ├── pdfingest/
│   │   └── main.go
│   ├── retrieve/
│   │   └── main.go
│   └── server/
│       ├── main.go
│       └── main_test.go
│
├── internal/
│   │
│   ├── action/
│   │   ├── executor.go
│   │   └── executor_test.go
│   │
│   ├── agent/
│   │   ├── agent.go
│   │   ├── context.go
│   │   ├── context_builder.go
│   │   ├── decision.go
│   │   ├── decision_test.go
│   │   ├── orchestrator.go
│   │   ├── orchestrator_test.go
│   │   └── retriever.go
│   │
│   ├── db/
│   │   ├── account_context.go
│   │   ├── audit_logs.go
│   │   ├── context_loader.go
│   │   ├── document_chunks.go
│   │   ├── documents.go
│   │   ├── postgres.go
│   │   └── migrations/
│   │
│   ├── document/
│   │   ├── chunker.go
│   │   ├── extractor.go
│   │   ├── metadata.go
│   │   └── service.go
│   │
│   ├── domain/
│   │   └── context.go
│   │
│   ├── embeddings/
│   │   ├── client.go
│   │   ├── indexer.go
│   │   ├── ollama.go
│   │   └── service.go
│   │
│   └── retrieval/
│       ├── postgres.go
│       ├── retriever.go
│       └── service.go
│
├── data/
│   ├── ParcelPilot_Assessment_Data.xlsx
│   └── pdfs/
│
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Local Development

### Requirements

- Go 1.25+
- Docker
- Docker Compose
- PostgreSQL
- Ollama

### Start Infrastructure

```bash
docker compose up -d
```

Check containers:

```bash
docker ps
```

### Configure Environment

Create `.env` with the required database and Ollama configuration.

```env
DATABASE_URL=postgres://parcelpilot:parcelpilot@localhost:5432/parcelpilot

OLLAMA_BASE_URL=http://localhost:11434

OLLAMA_EMBEDDING_MODEL=nomic-embed-text
```

### Run the Server

```bash
go run ./cmd/server
```

The server runs on:

```text
http://localhost:8080
```

Health check:

```bash
curl http://localhost:8080/health
```

Expected:

```json
{
  "status": "ok"
}
```

### Run the Chat UI

Start the server:

```bash
go run ./cmd/server
```

Then open:

```text
http://localhost:8080
```

The browser UI can be used to:

- Enter an account ID
- Enter an order ID
- Ask a natural-language question
- Review the agent's answer
- Review confidence
- Confirm a state-changing action
- See execution status

---

## Document Ingestion

Documents can be processed through the provided ingestion commands.

The repository contains the assessment documents under `data/pdfs/`, including:

```text
01_Support_Policy_v3_CURRENT.pdf
02_Support_Policy_v2_DEPRECATED.pdf
03_Cancellation_and_Service_Credit_SOP_v4.pdf
04_Product_Operations_Guide_and_Known_Issues.pdf
05_Northstar_Logistics_Enterprise_Agreement.pdf
06_LumenWorks_Service_Agreement.pdf
```

These documents provide the policy and customer-agreement knowledge used by the retrieval pipeline.

---

## Testing

The project includes tests covering:

- HTTP health endpoint
- Evaluation endpoint validation
- Decision engine behavior
- Orchestrator behavior
- Order cancellation
- Return-to-origin
- Idempotent return-to-origin
- Invalid state transitions
- Audit log creation
- Service-credit creation
- Escalation creation
- Empty query validation
- Action execution

Run the complete test suite:

```bash
go test ./...
```

Expected result:

```text
ok      github.com/Aditya7880900936/parcelpilot-ai/cmd/server
ok      github.com/Aditya7880900936/parcelpilot-ai/internal/action
ok      github.com/Aditya7880900936/parcelpilot-ai/internal/agent
```

---

## Example Scenarios

### Scenario 1 — Customer-Specific Cancellation

| Field | Value |
| --- | --- |
| Account | `ACCT-001` — Northstar Logistics |
| Order | `ORD-1001` — `BOOKED` |
| Request | Northstar wants to cancel this booked shipment |

Decision:

```text
Northstar Logistics may cancel BOOKED order ORD-1001
before pickup with no cancellation fee under the
active customer agreement.
```

Flow:

```text
BOOKED
   │
   ▼
Northstar agreement verified
   │
   ▼
Confidence = 0.95
   │
   ▼
Confirmation required
   │
   ▼
User confirms
   │
   ▼
CANCEL_ORDER
   │
   ▼
CANCELLED
   │
   ▼
Audit Log
```

### Scenario 2 — Picked-Up Shipment

Order state: `PICKED_UP`

Request:

```text
Northstar wants to cancel this shipment
```

Decision:

```text
picked-up shipments cannot be cancelled;
use return-to-origin
```

Action: `RETURN_TO_ORIGIN`

Result:

```text
PICKED_UP
      │
      ▼
RETURN_TO_ORIGIN
```

### Scenario 3 — Already Cancelled Order

Order: `CANCELLED`

Request:

```text
Cancel this order
```

The decision engine detects the current state and avoids performing an invalid duplicate transition.

### Scenario 4 — Service Credit

A failed pickup is evaluated against:

```text
Pickup delay
+
Carrier fault
+
Customer not at fault
```

The system evaluates:

```text
Default SOP
        │
        ├── Customer agreement?
        │
        ▼
Applicable credit policy
        │
        ▼
SERVICE_CREDIT
        │
        ▼
Persist credit
```

### Scenario 5 — Security Escalation

Request involving possible credential/API key exposure:

```text
Possible API key exposure
        │
        ▼
P1 Security Incident
        │
        ▼
ESCALATE
```

The system does not attempt an unsafe operational mutation.

---

## Confidence and Safety

The system uses confidence as one of the safety signals for action execution.

High-confidence deterministic decisions can produce actions, but state-changing actions additionally require explicit confirmation.

Conceptually:

```text
Decision
   │
   ├── Low confidence
   │       └── Answer / Escalate
   │
   └── High confidence
           │
           ▼
      Valid Action
           │
           ▼
     User Confirmation
           │
           ▼
      Action Executor
```

This provides an additional safety boundary between agent reasoning and database mutation.

---

## Tech Stack

**Backend**

- Go
- `net/http`
- pgx
- PostgreSQL

**AI / Retrieval**

- Ollama
- `nomic-embed-text`
- Vector embeddings
- PostgreSQL similarity retrieval
- Retrieval-augmented context

**Infrastructure**

- Docker
- Docker Compose

**API**

- REST
- JSON

**Architecture**

- Agent orchestration
- Deterministic decision engine
- Action executor
- Explicit confirmation workflow
- Audit logging

---

## Design Principles

**1. Retrieval is not authorization**

Retrieved documents provide context and policy evidence. They do not directly execute database operations.

**2. Customer agreements matter**

Customer-specific agreements can override default policies when applicable.

**3. Operational state is authoritative**

Before executing an action, the executor validates the current database state.

**4. Mutations require confirmation**

State-changing actions require explicit user confirmation.

**5. Actions are deterministic**

The action executor performs known, validated state transitions instead of allowing free-form model output to directly mutate the database.

**6. Mutations are auditable**

Every successful state-changing action records its previous and new state.

---

## Project Status

### Implemented

- [x] PostgreSQL operational data layer
- [x] Account context loading
- [x] Order context loading
- [x] Document ingestion
- [x] Document chunking
- [x] Embedding generation
- [x] PostgreSQL similarity retrieval
- [x] Policy-aware agent context
- [x] Deterministic decision engine
- [x] Customer-specific policy evaluation
- [x] Order cancellation
- [x] Return-to-origin workflow
- [x] Idempotent state handling
- [x] Service-credit workflow
- [x] Escalation workflow
- [x] Audit logging
- [x] REST `/evaluate` endpoint
- [x] REST `/confirm` endpoint
- [x] Explicit confirmation flow
- [x] Browser chat UI
- [x] Action execution safety checks
- [x] Automated tests

---

## Future Improvements

Potential production-hardening work includes:

- Persistent confirmation storage
- Authentication and authorization
- Rate limiting
- Structured observability
- Metrics and tracing
- More extensive integration tests
- Production vector-search optimization
- Deployment configuration
- Distributed confirmation handling
- More operational workflows

---

## Verification

The complete Go test suite can be executed with:

```bash
go test ./...
```

The current implementation has been verified end-to-end for:

```text
Natural Language Request
        ↓
Policy Retrieval
        ↓
Account / Order Context
        ↓
Deterministic Decision
        ↓
Action Generation
        ↓
Explicit Confirmation
        ↓
Action Execution
        ↓
Database State Change
        ↓
Audit Log
```

---

ParcelPilot AI demonstrates how an AI-assisted operational system can combine retrieval and agent reasoning with deterministic business rules and explicit human confirmation to safely automate logistics workflows.