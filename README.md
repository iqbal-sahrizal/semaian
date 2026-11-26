# Semaian — Project Bootstrap

Semaian is a coffee ecosystem product for coffee lovers, beginners, home brewers, baristas, roasteries, coffee shops, farmers, suppliers, and coffee communities.

This repository is a **bootstrap and architecture foundation**, not a completed product.

## Locked baseline

- Product name: Semaian
- AI development: Google Antigravity IDE / CLI (`agy`)
- Backend: Go
- Frontend: Next.js (web), React Native (mobile)
- Initial backend topology: modular monolith with explicit domain boundaries
- Future direction: extract services only when justified; each extracted service owns its database
- Database: PostgreSQL
- Local development: Docker Compose
- Cloud strategy: free-tier-first, no paid resources without explicit owner approval
- API contracts: OpenAPI
- Observability: OpenTelemetry-compatible interfaces, enabled incrementally

## Start safely

1. Read `AGENTS.md`.
2. Read `.agents/agents/*.md`.
3. Read `docs/architecture/`.
4. Run `docker compose -f infrastructure/local/docker-compose.yml config`.
5. Start only local PostgreSQL:
   `docker compose -f infrastructure/local/docker-compose.yml up -d postgres`
6. Open this folder in Antigravity and run the Phase 0 prompt in `docs/prompts/PHASE_0.md`.

No cloud resources are provisioned by this bootstrap. No paid service should be enabled without explicit approval.

## Repository status

The Go API is a minimal health-check starter. It does not yet implement authentication, authorization, business features, migrations, or production deployment.

## Commands

```bash
make help
make test
make run
make docker-up
make docker-down
```

See `docs/operations/FREE_TIER_POLICY.md` before connecting any cloud provider.
