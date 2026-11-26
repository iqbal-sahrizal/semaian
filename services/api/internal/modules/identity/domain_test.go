package identity_test

import (
	"context"
	"errors"
	"testing"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/identity"
)

type memRepo struct {
	users map[string]identity.User
	ext   map[string]string
}

func newMemRepo() *memRepo {
	return &memRepo{users: make(map[string]identity.User), ext: make(map[string]string)}
}

func (m *memRepo) ResolveOrCreateByExternal(_ context.Context, issuer, subject string) (identity.User, error) {
	key := issuer + "|" + subject
	if uid, ok := m.ext[key]; ok {
		return m.users[uid], nil
	}
	u := identity.User{ID: "u-" + subject, Status: identity.StatusActive}
	m.users[u.ID] = u
	m.ext[key] = u.ID
	return u, nil
}

func (m *memRepo) GetUser(_ context.Context, userID string) (identity.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return identity.User{}, identity.ErrNotFound
	}
	return u, nil
}

func (m *memRepo) UpdateStatus(_ context.Context, userID string, status identity.Status) (identity.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return identity.User{}, identity.ErrNotFound
	}
	u.Status = status
	m.users[userID] = u
	return u, nil
}

func (m *memRepo) GetProfile(_ context.Context, userID string) (identity.Profile, error) {
	return identity.Profile{UserID: userID, DisplayName: "Test"}, nil
}

func (m *memRepo) UpdateProfile(_ context.Context, userID string, upd identity.ProfileUpdate) (identity.Profile, error) {
	p := identity.Profile{UserID: userID}
	if upd.DisplayName != nil {
		p.DisplayName = *upd.DisplayName
	}
	return p, nil
}

func TestIdentityLifecycle(t *testing.T) {
	svc := identity.NewService(newMemRepo(), audit.NoopRecorder{})
	ctx := context.Background()

	u, err := svc.ResolveExternalIdentity(ctx, "https://issuer.example", "sub-1")
	if err != nil || u.Status != identity.StatusActive {
		t.Fatalf("expected active user, got err=%v u=%+v", err, u)
	}

	if _, err := svc.SetStatus(ctx, "admin-1", u.ID, identity.StatusSuspended, "cid-1"); err != nil {
		t.Fatalf("suspend failed: %v", err)
	}

	if _, err := svc.ResolveExternalIdentity(ctx, "https://issuer.example", "sub-1"); !errors.Is(err, identity.ErrInactiveIdentity) {
		t.Fatalf("expected ErrInactiveIdentity for suspended user, got %v", err)
	}

	if _, err := svc.SetStatus(ctx, "admin-1", u.ID, identity.StatusDeactivated, "cid-2"); err != nil {
		t.Fatalf("deactivate failed: %v", err)
	}

	if _, err := svc.SetStatus(ctx, "admin-1", u.ID, identity.StatusActive, "cid-3"); !errors.Is(err, identity.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition from deactivated to active, got %v", err)
	}
}

func strPtr(s string) *string { return &s }

func TestProfileValidationAndNormalization(t *testing.T) {
	svc := identity.NewService(newMemRepo(), audit.NoopRecorder{})
	ctx := context.Background()
	u, _ := svc.ResolveExternalIdentity(ctx, "https://issuer.example", "sub-prof")

	p, err := svc.UpdateProfile(ctx, u.ID, identity.ProfileUpdate{
		DisplayName:     strPtr("  Budi  "),
		AvatarRef:       strPtr("  https://cdn.example.com/a.png  "),
		PreferredLocale: strPtr("  id-ID  "),
		Timezone:        strPtr("  Asia/Jakarta  "),
	})
	if err != nil {
		t.Fatalf("expected valid profile update, got %v", err)
	}
	if p.DisplayName != "Budi" {
		t.Fatalf("expected trimmed DisplayName 'Budi', got %q", p.DisplayName)
	}

	invalidCases := []struct {
		name string
		upd  identity.ProfileUpdate
	}{
		{"http avatar", identity.ProfileUpdate{AvatarRef: strPtr("http://insecure.example/a.png")}},
		{"javascript avatar", identity.ProfileUpdate{AvatarRef: strPtr("javascript:alert(1)")}},
		{"path traversal avatar", identity.ProfileUpdate{AvatarRef: strPtr("../secret")}},
		{"invalid locale", identity.ProfileUpdate{PreferredLocale: strPtr("english_us_123456789")}},
		{"invalid timezone", identity.ProfileUpdate{Timezone: strPtr("Mars/Olympus")}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.UpdateProfile(ctx, u.ID, tc.upd); !errors.Is(err, identity.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput for %s, got %v", tc.name, err)
			}
		})
	}
}
