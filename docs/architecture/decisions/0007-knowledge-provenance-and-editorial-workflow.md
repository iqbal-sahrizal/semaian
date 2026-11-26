# ADR 0007: Knowledge Provenance, Licensing & Editorial Workflow

- **Status**: Proposed (Requires Owner Approval)
- **Context**: Publicly accessible coffee data is not automatically commercially reusable. Editorial quality requires review separation.
- **Decision**:
  1. Every external factual claim links to `knowledge.sources` with explicit `license_type` and `commercial_use_compatible` flags; `restricted` sources are blocked from publication.
  2. Editorial workflow (`draft` → `submitted` → `under_review` → `approved` → `published` → `superseded`/`archived`) enforces `reviewer_id != author_id` and separates platform editorial roles from `organization` roles.
- **Consequences**: Prevents accidental copyright/licensing violations and self-approval. Revisit when community contributions are introduced.
