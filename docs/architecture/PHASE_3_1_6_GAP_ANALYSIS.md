# Phase 3.1.6 Gap Analysis

| Finding ID | Priority | Description | Evidence | Impact | Recommended Resolution |
|---|---|---|---|---|---|
| `GAP-316-01` | `P1` | `EditorialService` and `PGXRepository` audit writes use `pool.Exec` outside the entity's DB transaction (`_ = s.audit.Record(...)`) | `editorial.go` | If `audit.Record` fails, entity mutation still commits | Support transactional `audit.Recorder` (`TxRecorder`) in Phase 3.1.7 |
| `GAP-316-02` | `P1` | Live GitHub Actions CI workflow has not been executed remotely yet | `.github/workflows/ci.yml` | Spectral OpenAPI lint & `gitleaks` not yet run in CI | Push to GitHub and verify `ci.yml` passes |
| `GAP-316-03` | `P2` | Public `GET /api/v1/knowledge/origins/{slug}` & `varieties/{slug}` return core entity fields without linked relationship lists (e.g., `region_varieties`, `variety_flavors`) | `query_pgx.go:129-165` | Consumer UI needs extra calls or future endpoint expansion | Add bounded related-entity expansion in Phase 4 / Consumer API |
| `GAP-316-04` | `P3` | No seed/import CLI for initial Indonesian/English coffee knowledge dataset | `services/api/cmd/` | Empty DB on fresh local/staging setup | Add idempotent, provenance-checked seed command in Phase 3.1.7 |
