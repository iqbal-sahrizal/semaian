# Phase 3.1.5 Gap Analysis

| Finding ID | Description | Evidence | Impact | Severity | Blocks 3.1.5? | Recommended Resolution |
|---|---|---|---|---|---|---|
| `GAP-315-01` | `insertTranslation` handler does not emit `audit.events` | `knowledge/http.go:241-282` | Translation writes lack audit trail | Medium | No (fix in 3.1.5) | Route through `EditorialService.InsertTranslation` with `audit.Recorder` |
| `GAP-315-02` | No HTTP endpoints to create structured entities (`countries`, `regions`, `varieties`, `processes`, `flavors`) | `knowledge/pgx_repo.go` vs `http.go` | Editors cannot seed/manage taxonomy via API | High | Yes | Expose editorial creation endpoints for structured entities |
| `GAP-315-03` | No endpoint to add a new revision to an existing article | `knowledge/editorial_pgx.go` (`CreateArticleWithRevision` only creates rev 1) | Corrections require creating a brand-new article | Medium | No | Add `CreateRevision` (`revision_number + 1`) in `EditorialService` + HTTP |
| `GAP-315-04` | No source/entity archival endpoint | `000004` (`verification_status` has `archived`) | Deprecated sources cannot be archived via API | Low | No | Add `ArchiveSource` endpoint |
