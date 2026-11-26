package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/semaian/platform/services/api/internal/platform/auth"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

type mockVerifier struct {
	tokens map[string]auth.Actor
}

func (m mockVerifier) Verify(_ context.Context, rawToken string) (auth.Actor, error) {
	actor, ok := m.tokens[rawToken]
	if !ok {
		return auth.Actor{}, errors.New("invalid token")
	}
	if !actor.Expires.IsZero() && time.Now().After(actor.Expires) {
		return auth.Actor{}, errors.New("token expired")
	}
	return actor, nil
}

func TestRequireAuth(t *testing.T) {
	verifier := mockVerifier{
		tokens: map[string]auth.Actor{
			"valid-token": {
				UserID:  "user-1",
				Subject: "sub-1",
				Issuer:  "https://issuer.example",
				Expires: time.Now().Add(5 * time.Minute),
			},
			"expired-token": {
				UserID:  "user-2",
				Subject: "sub-2",
				Issuer:  "https://issuer.example",
				Expires: time.Now().Add(-5 * time.Minute),
			},
		},
	}

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.ActorFromContext(r.Context())
		if !ok || actor.UserID != "user-1" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("nil verifier fails closed with 503", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
		rec := httptest.NewRecorder()
		auth.RequireAuth(nil)(okHandler).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
			t.Fatalf("expected application/problem+json, got %q", got)
		}
	})

	protected := auth.RequireAuth(verifier)(okHandler)
	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"malformed header", "Basic abc", http.StatusUnauthorized},
		{"empty bearer token", "Bearer   ", http.StatusUnauthorized},
		{"invalid token", "Bearer unknown", http.StatusUnauthorized},
		{"expired token", "Bearer expired-token", http.StatusUnauthorized},
		{"valid token", "Bearer valid-token", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}
			if tc.wantStatus == http.StatusUnauthorized {
				if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
					t.Fatalf("expected application/problem+json, got %q", got)
				}
				var p httpx.Problem
				if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
					t.Fatalf("failed to decode Problem JSON: %v", err)
				}
				if p.Status != http.StatusUnauthorized || p.Title == "" || p.Type == "" {
					t.Fatalf("invalid Problem payload: %+v", p)
				}
			}
		})
	}
}
