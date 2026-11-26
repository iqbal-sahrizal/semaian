-- Rollback Phase 3.1.1 Core Structured Coffee Knowledge tables and functions.
DROP TABLE IF EXISTS knowledge.translations;
DROP TABLE IF EXISTS knowledge.process_flavors;
DROP TABLE IF EXISTS knowledge.variety_flavors;
DROP TABLE IF EXISTS knowledge.region_varieties;
DROP TABLE IF EXISTS knowledge.flavor_descriptors;
DROP TABLE IF EXISTS knowledge.roast_levels;
DROP TABLE IF EXISTS knowledge.processing_methods;
DROP TABLE IF EXISTS knowledge.varieties;
DROP TABLE IF EXISTS knowledge.species;
DROP TABLE IF EXISTS knowledge.producers;
DROP TABLE IF EXISTS knowledge.regions;
DROP TABLE IF EXISTS knowledge.countries;

DROP FUNCTION IF EXISTS knowledge.cascade_delete_translations();
DROP FUNCTION IF EXISTS knowledge.enforce_translation_parent();
DROP FUNCTION IF EXISTS knowledge.prevent_hierarchy_cycle();
