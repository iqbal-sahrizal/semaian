# Phase 3 Roadmap & Prioritized Backlog

## Completed
- **Phase 3.1.1 / 3.1.1.1**: Core Structured Knowledge (`000003`, `knowledge` domain/pgx, hierarchy cycle & translation triggers) — `PASS`
- **Phase 3.1.2 / 3.1.2.1–3.1.2.3**: Provenance, Licensing & Editorial Foundation (`000004`, `000005`, `FOR SHARE OF c, s`, immutability, concurrency tests) — `PASS`
- **Phase 3.1.3 / 3.1.3.1**: Knowledge API, PostgreSQL FTS (`000006`), bounded rate limiter, OpenAPI 3.1 & `EXPLAIN ANALYZE` — `PASS`

## Phase 3.1.4 Prioritized Backlog (Pending Owner Approval)
| ID | Description | Priority | Complexity | Dependencies |
|---|---|---:|---:|---|
| `P3.1.4-01` | Wire server-side `EditorialRoleResolver` in `main.go` | P0 | M | Phase 2 `identity` |
| `P3.1.4-02` | Expose editorial `sources`, `claims`, `translations`, `supersede` HTTP routes | P0 | M | `P3.1.4-01` |
| `P3.1.4-03` | Expose public `origins/{slug}` & `varieties/{slug}` detail endpoints | P1 | S | Phase 3.1.3 |
| `P3.1.4-04` | Update `openapi.yaml` + route-level RBAC & integration tests | P0 | M | `P3.1.4-01..03` |

## Deferred to Future Phases
- **Phase 4**: `brew` (Brew Studio) & `discovery` (cross-module read projections).
- **Phase 5**: Next.js web & React Native mobile clients.
