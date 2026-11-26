# ADR 0002: Database Isolation Evolution

- Status: Accepted for MVP; future target requires review
- Decision: MVP may use one PostgreSQL deployment with domain-owned tables. This does not satisfy physical isolation.
- Future target: each extracted service owns a separately provisioned database deployment/project and credentials.
- Consequences: migration and service extraction require explicit data ownership and migration plans.
- Constraint: never describe schema separation as physical isolation.
