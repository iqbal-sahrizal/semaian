# ADR 0006: Coffee Knowledge Relational Model & Search Strategy

- **Status**: Proposed (Requires Owner Approval)
- **Context**: Semaian needs a reusable structured coffee knowledge graph, editorial articles, and search without premature infrastructure.
- **Decision**:
  1. Separate **Structured Knowledge** (countries, regions, varieties, processes, roast levels, flavor descriptors) from **Editorial Content** (`articles`) in the `knowledge` PostgreSQL schema.
  2. Model relationships using **PostgreSQL relational join tables** (no Neo4j/graph DB).
  3. Use **PostgreSQL Full-Text Search (`GIN` on `tsvector`)** for initial search (no Elasticsearch/OpenSearch).
- **Consequences**: Zero new infrastructure; strong referential integrity. Revisit if graph traversal depth or search volume exceeds PostgreSQL FTS capacity.
