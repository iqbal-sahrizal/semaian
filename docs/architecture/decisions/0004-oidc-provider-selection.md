# ADR 0004: OIDC Provider Selection

- **Status**: Proposed (Requires Owner Approval)
- **Context**: Semaian delegates authentication (password handling, MFA, token issuance) to a standards-compliant OIDC provider (`SECURITY_BASELINE.md`). We must not silently choose a paid provider or require paid accounts for local development.

## Options Considered
1. **Self-hosted / Local OIDC (e.g., Zitadel, Keycloak, or Dex in Docker)**
   - *Pros*: $0 cost, works offline in Docker Compose, full data ownership, no vendor lock-in.
   - *Cons*: Operational overhead if self-hosted in cloud later.
2. **Managed Free-Tier OIDC (e.g., Auth0, Clerk, Supabase Auth, Ory Network)**
   - *Pros*: Zero auth-server maintenance, built-in MFA/social login.
   - *Cons*: Free-tier MAU/feature limits, potential commercial-use restrictions, credit card or vendor lock-in risks.
3. **Hybrid (Standard OIDC Interface)**
   - *Pros*: Go API verifies standard OIDC JWKS + `iss`/`aud` claims. Local dev uses a lightweight OIDC emulator; production/preview can bind to any compliant OIDC provider approved by the owner.
   - *Cons*: Requires keeping local emulator claims aligned with production provider claims.

## Decision
Adopt **Option 3 (Standard OIDC Interface)** for Phase 1 architecture. Defer specific cloud OIDC vendor procurement until Phase 1/2 deployment approval.
