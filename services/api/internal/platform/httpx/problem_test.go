package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

func TestCorrelationIDSanitization(t *testing.T) {
	handler := middleware.RequestID(httpx.CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	t.Run("valid correlation id is preserved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Correlation-ID", "req-123_abc.456")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("X-Correlation-ID"); got != "req-123_abc.456" {
			t.Fatalf("expected preserved correlation id, got %q", got)
		}
	})

	t.Run("overlong or unsafe correlation id is replaced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Correlation-ID", strings.Repeat("a", 65))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		got := rec.Header().Get("X-Correlation-ID")
		if got == "" || len(got) > 64 || got == strings.Repeat("a", 65) {
			t.Fatalf("expected fallback request id, got %q", got)
		}
	})
}
