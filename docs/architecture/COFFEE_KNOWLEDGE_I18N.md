# Coffee Knowledge Internationalization (i18n) & Fallback Rules (Debt Closed)

## 1. Supported Locales
- `id` (Indonesian, primary) and `en` (English, secondary).
- Botanical/scientific names (e.g., *Coffea arabica*) are stored on the canonical entity and **never** translated.
- Invalid `locale` query parameters (anything other than `id` or `en`) return `400 Bad Request` (`application/problem+json`).

## 2. Deterministic Translation Fallback Rules (Public APIs)
Public APIs **only** consider translations with `translations.status = 'published'` AND parent entity `status = 'published'`:
1. **Requested `locale` exists and is `published`**: Return it (`resolved_locale = requested`).
2. **Requested `locale` is missing OR `draft`/`archived`**:
   - Fall back to `id` if `id` translation exists and is `published` (`resolved_locale = "id"`).
   - Otherwise fall back to `en` if `en` translation exists and is `published` (`resolved_locale = "en"`).
3. **Both `id` and `en` are missing or unpublished**: Return `404 Not Found` (`application/problem+json`) — never expose draft/unpublished translations or empty content.
4. **Parent entity is `archived` or `superseded`**: Return `404 Not Found` on public detail/search endpoints (or `410 Gone` if explicitly configured for superseded slugs).
