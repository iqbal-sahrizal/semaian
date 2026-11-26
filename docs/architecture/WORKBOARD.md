# Semaian Architecture & Implementation Workboard (Debt Closed)

> **Status**: Phase 0 (Architecture Governance & Debt Closure) complete, awaiting owner approval before Phase 1.

## Phase 0 — Architecture Governance (Complete)
- [x] `P0-01` Repository inspection & baseline verification (`architect`)
- [x] `P0-02` MVP scope validation & unanswered product questions (`architect`)
- [x] `P0-03` Bounded contexts, MVP modules & dependency rules (`architect`)
- [x] `P0-04` PostgreSQL schema ownership, runtime/migration roles & audit append-only strategy (`architect`)
- [x] `P0-05` Security baseline, OIDC validation, threat model, `discovery` ACL & rate limiting (`architect`, `security-auditor`)
- [x] `P0-06` API versioning, canonical OpenAPI path (`docs/openapi/v1/`), & in-process event bus standards (`architect`)
- [x] `P0-07` Free-tier strategy, migration safety gates, & CI quality gates (`architect`, `qa-engineer`)
- [ ] `P0-08` Owner sign-off on Phase 0, ADRs 0004–0005, & unanswered product questions (**BLOCKING PHASE 1**)

## Phase 1 — Core Foundations (Pending Owner Approval)
- [ ] `P1-01` **Go Modular Monolith Skeleton** (`backend-go`, `qa-engineer`)
  - *Acceptance Criteria*: `internal/modules/` layout created; `pgx` pool wired; `/health/live` returns `200`; `/health/ready` pings PostgreSQL with <=2s timeout and returns `503` when DB is down; `slog` JSON logging with PII redaction; `go vet` & `go test -race ./...` pass.
- [ ] `P1-02` **OIDC JWT Verification & Local Dev Auth** (`backend-go`, `security-auditor`, `qa-engineer`)
  - *Acceptance Criteria*: Middleware verifies JWKS, `iss`, `aud`, `exp`, `nbf`, and pins asymmetric `alg` (rejects `alg: none`); fails closed in non-local builds if unconfigured; offline tests use mock JWKS server; negative-path tests verify `401` on missing/invalid/expired tokens.
- [ ] `P1-03` **`identity`, `organization` & `audit` Schemas + Isolation** (`backend-go`, `qa-engineer`, `security-auditor`)
  - *Acceptance Criteria*: Dedicated schemas (`identity`, `organization`, `audit`); migration up/down/re-up passes on `semaian_test`; `audit` runtime role has `INSERT`/`SELECT` only; integration tests verify `403`/`404` when Org 1 user accesses Org 2 resources.
- [ ] `P1-04` **OpenAPI v1 Scaffold & CI Pipeline** (`backend-go`, `qa-engineer`)
  - *Acceptance Criteria*: `docs/openapi/v1/openapi.yaml` lints cleanly; `.github/workflows/ci.yml` runs `gofmt`, `go vet`, `go test -race ./...`, `go test -race -tags=integration ./...` (with `semaian_test`), boundary check, `govulncheck`, and secret scan.
- [ ] `P1-05` **`knowledge`, `brew` & `discovery` MVP Modules + Event Bus** (`backend-go`, `qa-engineer`, `security-auditor`)
  - *Acceptance Criteria*: In-process event bus dispatches versioned events with `event_id` (UUIDv7) and `correlation_id`, recovers subscriber panics, and enforces idempotency; `discovery` projections store `visibility`/`owner_id`/`org_id` and pass cross-user/cross-org isolation tests; unit + integration + OpenAPI contract tests pass.

## Phase 2 — Web & Mobile Clients (Pending Phase 1)
- [ ] `P2-01` Next.js web app scaffold consuming `docs/openapi/v1/openapi.yaml` (`frontend`)
- [ ] `P2-02` React Native mobile app scaffold consuming `docs/openapi/v1/openapi.yaml` (`frontend`)
- [ ] `P2-03` End-to-end & security verification (`qa-engineer`, `security-auditor`)
