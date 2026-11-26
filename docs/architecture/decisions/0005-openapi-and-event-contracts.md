# ADR 0005: OpenAPI Tooling & In-Process Event Bus (Updated)

- **Status**: Proposed (Requires Owner Approval)
- **Context**: Semaian requires versioned OpenAPI contracts (`/api/v1/...`) and decoupled cross-domain communication without deploying a message broker in MVP.

## Decisions
1. **Canonical OpenAPI Path**: `docs/openapi/v1/openapi.yaml` (OpenAPI 3.1, contract-first).
2. **Contract Validation**: CI lints `docs/openapi/v1/openapi.yaml` and runs runtime handler-vs-OpenAPI contract tests (including RFC 9457 `application/problem+json` error envelopes).
3. **In-Process Event Bus**:
   - Use an in-process Go event dispatcher for MVP (no Kafka/Redpanda/NATS).
   - Documented as best-effort/at-most-once; subscribers must be idempotent (`event_id`) and panic-isolated.
   - Breaking event changes use versioned `event_type` suffixes (`.v1`, `.v2`).
   - Future extraction migrates to a transactional outbox table per module schema without changing domain event structs.
