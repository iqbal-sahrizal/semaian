# Phase 3.1.4 Implementation Specification: Editorial CRUD, Role Wiring & Public Detail APIs

## 1. Objective & Problem Statement
Phase 3.1.1–3.1.3.1 delivered the Coffee Knowledge database schema, provenance/editorial domain logic, and core public/editorial HTTP endpoints. However, `main.go` still passes `nil` for `EditorialRoleResolver` (defaulting all users to `RoleReader`), and several implemented domain operations (`CreateSource`, `CreateClaim`, `Supersede`, `InsertTranslation`) and public detail-by-slug endpoints are not yet exposed via HTTP/OpenAPI.

## 2. In-Scope Capabilities
1. **Platform Editorial Role Wiring**: Implement a server-side `EditorialRoleResolver` (backed by `identity` / platform role assignment, never trusting client headers/claims or `organization` roles) and wire it into `main.go`.
2. **Editorial HTTP Endpoints**:
   - `POST /api/v1/editorial/knowledge/sources`
   - `POST /api/v1/editorial/knowledge/claims`
   - `POST /api/v1/editorial/knowledge/translations`
   - `POST /api/v1/editorial/knowledge/articles/{articleID}/supersede`
3. **Public Detail-by-Slug Endpoints**:
   - `GET /api/v1/knowledge/origins/{slug}`
   - `GET /api/v1/knowledge/varieties/{slug}`
4. **OpenAPI 3.1 & Tests**: Update `docs/openapi/v1/openapi.yaml` and add unit + PostgreSQL integration + route-level RBAC tests.

## 3. Out-of-Scope
- Frontend / React Native clients, `discovery` module, `brew` module, `catalog` module, external crawlers, or cloud deployment.

## 4. Measurable Acceptance Criteria
- `EditorialRoleResolver` never trusts client-supplied roles and rejects `reader` (`403`) on all editorial write routes.
- `POST .../supersede` transitions `published` -> `superseded`, preserves revision history, and emits `audit.events`.
- Public detail-by-slug endpoints enforce `status = 'published'` translation fallback (`requested -> id -> en -> 404`).
- `gofmt`, `go build ./...`, `go vet ./...`, and `go test -v -race -tags=integration ./...` pass with exit code `0`.
