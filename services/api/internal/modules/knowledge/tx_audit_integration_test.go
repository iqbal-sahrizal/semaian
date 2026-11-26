//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/platform/db"
)

type failingAuditRecorder struct{}

func (failingAuditRecorder) Record(_ context.Context, _ audit.Event) error {
	return errors.New("injected audit failure")
}

func TestTransactionalAuditAtomicity(t *testing.T) {
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

	// 1. Audit failure inside PGX transaction rolls back business mutation
	slugFail := "tx-fail-" + uuid.NewString()[:6]
	repoFail := knowledge.NewEditorialPGXRepository(pool).WithAudit(failingAuditRecorder{})
	if _, _, err := repoFail.CreateArticleWithRevision(ctx, slugFail, "Fail Title", "Body", uuid.NewString()); err == nil {
		t.Fatal("expected CreateArticleWithRevision to fail when audit recorder fails")
	}
	var artCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM knowledge.articles WHERE slug = $1`, slugFail).Scan(&artCount)
	if artCount != 0 {
		t.Fatalf("expected 0 articles after audit failure rollback, got %d", artCount)
	}

	// 2. Successful mutation + PGXRecorder commits both atomically
	slugOK := "tx-ok-" + uuid.NewString()[:6]
	repoOK := knowledge.NewEditorialPGXRepository(pool).WithAudit(audit.NewPGXRecorder(pool))
	art, _, err := repoOK.CreateArticleWithRevision(ctx, slugOK, "OK Title", "Body", uuid.NewString())
	if err != nil {
		t.Fatalf("CreateArticleWithRevision failed: %v", err)
	}
	var auditCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE target_id = $1 AND action = 'knowledge.article.created'`, art.ID).Scan(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected exactly 1 audit event, got %d", auditCount)
	}

	// 3. Business mutation failure (duplicate slug) leaves 0 audit events
	if _, _, err := repoOK.CreateArticleWithRevision(ctx, slugOK, "Dup Title", "Body", uuid.NewString()); err == nil {
		t.Fatal("expected duplicate slug to fail")
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE target_id = $1 AND action = 'knowledge.article.created'`, art.ID).Scan(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected still 1 audit event after failed duplicate, got %d", auditCount)
	}

	// 4. Standalone PGXRecorder.Record (without WithTx) remains backward-compatible
	standaloneID := uuid.NewString()
	if err := audit.NewPGXRecorder(pool).Record(ctx, audit.Event{
		Action: "standalone.test", TargetType: "test", TargetID: standaloneID,
	}); err != nil {
		t.Fatalf("standalone Record failed: %v", err)
	}
}
