# Coffee Knowledge Testing Strategy (Phase 3.1+)

- **Unit Tests**: Editorial state machine transitions, self-approval rejection, license compatibility gate (`restricted` blocked from publish), i18n fallback resolution, and XSS/input sanitization.
- **Integration Tests (`-tags=integration` on `semaian_test`)**: Migration `000003` up/down/re-up, `translations` uniqueness & FTS queries, `audit.events` emission on publish/archive.
- **Security & AuthZ Negative Tests**: Anonymous/reader `401`/`403` on editorial endpoints, `404` on `draft` items via public endpoints, org-admin rejected from editorial endpoints.
- **OpenAPI Contract Tests**: Validate `/api/v1/knowledge/...` and RFC 9457 error responses against `docs/openapi/v1/openapi.yaml`.
