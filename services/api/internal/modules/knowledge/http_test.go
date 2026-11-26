package knowledge_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/platform/auth"
)

type stubAuthVerifier struct {
	actor auth.Actor
}

func (s stubAuthVerifier) Verify(_ context.Context, rawToken string) (auth.Actor, error) {
	if rawToken == "ok" {
		return s.actor, nil
	}
	return auth.Actor{}, knowledge.ErrForbidden
}

type stubRoleResolver struct {
	role knowledge.EditorialRole
}

func (s stubRoleResolver) ResolveEditorialRole(_ auth.Actor) knowledge.EditorialRole {
	return s.role
}

func TestSearchRateLimiterAndSpoofedForwardedHeader(t *testing.T) {
	h := knowledge.NewHandler(nil, nil, nil)
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		h.RegisterPublicRoutes(v1)
	})

	// 60 requests with short query (validation runs after rate limiter, so 400 counts toward bucket)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search?q=a", nil)
		req.RemoteAddr = "198.51.100.10:12345"
		// Spoofed X-Forwarded-For must be ignored
		req.Header.Set("X-Forwarded-For", "203.0.113."+string(rune('1'+i%9)))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("unexpected 429 on request %d", i+1)
		}
	}

	// 61st request from same RemoteAddr must be rate-limited (429 + Retry-After)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search?q=a", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.250")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 61st request, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected Retry-After: 60, got %q", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestEditorialRoutesAuthAndRBAC(t *testing.T) {
	edSvc := knowledge.NewEditorialService(nil, audit.NoopRecorder{})

	buildRouter := func(role knowledge.EditorialRole) http.Handler {
		h := knowledge.NewHandler(nil, edSvc, stubRoleResolver{role: role})
		r := chi.NewRouter()
		r.Route("/api/v1", func(v1 chi.Router) {
			v1.Use(auth.RequireAuth(stubAuthVerifier{actor: auth.Actor{Subject: "actor-1", Expires: time.Now().Add(time.Minute)}}))
			h.RegisterEditorialRoutes(v1)
		})
		return r
	}

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeEditor)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/articles", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("reader cannot create article (403)", func(t *testing.T) {
		r := buildRouter(knowledge.RoleReader)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/articles", strings.NewReader(`{"slug":"a-1","title":"T","body":"B"}`))
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("editor cannot review or publish (403)", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeEditor)
		for _, path := range []string{
			"/api/v1/editorial/knowledge/articles/a1/revisions/r1/review",
			"/api/v1/editorial/knowledge/articles/a1/revisions/r1/publish",
		} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"decision":"approved"}`))
			req.Header.Set("Authorization", "Bearer ok")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 on %s, got %d", path, rec.Code)
			}
		}
	})

	t.Run("reviewer cannot emergency-unpublish (403)", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeReviewer)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/articles/a1/emergency-unpublish", nil)
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})
}

type stubTransRepo struct {
	knowledge.Repository
	dup bool
}

func (s stubTransRepo) InsertTranslation(_ context.Context, t knowledge.Translation) (knowledge.Translation, error) {
	if err := knowledge.ValidateTranslation(t); err != nil {
		return knowledge.Translation{}, err
	}
	if s.dup {
		return knowledge.Translation{}, knowledge.ErrConflict
	}
	t.ID = "tr-1"
	return t, nil
}

func TestInsertTranslationHTTPRoutes(t *testing.T) {
	buildRouter := func(role knowledge.EditorialRole, dup bool) http.Handler {
		edSvc := knowledge.NewEditorialService(nil, audit.NoopRecorder{}).WithStructuredRepo(stubTransRepo{dup: dup})
		h := knowledge.NewHandler(nil, edSvc, stubRoleResolver{role: role})
		r := chi.NewRouter()
		r.Route("/api/v1", func(v1 chi.Router) {
			v1.Use(auth.RequireAuth(stubAuthVerifier{actor: auth.Actor{Subject: "actor-1", Expires: time.Now().Add(time.Minute)}}))
			h.RegisterEditorialRoutes(v1)
		})
		return r
	}

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeEditor, false)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/translations", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("reader returns 403", func(t *testing.T) {
		r := buildRouter(knowledge.RoleReader, false)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/translations", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("article entity_type is blocked (400)", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeEditor, false)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/translations", strings.NewReader(`{"entity_type":"article","entity_id":"e1","locale":"id","title":"T"}`))
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for article entity_type, got %d", rec.Code)
		}
	})

	t.Run("duplicate translation returns 409", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeEditor, true)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/translations", strings.NewReader(`{"entity_type":"region","entity_id":"e1","locale":"id","title":"Gayo"}`))
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("valid structured translation returns 201", func(t *testing.T) {
		r := buildRouter(knowledge.RoleKnowledgeEditor, false)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/editorial/knowledge/translations", strings.NewReader(`{"entity_type":"region","entity_id":"e1","locale":"id","title":"Gayo"}`))
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})
}

func TestStructuredEntityEditorialHTTPRoutes(t *testing.T) {
	edSvc := knowledge.NewEditorialService(nil, audit.NoopRecorder{})
	buildRouter := func(role knowledge.EditorialRole) http.Handler {
		h := knowledge.NewHandler(nil, edSvc, stubRoleResolver{role: role})
		r := chi.NewRouter()
		r.Route("/api/v1", func(v1 chi.Router) {
			v1.Use(auth.RequireAuth(stubAuthVerifier{actor: auth.Actor{Subject: "actor-1", Expires: time.Now().Add(time.Minute)}}))
			h.RegisterEditorialRoutes(v1)
		})
		return r
	}

	endpoints := []string{
		"/api/v1/editorial/knowledge/countries",
		"/api/v1/editorial/knowledge/regions",
		"/api/v1/editorial/knowledge/producers",
		"/api/v1/editorial/knowledge/species",
		"/api/v1/editorial/knowledge/varieties",
		"/api/v1/editorial/knowledge/processing-methods",
		"/api/v1/editorial/knowledge/roast-levels",
		"/api/v1/editorial/knowledge/flavors",
	}

	for _, ep := range endpoints {
		t.Run("401 unauthenticated "+ep, func(t *testing.T) {
			r := buildRouter(knowledge.RoleKnowledgeEditor)
			req := httptest.NewRequest(http.MethodPost, ep, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 on %s, got %d", ep, rec.Code)
			}
		})
		t.Run("403 reader "+ep, func(t *testing.T) {
			r := buildRouter(knowledge.RoleReader)
			req := httptest.NewRequest(http.MethodPost, ep, strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer ok")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 on %s, got %d", ep, rec.Code)
			}
		})
		t.Run("400 invalid JSON "+ep, func(t *testing.T) {
			r := buildRouter(knowledge.RoleKnowledgeEditor)
			req := httptest.NewRequest(http.MethodPost, ep, strings.NewReader(`{bad`))
			req.Header.Set("Authorization", "Bearer ok")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 on %s, got %d", ep, rec.Code)
			}
		})
	}
}
