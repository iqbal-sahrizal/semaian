# Phase 3.1.5 Implementation Plan (Refined)

## Work Packages (Ordered by Dependency & Risk)
1. **`P3.1.5-01` — Audited `EditorialService.InsertTranslation`**:
   - *Files*: `knowledge/editorial.go`, `knowledge/http.go`, `knowledge/http_test.go`
   - *Acceptance*: `POST /api/v1/editorial/knowledge/translations` routes through `EditorialService`, emits `audit.events` with `entity_type`/`locale` on `201`, and emits `0` audit events on `400`/`403`/`409`.
2. **`P3.1.5-02` — Structured Entity Editorial Endpoints (all 8 entities)**:
   - *Files*: `knowledge/http.go`, `docs/openapi/v1/openapi.yaml`, `knowledge/http_test.go`
   - *Acceptance*: `POST` endpoints for `countries`, `regions`, `producers`, `species`, `varieties`, `processing-methods`, `roast-levels`, `flavors` enforce `401`/`403`, return `201`/`400`/`409`, and emit `audit.events`.
3. **`P3.1.5-03` — `CreateRevision` for Article Corrections**:
   - *Files*: `knowledge/editorial.go`, `knowledge/editorial_pgx.go`, `knowledge/http.go`, `docs/openapi/v1/openapi.yaml`, `knowledge/editorial_pgx_integration_test.go`
   - *Acceptance*: `FOR UPDATE` on `knowledge.articles` serializes concurrent `CreateRevision` calls; `published_revision_id` and prior revisions remain untouched; `audit.events` recorded.
4. **`P3.1.5-04` — OpenAPI & Full Regression Verification**:
   - *Checks*: `gofmt`, `go build ./...`, `go vet ./...`, `go test -v -race -tags=integration ./...` on `semaian_test`.
