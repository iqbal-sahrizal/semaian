//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/platform/db"
)

func TestProvenanceAndEditorialPGXIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	auditRec := audit.NewPGXRecorder(pool)
	svc := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool), auditRec)

	editorID := uuid.NewString()
	otherEditorID := uuid.NewString()
	reviewerID := uuid.NewString()
	adminID := uuid.NewString()

	validSrc, err := svc.CreateSource(ctx, editorID, knowledge.RoleKnowledgeEditor, knowledge.Source{
		SourceType:              "academic",
		Title:                   "Arabica Genetics",
		LicenseType:             knowledge.LicenseCCBY,
		CommercialUseCompatible: true,
		AttributionText:         "WCR 2024",
	}, "cid-ed-1")
	if err != nil {
		t.Fatalf("CreateSource valid failed: %v", err)
	}

	restrictedSrc, err := svc.CreateSource(ctx, editorID, knowledge.RoleKnowledgeEditor, knowledge.Source{
		SourceType:              "web",
		Title:                   "Restricted Blog",
		LicenseType:             knowledge.LicenseRestricted,
		CommercialUseCompatible: false,
	}, "cid-ed-2")
	if err != nil {
		t.Fatalf("CreateSource restricted failed: %v", err)
	}

	art, rev, err := svc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "arabica-guide-"+uuid.NewString()[:6], "Arabica Guide", "Body v1", "cid-ed-3")
	if err != nil {
		t.Fatalf("CreateArticle failed: %v", err)
	}

	// Revision immutability trigger
	if _, err := pool.Exec(ctx, `UPDATE knowledge.revisions SET body = 'Tampered' WHERE id = $1`, rev.ID); err == nil {
		t.Fatal("expected revision content mutation to be blocked by trigger")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM knowledge.revisions WHERE id = $1`, rev.ID); err == nil {
		t.Fatal("expected revision deletion to be blocked by trigger")
	}

	// SEC-02: Other editor cannot submit someone else's draft
	if _, err := svc.Submit(ctx, otherEditorID, knowledge.RoleKnowledgeEditor, art.ID, "cid-ed-4"); !errors.Is(err, knowledge.ErrForbidden) {
		t.Fatalf("expected ErrForbidden when other editor submits draft, got %v", err)
	}

	// Author submits draft
	if _, err := svc.Submit(ctx, editorID, knowledge.RoleKnowledgeEditor, art.ID, "cid-ed-5"); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// QA-01: Rejection without reason fails; rejection with reason returns article to draft
	if _, _, err := svc.Review(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, knowledge.DecisionRejected, "", "cid-ed-6"); !errors.Is(err, knowledge.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for rejection without reason, got %v", err)
	}
	if _, _, err := svc.Review(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, knowledge.DecisionRejected, "Needs citations", "cid-ed-7"); err != nil {
		t.Fatalf("Review reject failed: %v", err)
	}
	// Rejected revision cannot be published
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-ed-8"); !errors.Is(err, knowledge.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition publishing rejected revision, got %v", err)
	}

	// Create a new article for approval & provenance tests
	art2, rev2, err := svc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "arabica-guide-2-"+uuid.NewString()[:6], "Arabica Guide 2", "Body v2", "cid-ed-9")
	if err != nil {
		t.Fatalf("CreateArticle 2 failed: %v", err)
	}
	if _, err := svc.Submit(ctx, editorID, knowledge.RoleKnowledgeEditor, art2.ID, "cid-ed-10"); err != nil {
		t.Fatalf("Submit 2 failed: %v", err)
	}
	if _, _, err := svc.Review(ctx, editorID, knowledge.RoleKnowledgeReviewer, art2.ID, rev2.ID, knowledge.DecisionApproved, "", "cid-ed-11"); !errors.Is(err, knowledge.ErrSelfApproval) {
		t.Fatalf("expected ErrSelfApproval, got %v", err)
	}
	if _, _, err := svc.Review(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art2.ID, rev2.ID, knowledge.DecisionApproved, "", "cid-ed-12"); err != nil {
		t.Fatalf("Review approve 2 failed: %v", err)
	}

	// Finalized review decision cannot be rewritten by DB UPDATE
	if _, err := pool.Exec(ctx, `UPDATE knowledge.revisions SET review_decision = 'rejected' WHERE id = $1`, rev2.ID); err == nil {
		t.Fatal("expected finalized review_decision overwrite to be blocked by trigger")
	}

	// SEC-01: Publish blocked when no claims/sources exist
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art2.ID, rev2.ID, "cid-ed-13"); !errors.Is(err, knowledge.ErrLicenseBlocked) {
		t.Fatalf("expected ErrLicenseBlocked with no claims, got %v", err)
	}

	// SEC-01: Publish blocked when linked to restricted source
	restrictedClaim, err := svc.CreateClaim(ctx, editorID, knowledge.RoleKnowledgeEditor, knowledge.Claim{
		SourceID:    restrictedSrc.ID,
		SubjectType: "article",
		SubjectID:   art2.ID,
		ClaimText:   "Restricted claim.",
	}, "cid-ed-14")
	if err != nil {
		t.Fatalf("CreateClaim restricted failed: %v", err)
	}
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art2.ID, rev2.ID, "cid-ed-15"); !errors.Is(err, knowledge.ErrLicenseBlocked) {
		t.Fatalf("expected ErrLicenseBlocked with restricted source, got %v", err)
	}

	// Remove restricted claim, link valid CC BY claim, and publish
	_, _ = pool.Exec(ctx, `DELETE FROM knowledge.claims WHERE id = $1`, restrictedClaim.ID)
	if _, err := svc.CreateClaim(ctx, editorID, knowledge.RoleKnowledgeEditor, knowledge.Claim{
		SourceID:    validSrc.ID,
		SubjectType: "article",
		SubjectID:   art2.ID,
		ClaimText:   "Bourbon is a parent of Caturra.",
	}, "cid-ed-16"); err != nil {
		t.Fatalf("CreateClaim valid failed: %v", err)
	}

	pubArt, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art2.ID, rev2.ID, "cid-ed-17")
	if err != nil || pubArt.Status != knowledge.ArticlePublished {
		t.Fatalf("Publish failed: err=%v status=%v", err, pubArt.Status)
	}

	// Emergency unpublish by super_admin
	unpubArt, err := svc.EmergencyUnpublish(ctx, adminID, knowledge.RoleSuperAdmin, art2.ID, "cid-ed-18")
	if err != nil || unpubArt.Status != knowledge.ArticleArchived {
		t.Fatalf("EmergencyUnpublish failed: err=%v status=%v", err, unpubArt.Status)
	}
}

func TestPublishRejectsNonApprovedRevisions(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	svc := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool), audit.NewPGXRecorder(pool))
	editorID := uuid.NewString()
	reviewerID := uuid.NewString()

	art, rev, err := svc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "pending-rev-"+uuid.NewString()[:6], "Pending", "Body", "cid-p-1")
	if err != nil {
		t.Fatalf("CreateArticle failed: %v", err)
	}

	// Draft (pending revision) cannot be published
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-p-2"); !errors.Is(err, knowledge.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition for draft, got %v", err)
	}

	// Submitted (pending revision) cannot be published
	if _, err := svc.Submit(ctx, editorID, knowledge.RoleKnowledgeEditor, art.ID, "cid-p-3"); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-p-4"); !errors.Is(err, knowledge.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition for submitted, got %v", err)
	}

	// Under_review (pending revision) cannot be published
	if _, err := pool.Exec(ctx, `UPDATE knowledge.articles SET status = 'under_review' WHERE id = $1`, art.ID); err != nil {
		t.Fatalf("set under_review failed: %v", err)
	}
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-p-5"); !errors.Is(err, knowledge.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition for under_review, got %v", err)
	}

	var status string
	var pubRev *string
	if err := pool.QueryRow(ctx, `SELECT status, published_revision_id FROM knowledge.articles WHERE id = $1`, art.ID).Scan(&status, &pubRev); err != nil {
		t.Fatalf("query article failed: %v", err)
	}
	if status != "under_review" || pubRev != nil {
		t.Fatalf("expected unchanged state (under_review, nil), got %q %v", status, pubRev)
	}
}

func TestPublishTransactionRollbackOnFailure(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	svc := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool), audit.NewPGXRecorder(pool))
	editorID := uuid.NewString()
	reviewerID := uuid.NewString()

	art, rev, err := svc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "rollback-"+uuid.NewString()[:6], "Rollback", "Body", "cid-rb-1")
	if err != nil {
		t.Fatalf("CreateArticle failed: %v", err)
	}
	if _, err := svc.Submit(ctx, editorID, knowledge.RoleKnowledgeEditor, art.ID, "cid-rb-2"); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if _, _, err := svc.Review(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, knowledge.DecisionApproved, "", "cid-rb-3"); err != nil {
		t.Fatalf("Review failed: %v", err)
	}

	// Fails provenance check (0 claims) -> must roll back completely
	if _, err := svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-rb-4"); !errors.Is(err, knowledge.ErrLicenseBlocked) {
		t.Fatalf("expected ErrLicenseBlocked, got %v", err)
	}

	var status string
	var pubRev *string
	_ = pool.QueryRow(ctx, `SELECT status, published_revision_id FROM knowledge.articles WHERE id = $1`, art.ID).Scan(&status, &pubRev)
	if status != "approved" || pubRev != nil {
		t.Fatalf("expected article to remain approved with nil published_revision_id, got %q %v", status, pubRev)
	}
	var pubAuditCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE target_id = $1 AND action = 'knowledge.article.published'`, art.ID).Scan(&pubAuditCount)
	if pubAuditCount != 0 {
		t.Fatalf("expected 0 published audit events on rollback, got %d", pubAuditCount)
	}
}

func TestConcurrentPublishAndProvenanceLocking(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	svc := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool), audit.NewPGXRecorder(pool))
	editorID := uuid.NewString()
	reviewerID := uuid.NewString()

	src, err := svc.CreateSource(ctx, editorID, knowledge.RoleKnowledgeEditor, knowledge.Source{
		SourceType: "original", Title: "Semaian Original", LicenseType: knowledge.LicenseOriginal, CommercialUseCompatible: true,
	}, "cid-cc-1")
	if err != nil {
		t.Fatalf("CreateSource failed: %v", err)
	}
	art, rev, err := svc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "concurrent-"+uuid.NewString()[:6], "Concurrent", "Body", "cid-cc-2")
	if err != nil {
		t.Fatalf("CreateArticle failed: %v", err)
	}
	if _, err := svc.CreateClaim(ctx, editorID, knowledge.RoleKnowledgeEditor, knowledge.Claim{
		SourceID: src.ID, SubjectType: "article", SubjectID: art.ID, ClaimText: "Original claim.",
	}, "cid-cc-3"); err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}
	if _, err := svc.Submit(ctx, editorID, knowledge.RoleKnowledgeEditor, art.ID, "cid-cc-4"); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if _, _, err := svc.Review(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, knowledge.DecisionApproved, "", "cid-cc-5"); err != nil {
		t.Fatalf("Review failed: %v", err)
	}

	// Two concurrent Publish calls: exactly 1 succeeds, 1 receives ErrInvalidTransition.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.Publish(ctx, reviewerID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-cc-pub")
		}(i)
	}
	wg.Wait()

	successes, transitions := 0, 0
	for _, e := range errs {
		if e == nil {
			successes++
		} else if errors.Is(e, knowledge.ErrInvalidTransition) {
			transitions++
		} else {
			t.Fatalf("unexpected concurrent publish error: %v", e)
		}
	}
	if successes != 1 || transitions != 1 {
		t.Fatalf("expected 1 success and 1 ErrInvalidTransition, got successes=%d transitions=%d", successes, transitions)
	}
}

func TestConcurrentCreateRevision(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	svc := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool), audit.NewPGXRecorder(pool))
	editorID := uuid.NewString()
	otherEditorID := uuid.NewString()

	art, _, err := svc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "rev-conc-"+uuid.NewString()[:6], "Rev 1", "Body 1", "cid-cr-1")
	if err != nil {
		t.Fatalf("CreateArticle failed: %v", err)
	}

	// Other editor cannot create revision on someone else's article
	if _, err := svc.CreateRevision(ctx, otherEditorID, knowledge.RoleKnowledgeEditor, art.ID, "Rev X", "Body X", "cid-cr-2"); !errors.Is(err, knowledge.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for other editor, got %v", err)
	}

	// Concurrent CreateRevision calls by author must serialize cleanly with no 23505 race
	var wg sync.WaitGroup
	revs := make([]knowledge.Revision, 3)
	errs := make([]error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			revs[idx], errs[idx] = svc.CreateRevision(ctx, editorID, knowledge.RoleKnowledgeEditor, art.ID, "Rev Next", "Body Next", "cid-cr-conc")
		}(i)
	}
	wg.Wait()

	seen := make(map[int]bool)
	for i := 0; i < 3; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent CreateRevision %d failed: %v", i, errs[i])
		}
		seen[revs[i].RevisionNumber] = true
	}
	if !seen[2] || !seen[3] || !seen[4] {
		t.Fatalf("expected sequential revision numbers 2, 3, 4; got %v", seen)
	}
}
