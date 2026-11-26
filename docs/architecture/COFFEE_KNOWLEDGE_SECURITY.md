# Coffee Knowledge Security Model

- **Public Visibility Isolation**: All public `GET /api/v1/knowledge/...` queries enforce `status = 'published'`. `draft`, `submitted`, `under_review`, and `rejected` records return `404 Not Found` to public/anonymous callers.
- **Editorial Authorization**: Enforced server-side via `knowledge` editorial role checks; `organization` roles (`owner`/`admin`) grant **zero** editorial access.
- **Self-Approval Prevention**: Enforced in domain service (`actorID != authorID`).
- **Stored XSS Prevention**: Editorial content is stored as plain text or sanitized Markdown; raw `<script>` / unsafe HTML is rejected/escaped.
- **Media Boundary**: Images/diagrams reference `media` asset IDs only; uploads are handled exclusively by the `media` module.
