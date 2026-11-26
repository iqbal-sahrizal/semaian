# Phase 3.1.6 Acceptance Criteria for Next Phase

## Phase 3.1.7 (If Approved)
1. **Transactional Audit**: Injecting an `audit.events` failure rolls back the corresponding `knowledge.*` mutation (`0` orphan rows, `0` orphan audit events).
2. **Live CI**: `.github/workflows/ci.yml` passes on GitHub Actions (`gofmt`, `go build`, `go vet`, `go test -race -tags=integration`, Spectral OpenAPI lint, `govulncheck`, `gitleaks`).
3. **Seed CLI**: Running the seed command twice is idempotent (`0` duplicate errors) and every seeded claim links to a `commercial_use_compatible = true` source.
