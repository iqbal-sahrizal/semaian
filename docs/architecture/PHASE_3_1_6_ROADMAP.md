# Phase 3.1.6 Roadmap: Post-Knowledge Next Stages

## Recommended Next Phase: Phase 3.1.7 (Operational Hardening & Seed Tooling) — Optional, or Proceed to Phase 4
1. **`P3.1.7-01` (`P1`)**: Transactional `audit.Recorder` (`TxRecorder`) so `audit.events` and entity mutations share one DB transaction.
2. **`P3.1.7-02` (`P1`)**: Live GitHub Actions CI verification (Spectral OpenAPI lint, `gitleaks`, `govulncheck`, `-tags=integration`).
3. **`P3.1.7-03` (`P3`)**: Idempotent, provenance-checked initial Indonesian/English coffee knowledge seed CLI.

## Phase 4 (Requires Explicit Owner Approval)
- **Brew Studio (`brew` module)**: Personal brew recipes, brew logs, equipment, and timers.
- **Discovery (`discovery` module)**: Cross-module read projections with ACL filtering.
- **Web & Mobile Clients**: Next.js web & React Native mobile consuming `docs/openapi/v1/openapi.yaml`.
