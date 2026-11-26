# ADR 0001: Modular Monolith First

- Status: Accepted
- Decision: Start with one Go modular monolith and explicit domain boundaries.
- Context: Early product stage, limited budget, small team, and need to validate product-market fit.
- Consequences: Lower operational overhead; domain isolation must be enforced through code review and architecture tests. Services can be extracted later when justified.
- Revisit when: independent scaling, release cadence, team ownership, compliance, or failure isolation requires extraction.
