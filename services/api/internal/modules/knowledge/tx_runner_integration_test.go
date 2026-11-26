//go:build integration

package knowledge_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/identity"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/modules/organization"
	"github.com/semaian/platform/services/api/internal/platform/db"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

func TestTxRunnerAtomicAuditRollbackAcrossModules(t *testing.T) {
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

	runner := tx.NewPGXRunner(pool)

	// 1. Identity SetStatus rollback on audit failure
	idRepo := identity.NewPGXRepository(pool)
	u, err := idRepo.ResolveOrCreateByExternal(ctx, "https://issuer.tx", "sub-"+uuid.NewString())
	if err != nil {
		t.Fatalf("ResolveOrCreateByExternal failed: %v", err)
	}
	idSvcFail := identity.NewService(idRepo, failingAuditRecorder{}).WithTxRunner(runner)
	if _, err := idSvcFail.SetStatus(ctx, u.ID, u.ID, identity.StatusSuspended, "cid-tx-1"); err == nil {
		t.Fatal("expected SetStatus to fail when audit fails")
	}
	gotU, _ := idRepo.GetUser(ctx, u.ID)
	if gotU.Status != identity.StatusActive {
		t.Fatalf("expected user status to roll back to active, got %v", gotU.Status)
	}

	// 2. Organization ChangeMemberRole rollback on audit failure
	orgRepo := organization.NewPGXRepository(pool)
	org, _, err := orgRepo.CreateWithOwner(ctx, "TxOrg", "txorg-"+uuid.NewString()[:6], u.ID)
	if err != nil {
		t.Fatalf("CreateWithOwner failed: %v", err)
	}
	u2, _ := idRepo.ResolveOrCreateByExternal(ctx, "https://issuer.tx", "sub-"+uuid.NewString())
	_, _ = pool.Exec(ctx, `INSERT INTO organization.memberships (id, organization_id, user_id, role, status) VALUES ($1, $2, $3, 'member', 'active')`, uuid.NewString(), org.ID, u2.ID)
	orgSvcFail := organization.NewService(orgRepo, failingAuditRecorder{}).WithTxRunner(runner)
	if _, err := orgSvcFail.ChangeMemberRole(ctx, u.ID, org.ID, u2.ID, organization.RoleAdmin, "cid-tx-2"); err == nil {
		t.Fatal("expected ChangeMemberRole to fail when audit fails")
	}
	mem, _ := orgRepo.GetMembership(ctx, org.ID, u2.ID)
	if mem.Role != organization.RoleMember {
		t.Fatalf("expected member role to roll back to member, got %v", mem.Role)
	}

	// 3. Knowledge EmergencyUnpublish rollback on audit failure
	edRepo := knowledge.NewEditorialPGXRepository(pool)
	edSvcOK := knowledge.NewEditorialService(edRepo, audit.NewPGXRecorder(pool)).WithTxRunner(runner)
	src, _ := edSvcOK.CreateSource(ctx, u.ID, knowledge.RoleKnowledgeEditor, knowledge.Source{SourceType: "original", Title: "Orig", LicenseType: knowledge.LicenseOriginal, CommercialUseCompatible: true}, "cid-tx-3")
	art, rev, _ := edSvcOK.CreateArticle(ctx, u.ID, knowledge.RoleKnowledgeEditor, "tx-art-"+uuid.NewString()[:6], "Tx Art", "Body", "cid-tx-4")
	_, _ = edSvcOK.CreateClaim(ctx, u.ID, knowledge.RoleKnowledgeEditor, knowledge.Claim{SourceID: src.ID, SubjectType: "article", SubjectID: art.ID, ClaimText: "Claim"}, "cid-tx-5")
	_, _ = edSvcOK.Submit(ctx, u.ID, knowledge.RoleKnowledgeEditor, art.ID, "cid-tx-6")
	_, _, _ = edSvcOK.Review(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, knowledge.DecisionApproved, "", "cid-tx-7")
	_, _ = edSvcOK.Publish(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art.ID, rev.ID, "cid-tx-8")

	edSvcFail := knowledge.NewEditorialService(edRepo, failingAuditRecorder{}).WithTxRunner(runner)
	if _, err := edSvcFail.EmergencyUnpublish(ctx, u.ID, knowledge.RoleSuperAdmin, art.ID, "cid-tx-9"); err == nil {
		t.Fatal("expected EmergencyUnpublish to fail when audit fails")
	}
	var artStatus string
	_ = pool.QueryRow(ctx, `SELECT status FROM knowledge.articles WHERE id = $1`, art.ID).Scan(&artStatus)
	if artStatus != "published" {
		t.Fatalf("expected article status to roll back to published, got %q", artStatus)
	}

	// 4. Organization TransferOwnership rollback on audit failure
	if err := orgSvcFail.TransferOwnership(ctx, u.ID, org.ID, u2.ID, "cid-tx-10"); err == nil {
		t.Fatal("expected TransferOwnership to fail when audit fails")
	}
	memOwner, _ := orgRepo.GetMembership(ctx, org.ID, u.ID)
	if memOwner.Role != organization.RoleOwner {
		t.Fatalf("expected u1 to remain owner after rollback, got %v", memOwner.Role)
	}

	// 5. Organization RemoveMember rollback on audit failure
	if err := orgSvcFail.RemoveMember(ctx, u.ID, org.ID, u2.ID, "cid-tx-11"); err == nil {
		t.Fatal("expected RemoveMember to fail when audit fails")
	}
	mem2, _ := orgRepo.GetMembership(ctx, org.ID, u2.ID)
	if mem2.Status != organization.MembershipActive {
		t.Fatalf("expected u2 to remain active after rollback, got %v", mem2.Status)
	}

	// 6. Knowledge Publish rollback on audit failure
	artP, revP, _ := edSvcOK.CreateArticle(ctx, u.ID, knowledge.RoleKnowledgeEditor, "tx-pub-"+uuid.NewString()[:6], "Tx Pub", "Body", "cid-tx-12")
	_, _ = edSvcOK.CreateClaim(ctx, u.ID, knowledge.RoleKnowledgeEditor, knowledge.Claim{SourceID: src.ID, SubjectType: "article", SubjectID: artP.ID, ClaimText: "Claim"}, "cid-tx-13")
	_, _ = edSvcOK.Submit(ctx, u.ID, knowledge.RoleKnowledgeEditor, artP.ID, "cid-tx-14")
	_, _, _ = edSvcOK.Review(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, artP.ID, revP.ID, knowledge.DecisionApproved, "", "cid-tx-15")
	if _, err := edSvcFail.Publish(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, artP.ID, revP.ID, "cid-tx-16"); err == nil {
		t.Fatal("expected Publish to fail when audit fails")
	}
	_ = pool.QueryRow(ctx, `SELECT status FROM knowledge.articles WHERE id = $1`, artP.ID).Scan(&artStatus)
	if artStatus != "approved" {
		t.Fatalf("expected artP status to roll back to approved, got %q", artStatus)
	}

	// 7. Knowledge Supersede rollback on audit failure
	if _, err := edSvcFail.Supersede(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art.ID, artP.ID, "cid-tx-17"); err == nil {
		t.Fatal("expected Supersede to fail when audit fails")
	}
	_ = pool.QueryRow(ctx, `SELECT status FROM knowledge.articles WHERE id = $1`, art.ID).Scan(&artStatus)
	if artStatus != "published" {
		t.Fatalf("expected art status to roll back to published, got %q", artStatus)
	}
}

func TestPGXRunnerNestedAndPanicRollback(t *testing.T) {
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

	runner := tx.NewPGXRunner(pool)

	// Nested WithinTx reuses outer tx
	var sameTx bool
	if err := runner.WithinTx(ctx, func(outerCtx context.Context) error {
		outerTx, _ := tx.TxFromContext(outerCtx)
		return runner.WithinTx(outerCtx, func(innerCtx context.Context) error {
			innerTx, _ := tx.TxFromContext(innerCtx)
			sameTx = (outerTx == innerTx)
			return nil
		})
	}); err != nil || !sameTx {
		t.Fatalf("expected nested WithinTx to reuse outer tx, err=%v sameTx=%v", err, sameTx)
	}

	// Panic inside WithinTx rolls back and re-panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate")
		}
	}()
	_ = runner.WithinTx(ctx, func(_ context.Context) error {
		panic("boom")
	})
}

func TestSevenOperationsSuccessAuditConsistencyAndLifecycle(t *testing.T) {
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

	runner := tx.NewPGXRunner(pool)
	rec := audit.NewPGXRecorder(pool)

	// 1. Context cancellation rolls back active transaction
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := runner.WithinTx(cctx, func(_ context.Context) error { return nil }); err == nil {
		t.Fatal("expected canceled context to fail WithinTx")
	}

	// 2. Concurrent requests do not share transaction state
	var wg sync.WaitGroup
	txPtrs := make([]any, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = runner.WithinTx(ctx, func(txCtx context.Context) error {
				tVal, _ := tx.TxFromContext(txCtx)
				txPtrs[idx] = tVal
				return nil
			})
		}(i)
	}
	wg.Wait()
	if txPtrs[0] == nil || txPtrs[1] == nil || txPtrs[0] == txPtrs[1] {
		t.Fatalf("expected distinct non-nil pgx.Tx per concurrent request, got %v %v", txPtrs[0], txPtrs[1])
	}

	// 3. Successful audit consistency across all 7 mandatory operations
	idRepo := identity.NewPGXRepository(pool)
	u1, _ := idRepo.ResolveOrCreateByExternal(ctx, "https://issuer.ok", "sub-"+uuid.NewString())
	u2, _ := idRepo.ResolveOrCreateByExternal(ctx, "https://issuer.ok", "sub-"+uuid.NewString())
	idSvc := identity.NewService(idRepo, rec).WithTxRunner(runner)

	if _, err := idSvc.SetStatus(ctx, u1.ID, u1.ID, identity.StatusSuspended, "cid-ok-1"); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	orgRepo := organization.NewPGXRepository(pool)
	org, _, _ := orgRepo.CreateWithOwner(ctx, "OkOrg", "okorg-"+uuid.NewString()[:6], u1.ID)
	_, _ = pool.Exec(ctx, `INSERT INTO organization.memberships (id, organization_id, user_id, role, status) VALUES ($1, $2, $3, 'member', 'active')`, uuid.NewString(), org.ID, u2.ID)
	orgSvc := organization.NewService(orgRepo, rec).WithTxRunner(runner)

	if _, err := orgSvc.ChangeMemberRole(ctx, u1.ID, org.ID, u2.ID, organization.RoleAdmin, "cid-ok-2"); err != nil {
		t.Fatalf("ChangeMemberRole failed: %v", err)
	}
	if err := orgSvc.TransferOwnership(ctx, u1.ID, org.ID, u2.ID, "cid-ok-3"); err != nil {
		t.Fatalf("TransferOwnership failed: %v", err)
	}
	if err := orgSvc.RemoveMember(ctx, u2.ID, org.ID, u1.ID, "cid-ok-4"); err != nil {
		t.Fatalf("RemoveMember failed: %v", err)
	}

	edRepo := knowledge.NewEditorialPGXRepository(pool)
	edSvc := knowledge.NewEditorialService(edRepo, rec).WithTxRunner(runner)
	src, _ := edSvc.CreateSource(ctx, u1.ID, knowledge.RoleKnowledgeEditor, knowledge.Source{SourceType: "original", Title: "Orig", LicenseType: knowledge.LicenseOriginal, CommercialUseCompatible: true}, "cid-ok-5")
	art1, rev1, _ := edSvc.CreateArticle(ctx, u1.ID, knowledge.RoleKnowledgeEditor, "ok-art1-"+uuid.NewString()[:6], "A1", "B1", "cid-ok-6")
	_, _ = edSvc.CreateClaim(ctx, u1.ID, knowledge.RoleKnowledgeEditor, knowledge.Claim{SourceID: src.ID, SubjectType: "article", SubjectID: art1.ID, ClaimText: "C1"}, "cid-ok-7")
	_, _ = edSvc.Submit(ctx, u1.ID, knowledge.RoleKnowledgeEditor, art1.ID, "cid-ok-8")
	_, _, _ = edSvc.Review(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art1.ID, rev1.ID, knowledge.DecisionApproved, "", "cid-ok-9")
	if _, err := edSvc.Publish(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art1.ID, rev1.ID, "cid-ok-10"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	// Retry Publish after commit -> ErrInvalidTransition, 0 duplicate audit events
	if _, err := edSvc.Publish(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art1.ID, rev1.ID, "cid-ok-10b"); err == nil {
		t.Fatal("expected retry Publish to fail with ErrInvalidTransition")
	}

	art2, _, _ := edSvc.CreateArticle(ctx, u1.ID, knowledge.RoleKnowledgeEditor, "ok-art2-"+uuid.NewString()[:6], "A2", "B2", "cid-ok-11")
	if _, err := edSvc.Supersede(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art1.ID, art2.ID, "cid-ok-12"); err != nil {
		t.Fatalf("Supersede failed: %v", err)
	}
	art3, rev3, _ := edSvc.CreateArticle(ctx, u1.ID, knowledge.RoleKnowledgeEditor, "ok-art3-"+uuid.NewString()[:6], "A3", "B3", "cid-ok-13")
	_, _ = edSvc.CreateClaim(ctx, u1.ID, knowledge.RoleKnowledgeEditor, knowledge.Claim{SourceID: src.ID, SubjectType: "article", SubjectID: art3.ID, ClaimText: "C3"}, "cid-ok-14")
	_, _ = edSvc.Submit(ctx, u1.ID, knowledge.RoleKnowledgeEditor, art3.ID, "cid-ok-15")
	_, _, _ = edSvc.Review(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art3.ID, rev3.ID, knowledge.DecisionApproved, "", "cid-ok-16")
	_, _ = edSvc.Publish(ctx, u2.ID, knowledge.RoleKnowledgeReviewer, art3.ID, rev3.ID, "cid-ok-17")
	if _, err := edSvc.EmergencyUnpublish(ctx, u1.ID, knowledge.RoleSuperAdmin, art3.ID, "cid-ok-18"); err != nil {
		t.Fatalf("EmergencyUnpublish failed: %v", err)
	}

	// Assert exactly 1 audit event for each of the 7 operations
	checks := []struct {
		action string
		target string
	}{
		{"identity.status.suspended", u1.ID},
		{"organization.member.role_changed", ""},
		{"organization.ownership.transferred", u2.ID},
		{"organization.member.removed", u1.ID},
		{"knowledge.article.published", art1.ID},
		{"knowledge.article.superseded", art1.ID},
		{"knowledge.article.emergency_unpublished", art3.ID},
	}
	for _, c := range checks {
		var cnt int
		if c.target != "" {
			_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE action = $1 AND target_id = $2`, c.action, c.target).Scan(&cnt)
		} else {
			_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE action = $1 AND organization_id = $2`, c.action, org.ID).Scan(&cnt)
		}
		if cnt != 1 {
			t.Fatalf("expected 1 audit event for %s, got %d", c.action, cnt)
		}
	}
}
