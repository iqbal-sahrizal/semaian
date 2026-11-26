-- Phase 3.1.1: Core Structured Coffee Knowledge tables, hierarchy cycle prevention,
-- polymorphic translation referential integrity, and least-privilege grants.

CREATE TABLE IF NOT EXISTS knowledge.countries (
    id UUID PRIMARY KEY,
    iso_code TEXT NOT NULL UNIQUE CHECK (char_length(iso_code) = 2 AND iso_code = upper(iso_code)),
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge.regions (
    id UUID PRIMARY KEY,
    country_id UUID NOT NULL REFERENCES knowledge.countries(id) ON DELETE RESTRICT,
    parent_id UUID REFERENCES knowledge.regions(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL,
    min_masl INT CHECK (min_masl IS NULL OR (min_masl >= 0 AND min_masl <= 6000)),
    max_masl INT CHECK (max_masl IS NULL OR (max_masl >= 0 AND max_masl <= 6000)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (country_id, slug),
    CHECK (parent_id IS NULL OR parent_id <> id),
    CHECK (min_masl IS NULL OR max_masl IS NULL OR min_masl <= max_masl)
);
CREATE INDEX IF NOT EXISTS idx_regions_country_id ON knowledge.regions(country_id);
CREATE INDEX IF NOT EXISTS idx_regions_parent_id ON knowledge.regions(parent_id);

CREATE TABLE IF NOT EXISTS knowledge.producers (
    id UUID PRIMARY KEY,
    region_id UUID NOT NULL REFERENCES knowledge.regions(id) ON DELETE RESTRICT,
    producer_type TEXT NOT NULL CHECK (producer_type IN ('farm', 'estate', 'cooperative', 'washing_station')),
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_producers_region_id ON knowledge.producers(region_id);

CREATE TABLE IF NOT EXISTS knowledge.species (
    id UUID PRIMARY KEY,
    scientific_name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge.varieties (
    id UUID PRIMARY KEY,
    species_id UUID NOT NULL REFERENCES knowledge.species(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_varieties_species_id ON knowledge.varieties(species_id);

CREATE TABLE IF NOT EXISTS knowledge.processing_methods (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge.roast_levels (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge.flavor_descriptors (
    id UUID PRIMARY KEY,
    parent_id UUID REFERENCES knowledge.flavor_descriptors(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX IF NOT EXISTS idx_flavor_descriptors_parent_id ON knowledge.flavor_descriptors(parent_id);

-- Relationship / join tables
CREATE TABLE IF NOT EXISTS knowledge.region_varieties (
    id UUID PRIMARY KEY,
    region_id UUID NOT NULL REFERENCES knowledge.regions(id) ON DELETE CASCADE,
    variety_id UUID NOT NULL REFERENCES knowledge.varieties(id) ON DELETE CASCADE,
    confidence TEXT NOT NULL DEFAULT 'medium' CHECK (confidence IN ('high', 'medium', 'exploratory')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (region_id, variety_id)
);

CREATE TABLE IF NOT EXISTS knowledge.variety_flavors (
    id UUID PRIMARY KEY,
    variety_id UUID NOT NULL REFERENCES knowledge.varieties(id) ON DELETE CASCADE,
    flavor_descriptor_id UUID NOT NULL REFERENCES knowledge.flavor_descriptors(id) ON DELETE CASCADE,
    confidence TEXT NOT NULL DEFAULT 'medium' CHECK (confidence IN ('high', 'medium', 'exploratory')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (variety_id, flavor_descriptor_id)
);

CREATE TABLE IF NOT EXISTS knowledge.process_flavors (
    id UUID PRIMARY KEY,
    processing_method_id UUID NOT NULL REFERENCES knowledge.processing_methods(id) ON DELETE CASCADE,
    flavor_descriptor_id UUID NOT NULL REFERENCES knowledge.flavor_descriptors(id) ON DELETE CASCADE,
    confidence TEXT NOT NULL DEFAULT 'medium' CHECK (confidence IN ('high', 'medium', 'exploratory')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (processing_method_id, flavor_descriptor_id)
);

-- Canonical polymorphic translations table
CREATE TABLE IF NOT EXISTS knowledge.translations (
    id UUID PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN (
        'country', 'region', 'producer', 'species', 'variety',
        'processing_method', 'roast_level', 'flavor_descriptor'
    )),
    entity_id UUID NOT NULL,
    locale TEXT NOT NULL CHECK (locale IN ('id', 'en')),
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_type, entity_id, locale)
);
CREATE INDEX IF NOT EXISTS idx_translations_lookup ON knowledge.translations(entity_type, entity_id, locale);

-- Cycle prevention for hierarchical regions and flavor_descriptors
CREATE OR REPLACE FUNCTION knowledge.prevent_hierarchy_cycle() RETURNS TRIGGER AS $$
DECLARE
    cur_id UUID;
    depth INT := 0;
BEGIN
    cur_id := NEW.parent_id;
    WHILE cur_id IS NOT NULL LOOP
        IF cur_id = NEW.id THEN
            RAISE EXCEPTION 'hierarchy cycle detected for %', NEW.id;
        END IF;
        depth := depth + 1;
        IF depth > 32 THEN
            RAISE EXCEPTION 'hierarchy depth exceeded';
        END IF;
        IF TG_TABLE_NAME = 'regions' THEN
            SELECT parent_id INTO cur_id FROM knowledge.regions WHERE id = cur_id;
        ELSIF TG_TABLE_NAME = 'flavor_descriptors' THEN
            SELECT parent_id INTO cur_id FROM knowledge.flavor_descriptors WHERE id = cur_id;
        ELSE
            cur_id := NULL;
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_regions_prevent_cycle
    BEFORE INSERT OR UPDATE OF parent_id ON knowledge.regions
    FOR EACH ROW EXECUTE FUNCTION knowledge.prevent_hierarchy_cycle();

CREATE TRIGGER trg_flavor_descriptors_prevent_cycle
    BEFORE INSERT OR UPDATE OF parent_id ON knowledge.flavor_descriptors
    FOR EACH ROW EXECUTE FUNCTION knowledge.prevent_hierarchy_cycle();

-- Polymorphic referential integrity for knowledge.translations
CREATE OR REPLACE FUNCTION knowledge.enforce_translation_parent() RETURNS TRIGGER AS $$
DECLARE
    parent_exists BOOLEAN := FALSE;
BEGIN
    CASE NEW.entity_type
        WHEN 'country' THEN SELECT EXISTS(SELECT 1 FROM knowledge.countries WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'region' THEN SELECT EXISTS(SELECT 1 FROM knowledge.regions WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'producer' THEN SELECT EXISTS(SELECT 1 FROM knowledge.producers WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'species' THEN SELECT EXISTS(SELECT 1 FROM knowledge.species WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'variety' THEN SELECT EXISTS(SELECT 1 FROM knowledge.varieties WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'processing_method' THEN SELECT EXISTS(SELECT 1 FROM knowledge.processing_methods WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'roast_level' THEN SELECT EXISTS(SELECT 1 FROM knowledge.roast_levels WHERE id = NEW.entity_id) INTO parent_exists;
        WHEN 'flavor_descriptor' THEN SELECT EXISTS(SELECT 1 FROM knowledge.flavor_descriptors WHERE id = NEW.entity_id) INTO parent_exists;
        ELSE parent_exists := FALSE;
    END CASE;

    IF NOT parent_exists THEN
        RAISE EXCEPTION 'orphan translation: % % does not exist', NEW.entity_type, NEW.entity_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_translations_enforce_parent
    BEFORE INSERT OR UPDATE OF entity_type, entity_id ON knowledge.translations
    FOR EACH ROW EXECUTE FUNCTION knowledge.enforce_translation_parent();

CREATE OR REPLACE FUNCTION knowledge.cascade_delete_translations() RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM knowledge.translations WHERE entity_type = TG_ARGV[0] AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_countries_del_translations AFTER DELETE ON knowledge.countries FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('country');
CREATE TRIGGER trg_regions_del_translations AFTER DELETE ON knowledge.regions FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('region');
CREATE TRIGGER trg_producers_del_translations AFTER DELETE ON knowledge.producers FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('producer');
CREATE TRIGGER trg_species_del_translations AFTER DELETE ON knowledge.species FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('species');
CREATE TRIGGER trg_varieties_del_translations AFTER DELETE ON knowledge.varieties FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('variety');
CREATE TRIGGER trg_processing_methods_del_translations AFTER DELETE ON knowledge.processing_methods FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('processing_method');
CREATE TRIGGER trg_roast_levels_del_translations AFTER DELETE ON knowledge.roast_levels FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('roast_level');
CREATE TRIGGER trg_flavor_descriptors_del_translations AFTER DELETE ON knowledge.flavor_descriptors FOR EACH ROW EXECUTE FUNCTION knowledge.cascade_delete_translations('flavor_descriptor');

-- Least-privilege runtime grants (DML only, no DDL, no PUBLIC)
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA knowledge TO semaian_runtime;
