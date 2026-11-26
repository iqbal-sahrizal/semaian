# Semaian — Agent Instructions

## Product context
Semaian is a coffee ecosystem platform, not merely a B2B marketplace. It serves coffee beginners and enthusiasts as well as coffee professionals and businesses. Core consumer value includes coffee education, origin and flavor discovery, brewing guidance, recipes, and community. B2B connections and commerce can be introduced progressively.

## Locked engineering decisions
- Backend language: Go.
- Frontend: Next.js for web; React Native for mobile.
- Initial backend deployment: one modular monolith, with enforceable domain boundaries.
- Database: PostgreSQL.
- Local infrastructure: Docker Compose.
- API contracts: OpenAPI.
- AI environment: Antigravity IDE / CLI.
- Free-first: no paid cloud resources, subscriptions, or usage-based services without explicit owner approval.
- Future service extraction must preserve domain ownership and introduce a separately owned database for each extracted service.

## Architecture rules
1. Do not create 17 deployable microservices at project initialization.
2. Keep domain modules isolated inside the modular monolith.
3. No domain module may query another module's tables directly.
4. Cross-domain interaction must use an explicitly defined application interface or internal event contract.
5. Do not share business entities or business rules through generic shared packages.
6. Keep dependencies pointing inward: transport/adapters -> application -> domain.
7. Use interfaces at real boundaries; do not create interfaces for every struct.
8. Database schema changes require migrations and tests.
9. External API contracts must be versioned and documented.
10. Never claim physical database isolation for a single shared PostgreSQL instance.

## Security rules
- Never commit secrets, tokens, private keys, or production credentials.
- Never place privileged credentials in frontend code.
- Validate authentication and authorization server-side.
- Enforce resource ownership and organization membership in the owning domain.
- Validate inputs at transport boundaries and enforce business invariants in domain/application logic.
- Use parameterized SQL; never concatenate untrusted values into SQL.
- Do not log passwords, tokens, payment credentials, or sensitive personal data.
- Payment integration must use a compliant provider flow; never store raw card PAN/CVV.
- Do not expose admin operations without explicit authorization tests.
- No production deployment, paid account creation, or destructive migration without owner approval.

## Agent workflow
- Architect coordinates and owns architecture consistency.
- Backend Go owns backend implementation.
- Frontend owns web/mobile implementation.
- Security Auditor performs independent, preferably read-only reviews.
- QA Engineer owns test strategy and verification.
- Parallelize only independent tasks. Use separate Git worktrees when agents may edit concurrently.
- Do not have multiple agents edit the same files concurrently.
- Report actual commands and outputs; never invent test results.

## Required completion report
Every task report must include:
- Scope and assumptions
- Decisions made
- Files created/changed
- Commands executed and real results
- Tests and validation evidence
- Security considerations
- Risks / unresolved questions
- Verdict: PASS, PASS_WITH_DEBT, BLOCKED, or FAIL

## Execution policy
Start with Phase 0 architecture governance only. Do not implement product features until the owner approves Phase 0.
