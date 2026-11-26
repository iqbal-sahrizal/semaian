-- Rollback Phase 3.1.2 Provenance and Editorial tables.
DROP TABLE IF EXISTS knowledge.article_entities;
DROP TABLE IF EXISTS knowledge.revisions;
DROP TABLE IF EXISTS knowledge.articles;
DROP TABLE IF EXISTS knowledge.claims;
DROP TABLE IF EXISTS knowledge.sources;
DROP FUNCTION IF EXISTS knowledge.protect_revision_history();
