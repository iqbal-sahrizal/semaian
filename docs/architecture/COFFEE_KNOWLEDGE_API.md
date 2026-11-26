# Coffee Knowledge API, Search Protection & Discovery Boundary (Debt Closed)

## 1. Public Read APIs (`/api/v1/knowledge/...`, Anonymous Allowed)
- `GET /api/v1/knowledge/origins` & `GET /api/v1/knowledge/origins/{slug}`
- `GET /api/v1/knowledge/varieties`, `/processes`, `/roast-levels`, `/flavors`
- `GET /api/v1/knowledge/articles` & `GET /api/v1/knowledge/articles/{slug}`
- `GET /api/v1/knowledge/search?q=...&locale=id` (PostgreSQL FTS with `GIN` index; no Elasticsearch/OpenSearch).

## 2. Search Protection & Limits
- **Query Length**: Minimum `2` characters, maximum `100` characters. Empty `q` returns `400 Bad Request` (`application/problem+json`).
- **Pagination**: `limit` default `20`, max `50`; `offset` >= `0`.
- **Rate Limiting**: Per-IP rate limit on `/api/v1/knowledge/search` (e.g., 60 req/min burst 15) at the API/edge layer.
- **Query Timeout**: PostgreSQL statement/context timeout of `<= 2s` for search queries.
- **Public Visibility Filter**: Every search query filters `status = 'published'` and `translations.status = 'published'` before returning results.
- **Safe Sorting/Filtering**: Allowlisted sort fields (`relevance`, `title`, `updated_at`) and allowlisted `locale` (`id`, `en`).

## 3. Editorial Write APIs (`/api/v1/editorial/knowledge/...`, Authenticated + Editorial Role)
- `POST/PATCH` for structured entities, translations, sources, claims, and articles.
- Workflow transitions: `POST .../submit`, `POST .../approve`, `POST .../reject`, `POST .../publish`, `POST .../supersede`, `POST .../archive`.
- All errors use RFC 9457 `application/problem+json`.

## 4. Search & Discovery Boundary
- `knowledge` is the source of truth; `discovery` never queries `knowledge.*` tables directly.
- On `publish` / `supersede` / `archive`, `knowledge` emits an in-process domain event (`knowledge.entity.published.v1`, etc.) with `visibility: "public"`.
