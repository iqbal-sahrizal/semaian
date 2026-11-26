# Phase 3.1.5 Specification: Knowledge Data Quality & Editorial Operations (Refined)

## 1. Objectives
Complete the operational editorial loop so `knowledge_editor` and `knowledge_reviewer` users can manage all 8 structured coffee entities, add audited translations, and create new article revisions for corrections via HTTP—without bypassing provenance or revision-immutability invariants.

## 2. Structured Entity Scope Decision
- **Included (all 8)**: `countries`, `regions`, `producers`, `species`, `varieties`, `processing_methods`, `roast_levels`, `flavor_descriptors`.
  - *Rationale*: `varieties` requires `species_id` (`NOT NULL FK`), and `producers` is already implemented in `PGXRepository.CreateProducer` (`000003`). Excluding either would leave an incomplete editorial creation flow.

## 3. In-Scope Requirements
1. **Audited `InsertTranslation` (`P3.1.5-01`)**: Route `POST /api/v1/editorial/knowledge/translations` through `EditorialService.InsertTranslation`, deriving `actorID` from `auth.ActorFromContext`, enforcing `RoleKnowledgeEditor`/`RoleKnowledgeReviewer`/`RoleSuperAdmin`, and recording `audit.events` (`entity_type`, `entity_id`, `locale`) only on successful persistence.
2. **Structured Entity Editorial Endpoints (`P3.1.5-02`)**: Expose `POST` endpoints under `/api/v1/editorial/knowledge/...` for all 8 structured entities (`countries`, `regions`, `producers`, `species`, `varieties`, `processing-methods`, `roast-levels`, `flavors`), enforcing `RoleKnowledgeEditor`/`RoleSuperAdmin` and `audit.events`.
3. **Immutable `CreateRevision` (`P3.1.5-03`)**:
   - Lock `knowledge.articles` (`FOR UPDATE`), enforce `author_id == actorID` (for editors) or `RoleSuperAdmin`.
   - Allocate `revision_number = COALESCE(MAX(revision_number), 0) + 1` atomically.
   - Insert new `knowledge.revisions` (`review_decision = 'pending'`) without modifying `published_revision_id` or prior revisions.
   - Emit `audit.events` (`knowledge.revision.created`).
4. **OpenAPI 3.1 & Tests (`P3.1.5-04`)**: Document all new endpoints in `docs/openapi/v1/openapi.yaml` and add unit, PostgreSQL integration (including concurrent `CreateRevision`), and HTTP RBAC tests.

## 4. Acceptance Criteria
- Every new editorial endpoint enforces `401` (unauthenticated) and `403` (`RoleReader`), and emits `audit.events` only on success.
- Concurrent `CreateRevision` calls produce strictly sequential `revision_number`s with zero `23505` race failures.
- `gofmt`, `go build ./...`, `go vet ./...`, and `go test -v -race -tags=integration ./...` pass on `semaian_test` (`exit 0`).
