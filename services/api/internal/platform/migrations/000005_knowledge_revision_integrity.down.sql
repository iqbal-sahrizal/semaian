-- Rollback Phase 3.1.2.1 revision integrity hardening to Phase 3.1.2 behavior.
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
