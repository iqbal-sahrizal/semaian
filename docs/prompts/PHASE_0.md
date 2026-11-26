# Antigravity Prompt — Phase 0

Read `AGENTS.md`, all custom agent definitions under `.agents/agents/`, and all documents under `docs/architecture/`.

You are initializing Semaian. Start with repository inspection and Phase 0 Architecture Governance only.

Tasks:
1. Inspect every existing file and report the current state.
2. Validate the proposed product scope and identify unanswered product questions.
3. Review bounded contexts and identify overlaps or missing ownership.
4. Define MVP domain modules and their dependency rules.
5. Define initial PostgreSQL ownership and the future physical-isolation migration strategy.
6. Define authentication provider requirements without silently choosing a paid provider.
7. Define authorization model and cross-organization isolation tests.
8. Define API versioning and OpenAPI generation standards.
9. Define event standards, but do not deploy a broker yet.
10. Define security threat model and privacy/data minimization requirements.
11. Define local development and free-tier cloud criteria.
12. Define test strategy, CI checks, and quality gates.
13. Create `docs/architecture/WORKBOARD.md` with sequenced, independently reviewable tasks.
14. Record unresolved decisions as ADRs with options and trade-offs.

Constraints:
- Do not implement business features.
- Do not provision cloud resources or create accounts.
- Do not deploy anything.
- Do not add paid dependencies or services.
- Do not create 17 independently deployed services.
- Do not claim physical database isolation for the MVP.
- Do not modify locked decisions without an ADR and owner approval.
- Use specialist agents for independent reviews; avoid overlapping file edits.
- Do not claim tests passed unless they were executed.

At completion, report files changed, decisions, validation evidence, risks, unresolved questions, and verdict: PASS, PASS_WITH_DEBT, BLOCKED, or FAIL.

Stop and wait for explicit owner approval before Phase 1.
