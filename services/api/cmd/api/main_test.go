package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

func TestHealthAndMetaEndpoints(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router := newRouter(logger, nil, nil)

	t.Run("health live matches OpenAPI HealthStatus", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json, got %q", got)
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if body.Status != "ok" {
			t.Fatalf("expected status 'ok', got %q", body.Status)
		}
	})

	t.Run("health ready without db matches OpenAPI Problem", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
			t.Fatalf("expected application/problem+json, got %q", got)
		}
		var p httpx.Problem
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if p.Status != http.StatusServiceUnavailable || p.Title == "" || p.Type == "" {
			t.Fatalf("invalid Problem payload: %+v", p)
		}
	})

	t.Run("api v1 meta matches OpenAPI schema", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json, got %q", got)
		}
		var meta struct {
			Service string `json:"service"`
			Version string `json:"version"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if meta.Service != "semaian-api" || meta.Version != "v1" {
			t.Fatalf("unexpected meta payload: %+v", meta)
		}
	})
}
