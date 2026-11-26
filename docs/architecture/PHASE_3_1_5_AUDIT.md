# Phase 3.1.5 Audit: Coffee Knowledge Foundation

## 1. Verified Baseline
- **Migrations**: `000001`–`000006` (`identity`, `organization`, `audit`, `knowledge` schemas, roles, triggers, FTS `GIN` index).
- **Go Implementation**: `services/api/cmd/api/main.go`, `services/api/internal/modules/knowledge/` (`domain.go`, `pgx_repo.go`, `editorial.go`, `editorial_pgx.go`, `query_pgx.go`, `http.go`), `services/api/internal/platform/auth/jwks.go`.
- **OpenAPI**: `docs/openapi/v1/openapi.yaml` (OpenAPI 3.1.0).
- **Test Evidence**: `go test -v -race -tags=integration ./...` passed against `semaian_test` (`exit 0`).

## 2. Current Capabilities vs. Operational Gaps
1. **Verified by Tests**: Structured entity persistence, hierarchy cycle prevention, polymorphic translation integrity, provenance/license gate (`FOR SHARE OF c, s`), author-only submission, self-approval prevention, revision immutability, concurrent publish locking, public FTS search, rate limiting, and fail-closed `EditorialRoleResolver`.
2. **Present in Repo, Not Exposed via HTTP**: `PGXRepository.CreateCountry/Region/Producer/Species/Variety/ProcessingMethod/RoastLevel/FlavorDescriptor` and `LinkRegionVariety/VarietyFlavor/ProcessFlavor` have no HTTP endpoints yet.
3. **Missing Capabilities**:
   - Editorial `POST /api/v1/editorial/knowledge/translations` does not emit an `audit.events` record (`http.go:241-282` calls `h.repo.InsertTranslation` directly).
   - No safe **update** operation for existing structured entity translations (`ON CONFLICT DO UPDATE` or `PATCH`) or **archive** operation for deprecated sources/entities.
   - No **new revision** creation endpoint (`CreateRevision`) for correcting an already-published article before `Supersede`.
