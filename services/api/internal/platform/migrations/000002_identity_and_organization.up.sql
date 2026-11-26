-- Phase 2: Identity, Organization, and Audit tables.
-- Preserves module schema ownership: no cross-schema foreign keys.

CREATE TABLE IF NOT EXISTS identity.users (
    id UUID PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'deactivated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS identity.external_identities (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE INDEX IF NOT EXISTS idx_external_identities_user_id ON identity.external_identities(user_id);

CREATE TABLE IF NOT EXISTS identity.profiles (
    user_id UUID PRIMARY KEY REFERENCES identity.users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL DEFAULT '',
    avatar_ref TEXT NOT NULL DEFAULT '',
    preferred_locale TEXT NOT NULL DEFAULT 'id',
    timezone TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS organization.organizations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'deactivated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS organization.memberships (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organization.organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    status TEXT NOT NULL CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON organization.memberships(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_memberships_single_active_owner
    ON organization.memberships(organization_id)
    WHERE role = 'owner' AND status = 'active';

CREATE TABLE IF NOT EXISTS audit.events (
    id UUID PRIMARY KEY,
    actor_id UUID,
    organization_id UUID,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_org_created ON audit.events(organization_id, created_at DESC);

-- Re-apply default DML grants for existing tables
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA identity, organization TO semaian_runtime;
GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA audit TO semaian_audit_writer;
