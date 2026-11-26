# Free-Tier-First Strategy

## Principle
Local development must remain 100% functional without paid services or cloud accounts. Cloud preview environments are optional and must stay within explicitly verified free allowances.

## Local Development Criteria
- **Required**: Docker Compose (`infrastructure/local/docker-compose.yml`) running PostgreSQL 17 (`127.0.0.1:5433`).
- **Optional / Deferred**: Add local OIDC emulator, MinIO (object storage), or Redis only when an approved Phase 1+ feature requires them.
- **Zero Cloud Dependency**: `make docker-up && make run` must work offline once dependencies are cached.

## Free-Tier Cloud Criteria (Before Any Provisioning)
Provider choices are proposals, not approvals. Before connecting any cloud provider:
1. Verify current free-plan terms and commercial-use restrictions.
2. Verify whether a payment method / credit card is required.
3. Document sleep/cold-start, storage, bandwidth, build-minute, and request limits.
4. Configure usage alerts and hard spending caps ($0) where available.
5. Never enable paid overages without explicit owner approval.
6. Document export/migration paths to avoid vendor lock-in.
