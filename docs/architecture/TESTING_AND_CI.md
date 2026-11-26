# Testing Strategy, CI Checks & Quality Gates (Debt Closed)

## 1. Test Pyramid & Execution Requirements
- **Unit Tests (`go test -race ./...`)**: Fast, deterministic tests for domain rules and application use cases (no DB or network).
- **Integration Tests (`go test -race -tags=integration ./...`)**:
  - Executed against a dedicated PostgreSQL 17 instance (`semaian_test`, provisioned via Docker Compose locally or GitHub Actions `services:` in CI).
  - Covers SQL queries, migrations, `discovery` ACL filtering, and **cross-organization isolation**.
- **Runtime Contract Tests**: Validate HTTP handlers and RFC 9457 `application/problem+json` error envelopes against `docs/openapi/v1/openapi.yaml`.
- **OIDC & Health Check Testability**:
  - Offline CI/integration tests use an **in-process mock JWKS server + test-token signer** (never enabled in non-test builds).
  - `/health/live` checks process liveness only; `/health/ready` verifies PostgreSQL connectivity (`pgx` ping with <=2s timeout).

## 2. Migration Safety Gates
Every PR touching `migrations/` must pass:
1. **Up + Down + Re-Up**: Apply all migrations, roll back the latest migration, and re-apply cleanly on `semaian_test`.
2. **Idempotency / Drift Check**: Verify schema matches expected state with zero uncommitted drift.

## 3. CI Checks & Quality Gates (Before Merge to `main`)
1. `gofmt -l services/api` (must be empty)
2. `go vet ./...`
3. `go test -race ./...` (unit + contract tests)
4. `go test -race -tags=integration ./...` (with `semaian_test` + migration safety gate)
5. Architecture boundary check (no cross-domain `internal/modules/<other>` SQL or entity imports)
6. OpenAPI lint/validation on `docs/openapi/v1/openapi.yaml`
7. Security scans: `govulncheck ./...` and secret scanning (`gitleaks`)
8. **AuthZ Negative-Path Gate**: Protected endpoints must have automated `401`/`403`/`404` and cross-org isolation tests.

## 4. Verification Discipline
- Never claim tests passed unless executed after the final edit.
- Distinguish clearly between `passed`, `failed`, `blocked`, and `not-run`.
