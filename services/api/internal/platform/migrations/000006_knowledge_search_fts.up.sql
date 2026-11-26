-- Phase 3.1.3: PostgreSQL Full-Text Search vector and GIN index on knowledge.translations.
ALTER TABLE knowledge.translations
    ADD COLUMN IF NOT EXISTS search_vector TSVECTOR
    GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(summary, '') || ' ' || coalesce(body, ''))
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_translations_search_vector
    ON knowledge.translations USING GIN (search_vector)
    WHERE status = 'published';
