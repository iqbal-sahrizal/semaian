package organization_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/organization"
)

type memOrgRepo struct {
	mu      sync.Mutex
	orgs    map[string]organization.Organization
	members map[string]map[string]organization.Membership
}

func newMemOrgRepo() *memOrgRepo {
	return &memOrgRepo{
		orgs:    make(map[string]organization.Organization),
		members: make(map[string]map[string]organization.Membership),
	}
}

func (m *memOrgRepo) ListByUser(_ context.Context, userID string) ([]organization.Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []organization.Organization
	for orgID, org := range m.orgs {
		if org.Status != organization.OrgStatusActive {
			continue
		}
		if mem, ok := m.members[orgID][userID]; ok && mem.Status == organization.MembershipActive {
			out = append(out, org)
		}
	}
	return out, nil
}

func (m *memOrgRepo) CreateWithOwner(_ context.Context, name, slug, ownerUserID string) (organization.Organization, organization.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.orgs {
		if o.Slug == slug {
			return organization.Organization{}, organization.Membership{}, organization.ErrConflict
		}
	}
	org := organization.Organization{ID: "org-" + slug, Name: name, Slug: slug, Status: organization.OrgStatusActive}
	mem := organization.Membership{ID: "mem-" + ownerUserID, OrganizationID: org.ID, UserID: ownerUserID, Role: organization.RoleOwner, Status: organization.MembershipActive}
	m.orgs[org.ID] = org
	m.members[org.ID] = map[string]organization.Membership{ownerUserID: mem}
	return org, mem, nil
}

func (m *memOrgRepo) GetByID(_ context.Context, orgID string) (organization.Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[orgID]
	if !ok {
		return organization.Organization{}, organization.ErrNotFound
	}
	return o, nil
}

func (m *memOrgRepo) Update(_ context.Context, orgID, name string) (organization.Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o := m.orgs[orgID]
	o.Name = name
	m.orgs[orgID] = o
	return o, nil
}

func (m *memOrgRepo) GetMembership(_ context.Context, orgID, userID string) (organization.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.members[orgID][userID]
	if !ok {
		return organization.Membership{}, organization.ErrNotFound
	}
	return mem, nil
}

func (m *memOrgRepo) ListMembers(_ context.Context, orgID string) ([]organization.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []organization.Membership
	for _, mem := range m.members[orgID] {
		if mem.Status == organization.MembershipActive {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (m *memOrgRepo) ChangeMemberRole(_ context.Context, orgID, targetUserID string, newRole organization.Role) (organization.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.members[orgID][targetUserID]
	if !ok {
		return organization.Membership{}, organization.ErrNotFound
	}
	if mem.Role == organization.RoleOwner && newRole != organization.RoleOwner {
		return organization.Membership{}, organization.ErrLastOwner
	}
	mem.Role = newRole
	m.members[orgID][targetUserID] = mem
	return mem, nil
}

func (m *memOrgRepo) RemoveMember(_ context.Context, orgID, targetUserID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.members[orgID][targetUserID]
	if !ok {
		return organization.ErrNotFound
	}
	if mem.Role == organization.RoleOwner {
		return organization.ErrLastOwner
	}
	mem.Status = organization.MembershipInactive
	m.members[orgID][targetUserID] = mem
	return nil
}

func (m *memOrgRepo) TransferOwnership(_ context.Context, orgID, currentOwnerID, newOwnerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.members[orgID][currentOwnerID]
	nxt, ok := m.members[orgID][newOwnerID]
	if !ok || nxt.Status != organization.MembershipActive {
		return organization.ErrNotFound
	}
	if cur.Role != organization.RoleOwner {
		return organization.ErrForbidden
	}
	cur.Role = organization.RoleAdmin
	nxt.Role = organization.RoleOwner
	m.members[orgID][currentOwnerID] = cur
	m.members[orgID][newOwnerID] = nxt
	return nil
}

func TestOrganizationServiceLifecycleAndIsolation(t *testing.T) {
	repo := newMemOrgRepo()
	svc := organization.NewService(repo, audit.NoopRecorder{})
	ctx := context.Background()

	org, err := svc.Create(ctx, "owner-1", "Kopi Nusantara", "kopi-nusantara", "cid-1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Add member-1 and admin-1
	repo.members[org.ID]["member-1"] = organization.Membership{ID: "m-2", OrganizationID: org.ID, UserID: "member-1", Role: organization.RoleMember, Status: organization.MembershipActive}
	repo.members[org.ID]["admin-1"] = organization.Membership{ID: "m-3", OrganizationID: org.ID, UserID: "admin-1", Role: organization.RoleAdmin, Status: organization.MembershipActive}

	// Cross-org / non-member rejected
	if _, err := svc.Get(ctx, "outsider-1", org.ID); !errors.Is(err, organization.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for outsider, got %v", err)
	}

	// Member cannot update org
	if _, err := svc.Update(ctx, "member-1", org.ID, "Hacked", "cid-2"); !errors.Is(err, organization.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for member update, got %v", err)
	}

	// Admin cannot transfer ownership
	if err := svc.TransferOwnership(ctx, "admin-1", org.ID, "member-1", "cid-3"); !errors.Is(err, organization.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin transfer, got %v", err)
	}

	// Last owner cannot be removed
	if err := svc.RemoveMember(ctx, "owner-1", org.ID, "owner-1", "cid-4"); !errors.Is(err, organization.ErrLastOwner) {
		t.Fatalf("expected ErrLastOwner, got %v", err)
	}

	// Owner transfers ownership to admin-1
	if err := svc.TransferOwnership(ctx, "owner-1", org.ID, "admin-1", "cid-5"); err != nil {
		t.Fatalf("TransferOwnership failed: %v", err)
	}

	// Suspended org blocks access
	o := repo.orgs[org.ID]
	o.Status = organization.OrgStatusSuspended
	repo.orgs[org.ID] = o
	if _, err := svc.Get(ctx, "admin-1", org.ID); !errors.Is(err, organization.ErrOrgInactive) {
		t.Fatalf("expected ErrOrgInactive, got %v", err)
	}
}
