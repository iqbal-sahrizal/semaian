package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/identity"
	"github.com/semaian/platform/services/api/internal/platform/auth"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

type stubVerifier struct{}

func (stubVerifier) Verify(_ context.Context, rawToken string) (auth.Actor, error) {
	if rawToken == "valid" {
		return auth.Actor{Subject: "sub-1", Issuer: "https://issuer.test", Expires: time.Now().Add(time.Minute)}, nil
	}
	return auth.Actor{}, context.DeadlineExceeded
}

type stubIDRepo struct{}

func (stubIDRepo) ResolveOrCreateByExternal(_ context.Context, _, _ string) (identity.User, error) {
	return identity.User{ID: "u-1", Status: identity.StatusActive}, nil
}
func (stubIDRepo) GetUser(_ context.Context, _ string) (identity.User, error) {
	return identity.User{ID: "u-1", Status: identity.StatusActive}, nil
}
func (stubIDRepo) UpdateStatus(_ context.Context, _ string, s identity.Status) (identity.User, error) {
	return identity.User{ID: "u-1", Status: s}, nil
}
func (stubIDRepo) GetProfile(_ context.Context, _ string) (identity.Profile, error) {
	return identity.Profile{UserID: "u-1", DisplayName: "Budi", PreferredLocale: "id", Timezone: "Asia/Jakarta"}, nil
}
func (stubIDRepo) UpdateProfile(_ context.Context, _ string, _ identity.ProfileUpdate) (identity.Profile, error) {
	return identity.Profile{UserID: "u-1", DisplayName: "Budi", PreferredLocale: "id", Timezone: "Asia/Jakarta"}, nil
}

func TestProtectedHTTPEndpointsAndProblemJSON(t *testing.T) {
	r := chi.NewRouter()
	idSvc := identity.NewService(stubIDRepo{}, audit.NoopRecorder{})
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(auth.RequireAuth(stubVerifier{}))
		identity.NewHandler(idSvc).RegisterRoutes(v1)
	})

	t.Run("unauthenticated returns 401 RFC 9457", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("expected 401 problem+json, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
		}
		var p httpx.Problem
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil || p.Status != 401 {
			t.Fatalf("invalid Problem body: %+v err=%v", p, err)
		}
	})

	t.Run("authenticated GET /api/v1/me returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer valid")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("invalid JSON PATCH /api/v1/me/profile returns 400 RFC 9457", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/profile", strings.NewReader("{bad"))
		req.Header.Set("Authorization", "Bearer valid")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("expected 400 problem+json, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
		}
	})
}
