-- Rollback Phase 2 Identity, Organization, and Audit tables.
DROP TABLE IF EXISTS audit.events;
DROP TABLE IF EXISTS organization.memberships;
DROP TABLE IF EXISTS organization.organizations;
DROP TABLE IF EXISTS identity.profiles;
DROP TABLE IF EXISTS identity.external_identities;
DROP TABLE IF EXISTS identity.users;
