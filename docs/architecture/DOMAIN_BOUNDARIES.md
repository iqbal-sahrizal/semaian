# Domain Boundaries — Phase 0 Architecture (Debt Closed)

## Bounded Context Review & Overlap Resolution
1. **Identity vs. Organization**: `Identity` owns user profiles and OIDC subject mappings; `Organization` owns orgs, branches, memberships, and org roles. `Organization` references `user_id` via application interface, not FK joins.
2. **Coffee Knowledge vs. Catalog vs. Brew Studio**: `Coffee Knowledge` owns canonical taxonomy and guides; `Catalog` (deferred) owns commercial SKUs/pricing; `Brew Studio` owns personal/public recipes and brew logs.
3. **Discovery (Read Projections & Privacy Filtering)**:
   - `Discovery` owns read projections and search indices only—never source-of-truth state.
   - Every projection **must** store `visibility` (`public` | `org` | `private`), `owner_id`, and `org_id` (nullable) and enforce authorization filtering at query time.
4. **Review vs. Community vs. Moderation**: Deferred to post-MVP phases.

## MVP Domain Modules (Phase 1–2)
Active modules in `services/api`: `identity`, `organization`, `knowledge`, `brew`, `discovery`, `media`, `audit`.

## Dependency Rules (Enforced in Go & CI)
1. **Inward Dependency Rule**: `transport (HTTP/Chi)` -> `application` -> `domain`. Domain packages must have zero imports from `net/http`, `pgx`, or external SDKs.
2. **No Cross-Domain Table Access**: Module A never queries Module B's PostgreSQL schema.
3. **Explicit Cross-Domain Contracts**: Use exported Go application interfaces or in-process domain events.
4. **No Shared Business Entities**: Only technical primitives (`logger`, `clock`, `httpx`) may be shared.
