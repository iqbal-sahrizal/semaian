-- Rollback Phase 3.1.3 FTS index and generated column.
DROP INDEX IF EXISTS knowledge.idx_translations_search_vector;
ALTER TABLE knowledge.translations DROP COLUMN IF EXISTS search_vector;
