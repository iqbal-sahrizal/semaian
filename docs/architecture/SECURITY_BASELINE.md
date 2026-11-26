# Security Baseline — Phase 0 (Debt Closed)

## 1. Authentication Provider Requirements (OIDC)
- **Delegated Authentication**: Use a mature, standards-compliant OIDC provider. Do **not** implement password hashing, token issuance, or MFA from scratch (see **ADR 0004**).
- **JWT Validation (Server-Side)**:
  - Verify signature via JWKS, `iss`, `aud`, `exp`, and `nbf`.
  - **Algorithm Pinning**: Explicitly allowlist asymmetric algorithms (e.g., `RS256`, `ES256`, `EdDSA`); reject `alg: none` and symmetric/asymmetric confusion.
- **Client Flow & Token Storage**:
  - Require **Authorization Code Flow + PKCE**, `state`, and `nonce` for Next.js and React Native clients.
  - Web: prefer `HttpOnly`, `Secure`, `SameSite=Lax` (or `Strict`) cookies over `localStorage` for session/refresh tokens.
- **Membership & Revocation Freshness**:
  - Keep access tokens short-lived (e.g., 5–15 minutes).
  - Verify active organization membership and account status server-side on privileged/org-scoped requests rather than trusting stale JWT claims alone.
- **Local Dev Guardrail**:
  - Local OIDC emulator/mock-JWKS is permitted **only** when `APP_ENV=local` (or test builds). Production/staging binaries must fail closed if OIDC issuer/JWKS is unconfigured.

## 2. Authorization, Cross-Org Isolation & Discovery Filtering
- **Server-Side Enforcement**: Never trust client-supplied `user_id`, `org_id`, or `role` headers/claims without server-side verification.
- **Resource Ownership**:
  - Personal resources: `resource.owner_id == actor.user_id`.
  - Organization resources: active membership in `organization` with required role (`owner`, `admin`, `member`).
- **Discovery Projection Authorization & Privacy Filtering**:
  - All `discovery` read models/search indices must carry visibility/ACL attributes (`visibility`: `public` | `org` | `private`, `owner_id`, `org_id`).
  - Every `discovery` query must enforce visibility filtering at query time; private brew logs and org-restricted records must never leak to unauthorized actors.
- **Mandatory Isolation Tests**: Every org-scoped and `discovery` endpoint must have automated integration tests verifying `403 Forbidden` or `404 Not Found` on cross-org/cross-user access.

## 3. Threat Model & Privacy / Data Minimization
| Threat | Mitigation |
|---|---|
| **BOLA / IDOR** | Enforce `owner_id` / `org_id` checks in domain use cases; test cross-tenant denial. |
| **SQL Injection** | Parameterized SQL only (`pgx` / `sqlc`); zero string concatenation. |
| **Sensitive Log Exposure** | Structured `slog` with explicit field allowlists; redact `Authorization`, cookies, tokens, emails, and PII. |
| **Rate Limiting & Abuse** | Enforce per-IP, per-user, and per-org rate limits on auth, upload, and search/discovery endpoints. |
| **Media XSS & EXIF Leaks** | Private-by-default object storage; validate magic bytes and max size; disallow active content (`image/svg+xml`, `text/html`); strip EXIF/GPS metadata; serve with `Content-Disposition: attachment` or isolated origin via short-lived signed URLs. |
| **Transport & Headers** | Enforce TLS, HSTS, `X-Content-Type-Options: nosniff`, explicit CORS allowlist (no `*` with credentials), and request body size/timeout limits. |
| **Supply Chain & Secrets** | Never commit secrets; inject via runtime env/secret manager; run `govulncheck`, `npm audit`, and secret scanning (`gitleaks`) in CI. |
| **Data Minimization** | Store only OIDC `sub`, display name, and minimal profile fields; defer PII until required by an approved feature. |
