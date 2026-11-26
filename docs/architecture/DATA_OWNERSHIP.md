# Data Ownership and Isolation (Debt Closed)

## MVP: Single PostgreSQL, Logical Ownership
For MVP, the modular monolith uses a single PostgreSQL 17 instance (`semaian_dev` locally).
**Constraint**: This is a cost-saving MVP compromise. It does **not** provide physical database isolation.

### Schema Ownership, Roles & Audit Append-Only Strategy
1. **Schema-per-Module**: Each domain module owns a dedicated PostgreSQL schema (`identity`, `organization`, `knowledge`, `brew`, `discovery`, `media`, `audit`).
2. **Table & Migration Ownership**: Each module owns its SQL migrations (`services/api/internal/modules/<domain>/migrations/`).
3. **No Cross-Schema Queries or Joins**: Module A must never query Module B's schema directly. Cross-domain references use opaque `uuid`s without cross-schema `FOREIGN KEY` constraints.
4. **Migration vs. Runtime Roles**:
   - **Migration role** (DDL): Used only by migration runners/CI; owns schema creation and `ALTER TABLE`.
   - **Runtime role(s)** (DML): Used by `services/api`; granted `SELECT, INSERT, UPDATE, DELETE` only on owned module schemas (no DDL). In deployed environments, prefer per-module DB roles with schema-scoped `GRANT`s to limit blast radius.
5. **Audit Append-Only Strategy**:
   - The `audit` schema's runtime role is granted `INSERT` and `SELECT` only—**no `UPDATE` or `DELETE`**.
   - Audit records are immutable; corrections are recorded as compensating events.

## Future Physical-Isolation Migration Strategy
When a domain module is extracted into a standalone service:
1. **Provision**: Create a dedicated PostgreSQL instance/project with isolated credentials.
2. **Backfill**: Export schema and data via a reviewed migration plan; verify row counts and checksums.
3. **Contract Cutover**: Replace in-process Go interface calls with versioned OpenAPI or async event contracts.
4. **Reliability**: Introduce transactional outbox/inbox tables and idempotent consumers.
5. **Decommission**: Drop the old schema from the monolith DB only after rollback windows close and backups are verified.
