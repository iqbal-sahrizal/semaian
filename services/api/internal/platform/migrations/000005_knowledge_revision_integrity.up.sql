-- Phase 3.1.2.1: Harden revision history immutability (id, article_id, created_at, and finalized review decisions).
CREATE OR REPLACE FUNCTION knowledge.protect_revision_history() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'historical revisions cannot be deleted';
    END IF;
    IF OLD.id <> NEW.id OR OLD.article_id <> NEW.article_id OR OLD.created_at <> NEW.created_at
       OR OLD.title <> NEW.title OR OLD.body <> NEW.body OR OLD.author_id <> NEW.author_id
       OR OLD.revision_number <> NEW.revision_number THEN
        RAISE EXCEPTION 'revision content, identity, and author are immutable';
    END IF;
    IF OLD.review_decision <> 'pending' AND (OLD.review_decision <> NEW.review_decision OR OLD.reviewer_id IS DISTINCT FROM NEW.reviewer_id) THEN
        RAISE EXCEPTION 'finalized review decision cannot be rewritten';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
