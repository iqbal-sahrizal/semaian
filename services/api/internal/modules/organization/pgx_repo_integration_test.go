//go:build integration

package organization_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/identity"
	"github.com/semaian/platform/services/api/internal/modules/organization"
	"github.com/semaian/platform/services/api/internal/platform/db"
)

func TestIdentityAndOrganizationPGXIntegration(t *testing.T) {
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
	idSvc := identity.NewService(identity.NewPGXRepository(pool), auditRec)
	orgRepo := organization.NewPGXRepository(pool)
	orgSvc := organization.NewService(orgRepo, auditRec)

	u1, err := idSvc.ResolveExternalIdentity(ctx, "https://issuer.test", "sub-"+uuid.NewString())
	if err != nil {
		t.Fatalf("ResolveExternalIdentity u1 failed: %v", err)
	}
	u2, err := idSvc.ResolveExternalIdentity(ctx, "https://issuer.test", "sub-"+uuid.NewString())
	if err != nil {
		t.Fatalf("ResolveExternalIdentity u2 failed: %v", err)
	}

	slug := "roastery-" + uuid.NewString()[:8]
	org, err := orgSvc.Create(ctx, u1.ID, "Semaian Roastery", slug, "cid-int-1")
	if err != nil {
		t.Fatalf("Create org failed: %v", err)
	}

	// Duplicate slug must fail with ErrConflict
	if _, err := orgSvc.Create(ctx, u1.ID, "Duplicate", slug, "cid-int-2"); !errors.Is(err, organization.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate slug, got %v", err)
	}

	// Last owner cannot be removed
	if err := orgSvc.RemoveMember(ctx, u1.ID, org.ID, u1.ID, "cid-int-3"); !errors.Is(err, organization.ErrLastOwner) {
		t.Fatalf("expected ErrLastOwner, got %v", err)
	}

	// Add u2 as member, promote to admin, transfer ownership to u2, then remove u1
	_, err = pool.Exec(ctx,
		`INSERT INTO organization.memberships (id, organization_id, user_id, role, status) VALUES ($1, $2, $3, 'member', 'active')`,
		uuid.NewString(), org.ID, u2.ID,
	)
	if err != nil {
		t.Fatalf("insert member u2 failed: %v", err)
	}
	if _, err := orgSvc.ChangeMemberRole(ctx, u1.ID, org.ID, u2.ID, organization.RoleAdmin, "cid-int-4"); err != nil {
		t.Fatalf("ChangeMemberRole failed: %v", err)
	}
	if err := orgSvc.TransferOwnership(ctx, u1.ID, org.ID, u2.ID, "cid-int-5"); err != nil {
		t.Fatalf("TransferOwnership failed: %v", err)
	}
	if err := orgSvc.RemoveMember(ctx, u2.ID, org.ID, u1.ID, "cid-int-6"); err != nil {
		t.Fatalf("RemoveMember u1 after transfer failed: %v", err)
	}

	// Verify audit events were recorded
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE organization_id = $1`, org.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query audit.events failed: %v", err)
	}
	if auditCount < 4 {
		t.Fatalf("expected at least 4 audit events for org, got %d", auditCount)
	}
}
