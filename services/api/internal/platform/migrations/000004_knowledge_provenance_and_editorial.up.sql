-- Phase 3.1.2: Provenance (sources, claims) and Editorial (articles, revisions, article_entities).

CREATE TABLE IF NOT EXISTS knowledge.sources (
    id UUID PRIMARY KEY,
    source_type TEXT NOT NULL CHECK (source_type IN ('academic', 'book', 'industry_standard', 'government', 'original', 'web')),
    title TEXT NOT NULL,
    canonical_url TEXT NOT NULL DEFAULT '',
    publisher TEXT NOT NULL DEFAULT '',
    license_type TEXT NOT NULL CHECK (license_type IN ('original', 'public_domain', 'cc0', 'cc_by', 'permissive', 'restricted', 'unknown')),
    commercial_use_compatible BOOLEAN NOT NULL DEFAULT FALSE,
    attribution_text TEXT NOT NULL DEFAULT '',
    verification_status TEXT NOT NULL DEFAULT 'draft' CHECK (verification_status IN ('draft', 'pending_review', 'verified', 'rejected', 'archived')),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge.claims (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES knowledge.sources(id) ON DELETE RESTRICT,
    subject_type TEXT NOT NULL,
    subject_id UUID NOT NULL,
    claim_text TEXT NOT NULL,
    verification_status TEXT NOT NULL DEFAULT 'draft' CHECK (verification_status IN ('draft', 'pending_review', 'verified', 'rejected', 'archived')),
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_claims_source_id ON knowledge.claims(source_id);
CREATE INDEX IF NOT EXISTS idx_claims_subject ON knowledge.claims(subject_type, subject_id);

CREATE TABLE IF NOT EXISTS knowledge.articles (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'under_review', 'approved', 'published', 'superseded', 'archived')),
    author_id UUID NOT NULL,
    published_revision_id UUID,
    superseded_by_article_id UUID REFERENCES knowledge.articles(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge.revisions (
    id UUID PRIMARY KEY,
    article_id UUID NOT NULL REFERENCES knowledge.articles(id) ON DELETE RESTRICT,
    revision_number INT NOT NULL CHECK (revision_number > 0),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    author_id UUID NOT NULL,
    reviewer_id UUID,
    review_decision TEXT NOT NULL DEFAULT 'pending' CHECK (review_decision IN ('pending', 'approved', 'rejected')),
    rejection_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (article_id, revision_number)
);
CREATE INDEX IF NOT EXISTS idx_revisions_article_id ON knowledge.revisions(article_id, revision_number DESC);

CREATE TABLE IF NOT EXISTS knowledge.article_entities (
    id UUID PRIMARY KEY,
    article_id UUID NOT NULL REFERENCES knowledge.articles(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (article_id, entity_type, entity_id)
);

-- Prevent mutation of historical revision content or deletion of revisions.
CREATE OR REPLACE FUNCTION knowledge.protect_revision_history() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'historical revisions cannot be deleted';
    END IF;
    IF OLD.title <> NEW.title OR OLD.body <> NEW.body OR OLD.author_id <> NEW.author_id OR OLD.revision_number <> NEW.revision_number THEN
        RAISE EXCEPTION 'revision content and author are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_revisions_immutable
    BEFORE UPDATE OR DELETE ON knowledge.revisions
    FOR EACH ROW EXECUTE FUNCTION knowledge.protect_revision_history();

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA knowledge TO semaian_runtime;
