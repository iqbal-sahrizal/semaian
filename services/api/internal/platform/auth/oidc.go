package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

type ctxKey string

const actorKey ctxKey = "auth_actor"

// Actor represents a verified authenticated identity.
type Actor struct {
	UserID   string
	Subject  string
	Issuer   string
	Audience []string
	Expires  time.Time
}

// TokenVerifier validates a bearer token and returns a verified Actor.
// Implementations must verify signature (JWKS), iss, aud, exp, nbf, and pin asymmetric alg.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (Actor, error)
}

// OrgMembershipChecker verifies organization membership and role server-side.
type OrgMembershipChecker interface {
	HasRole(ctx context.Context, userID, orgID string, requiredRoles ...string) (bool, error)
}

// WithActor attaches an Actor to context.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// ActorFromContext retrieves the authenticated Actor from context.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey).(Actor)
	return a, ok
}

// RequireAuth enforces valid bearer token authentication via TokenVerifier.
// Fails closed with 503 if verifier is nil.
func RequireAuth(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if verifier == nil {
				httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "authentication verifier is not configured")
				return
			}
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing or malformed bearer token")
				return
			}
			rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if rawToken == "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing or malformed bearer token")
				return
			}
			actor, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "invalid or expired token")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actor)))
		})
	}
}
