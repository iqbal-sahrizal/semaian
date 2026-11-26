# Phase 3.1.6 Product Journey Readiness Matrix

| # | User Journey | Status | Evidence |
|---|---|---|---|
| 1 | Beginner discovers a coffee variety & characteristics | `READY` | `GET /api/v1/knowledge/varieties`, `varieties/{slug}`, `id`/`en` fallback |
| 2 | User explores origin, region & producer | `READY` | `GET /api/v1/knowledge/origins`, `origins/{slug}`, `knowledge.producers` |
| 3 | User learns differences between processing methods | `READY` | `GET /api/v1/knowledge/processes` + `process_flavors` |
| 4 | User explores flavor descriptors & hierarchy | `READY` | `GET /api/v1/knowledge/flavors` + `prevent_hierarchy_cycle` |
| 5 | User reads educational articles with reliable sources | `READY` | `GET /api/v1/knowledge/articles`, `articles/{slug}`, provenance gate |
| 6 | User switches between Indonesian and English | `READY` | `locale=id|en`, deterministic `requested -> id -> en -> 404` |
| 7 | Editor creates & submits new knowledge | `READY` | `POST /api/v1/editorial/knowledge/...` + `submit` |
| 8 | Reviewer verifies sources & approves content | `READY` | `POST .../review` (`reviewer_id != author_id`) + `publish` |
| 9 | Published article receives new revision without corrupting published version | `READY` | `POST .../revisions` (`CreateRevision` `FOR UPDATE`) |
| 10 | Consumer searches for relevant coffee knowledge | `READY` | `GET /api/v1/knowledge/search?q=...` (`GIN` FTS + rate limit) |
