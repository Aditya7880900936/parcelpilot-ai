# ParcelPilot AI — Architecture Note

## Agent Design

ParcelPilot AI uses an orchestrator-based agent architecture.

An incoming request is processed through:

1. Document retrieval
2. Structured account/order/ticket context loading
3. Context construction
4. Deterministic decision evaluation
5. Action validation
6. Optional state-changing execution
7. Audit logging

The system intentionally separates contextual reasoning from operational state mutation.

Retrieved documents provide policy context, but they cannot directly modify database state.

## Tool Design

The main operational capabilities are represented as validated actions:

- CANCEL_ORDER
- RETURN_TO_ORIGIN
- ESCALATE

Actions are executed only after the decision engine validates that the requested transition is allowed for the current operational state.

State-changing actions require explicit confirmation through the confirmation flow.

## Document and Structured Data Handling

Policy and agreement PDFs are extracted, chunked and embedded for similarity retrieval.

Structured operational information such as:

- Accounts
- Orders
- Tickets
- Audit logs
- Escalations

is stored in PostgreSQL.

The agent combines retrieved document context with structured operational state before making a decision.

## Source Reliability and Conflict Handling

Source precedence is important because customer agreements may override default policies.

The system prioritizes:

1. Signed customer agreement
2. Current support/policy documentation
3. Current product documentation

Deprecated or historical documents are not treated as authoritative current policy.

When policy evidence conflicts or required operational information is uncertain, the system avoids unsafe state-changing actions and can escalate/request verification.

## Safety Model

The system follows:

Retrieval
→ Context
→ Deterministic Decision
→ Validated Action
→ Database Mutation
→ Audit Log

This prevents retrieved text from directly performing operational mutations.

## Technical Trade-offs

PostgreSQL was selected for both operational data and document retrieval to keep the architecture simple and transactional.

Ollama provides local embeddings, avoiding dependency on an external embedding API during development.

A deterministic decision engine was preferred over allowing an LLM to directly choose database mutations. This improves predictability, testability and safety at the cost of requiring explicit business rules for supported workflows.