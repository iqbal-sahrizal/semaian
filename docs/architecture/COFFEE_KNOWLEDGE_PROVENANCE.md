# Coffee Knowledge Provenance, Licensing & Data Quality

## 1. Source & License Classification
Every external factual record or editorial reference must link to `knowledge.sources`:
- `original`: Original Semaian-authored content.
- `public_domain`: Verified public-domain material.
- `cc0`: Creative Commons Zero.
- `cc_by`: Creative Commons Attribution (**requires** `attribution_text` to be populated and displayed).
- `permissive`: Other verified permissive license with documented commercial-use compatibility.
- `restricted`: Unclear or non-commercial license — **blocked from publication**.

## 2. Publication Gate
- A `claim` or `article` cannot transition to `published` if any linked source has `license_type = 'restricted'` or `commercial_use_compatible = false`.
- Reviewers must verify `source_url`, `retrieved_at`, and `evidence_notes` before marking a claim `verified`.
