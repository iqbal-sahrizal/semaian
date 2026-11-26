# Phase 3 Gap Analysis & Domain Ownership

## 1. Domain Ownership Boundaries (No Catch-All `knowledge` Domain)
- **`knowledge`**: Canonical geography, taxonomy, reference brewing concepts, educational articles, provenance (`sources`, `claims`), and `id`/`en` translations.
- **`catalog` (Future)**: Commercial SKUs, roasteries' products, pricing, and green-coffee lots/batches.
- **`brew` (Future)**: User/org personal recipes, brew logs, timers, and equipment inventories.
- **`discovery` (Future)**: Cross-module search/read projections.
- **`community` / `b2b` (Future)**: Social posts, reviews, RFQs, and commerce.

## 2. Key Technical & Product Gaps Identified
1. **Editorial Role Resolution (`main.go`)**: `knowledge.NewHandler` currently receives `nil` for `EditorialRoleResolver`, which defaults all authenticated callers to `RoleReader`. Phase 3.1.4 must wire a concrete `EditorialRoleResolver` (e.g., from `identity` / platform role table or verified OIDC claims).
2. **Editorial HTTP Endpoints for Sources, Claims, Translations & Supersede**: `EditorialService` already implements `CreateSource`, `CreateClaim`, and `Supersede`, and `PGXRepository` implements `InsertTranslation`, but they are not yet exposed via `http.go` + `openapi.yaml`.
3. **Public Origin/Variety Detail-by-Slug**: List endpoints exist (`/api/v1/knowledge/origins`, `varieties`, etc.), but detail-by-slug endpoints (`/api/v1/knowledge/origins/{slug}`) are not yet wired.
