# API Versioning & Event Standards (Debt Closed)

## 1. API Versioning & OpenAPI Standards
- **URL Path Versioning**: All public HTTP endpoints use `/api/v1/...`. Health endpoints (`/health/live`, `/health/ready`) remain unversioned.
- **Canonical OpenAPI Path**: All OpenAPI 3.1 specifications live in **`docs/openapi/v1/`** (e.g., `docs/openapi/v1/openapi.yaml`).
- **Automated Contract Validation**:
  - CI lints `docs/openapi/v1/openapi.yaml` (e.g., Spectral or `kin-openapi` validator).
  - HTTP handler tests validate request/response payloads and RFC 9457 (`application/problem+json`) error envelopes against the OpenAPI 3.1 schema at runtime.
- **Breaking Changes**: Removing fields, changing types, or altering auth requirements requires `/api/v2` or an explicit deprecation window.

## 2. Internal Event Standards (In-Process Bus, No Broker in MVP)
- **In-Process Event Bus**: Cross-domain side effects use a lightweight in-process Go event dispatcher. No Kafka, Redpanda, RabbitMQ, or NATS in Phase 0–1.
- **Event Envelope Contract**:
  - `event_id` (UUIDv7, deterministic clock/generator injectable for tests)
  - `event_type` (versioned, e.g., `brew.recipe.published.v1`)
  - `occurred_at` (RFC 3339 UTC)
  - `actor_id` (UUID, nullable for system events)
  - `correlation_id` (UUID, propagated from `context.Context`)
  - `payload` (immutable, versioned struct)
- **Delivery Limitations & Error Handling**:
  - **At-most-once / best-effort in-process**: Events are dispatched in-memory; if the process crashes mid-dispatch, uncompleted handlers are lost. Critical state transitions must not rely solely on in-process events without a fallback.
  - **Handler Isolation**: Subscriber panics/errors must be recovered, logged with `correlation_id`, and never crash the caller's HTTP request.
- **Idempotency & Versioning**:
  - Subscribers must be **idempotent** (keyed by `event_id` or natural idempotency key) so replays are safe.
  - Breaking payload changes require a new `event_type` version (e.g., `.v2`); old subscribers continue handling `.v1` during transition.
- **Future Outbox Migration**:
  - When a module is extracted or requires guaranteed delivery, replace the in-process publisher with a **transactional outbox** table in the module's schema + background relay/broker, keeping domain event structs unchanged.
