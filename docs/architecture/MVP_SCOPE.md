# MVP Scope — Phase 0 Validation

## Product Positioning
Semaian is a coffee ecosystem platform serving both **consumers** (coffee beginners, home brewers, enthusiasts) and **industry participants** (baristas, roasteries, coffee shops, farmers, suppliers). Consumer education and brewing guidance are first-class capabilities; B2B discovery and commerce are progressive capabilities.

## Validated MVP Scope (Phase 1–2)
1. **Consumer Core (First-Class)**
   - **Coffee Knowledge**: origins, varieties, processing methods, roast levels, flavor taxonomy, and structured educational content.
   - **Brew Studio**: brew guidance (ratio, grind size, water temperature, equipment), personal brew recipes, and brew logs.
   - **Discovery**: search and filtering across coffee knowledge and (later) public recipes/listings.
2. **Foundational Platform**
   - **Identity & Organization**: OIDC-delegated authentication, user profiles, and organization accounts/memberships (for roasteries, cafes, suppliers).
   - **Media**: private-by-default asset metadata and short-lived signed upload/download URLs.
   - **Audit**: security and business audit records for sensitive actions.

## Progressive Capabilities (Post-MVP / Phased)
- Community (posts, comments, follows) and Reviews (ratings, eligibility).
- Catalog (commercial products, SKUs, seller listings).
- B2B Network (business discovery, inquiries, RFQs, quotations).
- Order & Payment (cart/order lifecycle, hosted/tokenized payment provider integration).
- Events & Academy, Business Tools, Notification, Moderation.

## Explicitly Out of Initial Scope
- Real-money payment processing and full marketplace settlement.
- 17 independently deployed microservices.
- Kubernetes, Kafka/Redpanda, or Temporal (until a demonstrated use case requires them).

## Unanswered Product Questions (Requires Owner Input)
1. **Consumer vs. B2B Launch Order**: Should Phase 1 launch *purely* consumer-facing (Coffee Knowledge + Brew Studio) with Organization accounts deferred to Phase 2, or are basic Roastery/Cafe profiles needed on Day 1?
2. **Offline Support**: Does the React Native mobile app require offline-first brew logging and recipe access in MVP, or is online-with-local-cache sufficient?
3. **Content Curation**: Who authors initial Coffee Knowledge entries—internal editorial/admin only, or community-contributed with moderation?
4. **Multi-language**: Is MVP English-only, Indonesian-only, or bilingual (EN/ID) from Phase 1?
5. **Public vs. Authenticated Access**: Can anonymous users browse Coffee Knowledge and public Brew Recipes without signing in?
