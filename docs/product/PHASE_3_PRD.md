# Phase 3 PRD: Coffee Knowledge Foundation

## 1. Product Vision & Positioning
Semaian's Coffee Knowledge Foundation is a **reusable, structured coffee knowledge system**—not merely an article CMS. It serves consumers (beginners, enthusiasts, home brewers) and industry participants (baristas, roasteries, cafes, farmers) with accurate, localized (`id` first, `en` ready), and provenance-backed coffee education.

## 2. Core Principles
- **Consumer-first, B2B-ready**: Anonymous public browsing of published knowledge; structured entities reusable by future `catalog`, `brew`, and `b2b` modules.
- **Indonesian-first, English-ready**: All localized fields support `id` and `en` with deterministic fallback.
- **Editorial Curation First**: Phase 3.1+ launches with editorially curated content (`editor` / `reviewer` workflow). Community contributions are deferred.
- **Provenance & Licensing**: Every external factual claim requires source attribution, license compatibility (`public_domain`, `cc0`, `cc_by`, `original`), and commercial-use verification before publication.

## 3. In-Scope (Phase 3)
- **Structured Knowledge**: Countries, Regions/Subregions, Farms/Estates/Cooperatives (optional), Species & Varieties, Processing Methods, Roast Levels, Sensory & Flavor Taxonomy (hierarchical, non-enum), and Brewing Reference Concepts.
- **Editorial Content**: Guides and educational articles linked to structured knowledge entities.
- **Provenance & Editorial Workflow**: Draft → Submitted → Under Review → Approved → Published → Superseded/Archived, with separation of author and reviewer.
- **Public & Editorial APIs**: `/api/v1/knowledge/...` (public read) and `/api/v1/editorial/knowledge/...` (authenticated editorial).

## 4. Out-of-Scope (Phase 3)
- Brew Studio (personal recipes, brew logs, timers) — belongs to `brew` module.
- Commercial SKUs, pricing, or lots/batches for sale — belongs to `catalog` / `b2b`.
- Community comments/reviews, Elasticsearch/OpenSearch, or graph databases.
