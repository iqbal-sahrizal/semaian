# Phase 3 Implementation Audit & Capability Matrix

**Baseline**: Migrations `000001`–`000006`, Go `knowledge` module (`domain.go`, `pgx_repo.go`, `editorial.go`, `editorial_pgx.go`, `query_pgx.go`, `http.go`), and `docs/openapi/v1/openapi.yaml`.

## 1. Structured Coffee Knowledge
| Capability | Status | Evidence | Gaps / Next Action |
|---|---|---|---|
| Countries, Regions, Producers | `IMPLEMENTED_AND_VERIFIED` | `000003`, `pgx_repo.go`, `TestKnowledgePGXIntegration` | None for core persistence |
| Species, Varieties, Processes, Roasts, Flavors | `IMPLEMENTED_AND_VERIFIED` | `000003`, `pgx_repo.go`, `TestKnowledgePGXIntegration` | None for core persistence |
| Relationship tables (`region_varieties`, `variety_flavors`, `process_flavors`) | `IMPLEMENTED_AND_VERIFIED` | `000003`, `pgx_repo.go` | None |
| Polymorphic `translations` + cascade triggers | `IMPLEMENTED_AND_VERIFIED` | `000003`, `InsertTranslation`, `TestKnowledgePGXIntegration` | None |
| Public taxonomy list endpoints | `IMPLEMENTED_AND_VERIFIED` | `http.go`, `query_pgx.go`, `openapi.yaml` | Detail-by-slug endpoints for origins/varieties not yet wired |

## 2. Provenance, Licensing & Editorial Workflow
| Capability | Status | Evidence | Gaps / Next Action |
|---|---|---|---|
| Sources & Claims persistence | `IMPLEMENTED_AND_VERIFIED` | `000004`, `editorial_pgx.go` | HTTP endpoints for managing sources/claims not yet wired |
| License compatibility & attribution gate | `IMPLEMENTED_AND_VERIFIED` | `ValidateSourceForPublish`, `PublishArticle` (`FOR SHARE OF c, s`) | None |
| Editorial lifecycle (`draft`→`published`→`superseded`/`archived`) | `IMPLEMENTED_AND_VERIFIED` | `000004`, `000005`, `editorial_pgx.go` | `Supersede` HTTP route not yet wired in `http.go` |
| Revision immutability (`protect_revision_history`) | `IMPLEMENTED_AND_VERIFIED` | `000005`, `TestProvenanceAndEditorialPGXIntegration` | None |
| `EditorialRoleResolver` wiring in `main.go` | `IMPLEMENTED_WITH_GAPS` | `main.go` passes `nil` to `knowledge.NewHandler` | Defaults to `RoleReader`; needs OIDC/DB role resolver in Phase 3.1.4 |

## 3. Public API, Search & Operational Readiness
| Capability | Status | Evidence | Gaps / Next Action |
|---|---|---|---|
| Public articles list/detail & search (`GIN` FTS) | `IMPLEMENTED_AND_VERIFIED` | `000006`, `query_pgx.go`, `http.go`, `EXPLAIN ANALYZE` | None |
| Per-IP rate limiter (`60 req/min`, bounded map) | `IMPLEMENTED_AND_VERIFIED` | `http.go`, `http_test.go` | None |
| OpenAPI 3.1 contract | `IMPLEMENTED_AND_VERIFIED` | `docs/openapi/v1/openapi.yaml` | Add new Phase 3.1.4 endpoints when implemented |
