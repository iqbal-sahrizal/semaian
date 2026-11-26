package httpx

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type ctxKey string

const correlationIDKey ctxKey = "correlation_id"

var validCorrelationID = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

// Problem represents an RFC 9457 application/problem+json error response.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// WriteProblem writes an RFC 9457 problem response.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	p := Problem{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

// CorrelationID validates incoming X-Correlation-ID (^[a-zA-Z0-9._-]{1,64}$)
// and falls back to Chi's RequestID if missing or invalid.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid := r.Header.Get("X-Correlation-ID")
		if !validCorrelationID.MatchString(cid) {
			cid = middleware.GetReqID(r.Context())
		}
		if cid != "" {
			w.Header().Set("X-Correlation-ID", cid)
			r = r.WithContext(context.WithValue(r.Context(), correlationIDKey, cid))
		}
		next.ServeHTTP(w, r)
	})
}

// GetCorrelationID returns the correlation ID from context if present.
func GetCorrelationID(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDKey).(string); ok {
		return v
	}
	return ""
}

// RequestLogger logs structured HTTP requests without sensitive headers.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"correlation_id", GetCorrelationID(r.Context()),
			)
		})
	}
}
