-- Phase 1: Module-owned schemas and least-privilege roles.
-- Note: Single PostgreSQL instance is an MVP compromise, NOT physical isolation.

CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS organization;
CREATE SCHEMA IF NOT EXISTS knowledge;
CREATE SCHEMA IF NOT EXISTS brew;
CREATE SCHEMA IF NOT EXISTS discovery;
CREATE SCHEMA IF NOT EXISTS media;
CREATE SCHEMA IF NOT EXISTS audit;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'semaian_runtime') THEN
    CREATE ROLE semaian_runtime NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'semaian_audit_writer') THEN
    CREATE ROLE semaian_audit_writer NOLOGIN;
  END IF;
END
$$;

-- Runtime role: DML only on non-audit schemas, no DDL.
GRANT USAGE ON SCHEMA identity, organization, knowledge, brew, discovery, media TO semaian_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA identity, organization, knowledge, brew, discovery, media TO semaian_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA identity, organization, knowledge, brew, discovery, media
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO semaian_runtime;

-- Audit role: append-only (INSERT and SELECT only, no UPDATE or DELETE).
GRANT USAGE ON SCHEMA audit TO semaian_audit_writer;
GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA audit TO semaian_audit_writer;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit
  GRANT SELECT, INSERT ON TABLES TO semaian_audit_writer;
