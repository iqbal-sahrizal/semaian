package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/identity"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/modules/organization"
	"github.com/semaian/platform/services/api/internal/platform/auth"
	"github.com/semaian/platform/services/api/internal/platform/config"
	"github.com/semaian/platform/services/api/internal/platform/db"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type identityUserResolver struct {
	svc *identity.Service
}

func (r identityUserResolver) ResolveUserID(ctx context.Context, issuer, subject string) (string, error) {
	u, err := r.svc.ResolveExternalIdentity(ctx, issuer, subject)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

type envEditorialRoleResolver struct {
	superAdmins map[string]bool
	reviewers   map[string]bool
	editors     map[string]bool
}

func newEnvEditorialRoleResolver() (envEditorialRoleResolver, error) {
	parseSet := func(key string) map[string]bool {
		m := make(map[string]bool)
		for _, s := range strings.Split(os.Getenv(key), ",") {
			if t := strings.TrimSpace(s); t != "" {
				m[t] = true
			}
		}
		return m
	}
	res := envEditorialRoleResolver{
		superAdmins: parseSet("EDITORIAL_SUPER_ADMINS"),
		reviewers:   parseSet("EDITORIAL_REVIEWERS"),
		editors:     parseSet("EDITORIAL_EDITORS"),
	}
	for sub := range res.superAdmins {
		if res.reviewers[sub] || res.editors[sub] {
			return envEditorialRoleResolver{}, fmt.Errorf("conflicting editorial role assignment for subject %q", sub)
		}
	}
	for sub := range res.reviewers {
		if res.editors[sub] {
			return envEditorialRoleResolver{}, fmt.Errorf("conflicting editorial role assignment for subject %q", sub)
		}
	}
	return res, nil
}

func (r envEditorialRoleResolver) ResolveEditorialRole(actor auth.Actor) knowledge.EditorialRole {
	if strings.TrimSpace(actor.Subject) == "" {
		return knowledge.RoleReader
	}
	count := 0
	role := knowledge.RoleReader
	if r.superAdmins[actor.Subject] {
		count++
		role = knowledge.RoleSuperAdmin
	}
	if r.reviewers[actor.Subject] {
		count++
		role = knowledge.RoleKnowledgeReviewer
	}
	if r.editors[actor.Subject] {
		count++
		role = knowledge.RoleKnowledgeEditor
	}
	if count != 1 {
		return knowledge.RoleReader
	}
	return role
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		pool, err = db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to initialize database pool", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
	}

	var verifier auth.TokenVerifier
	if cfg.OIDCIssuer != "" && cfg.OIDCAudience != "" && cfg.OIDCJWKSURL != "" {
		verifier, err = auth.NewJWKSVerifier(auth.JWKSVerifierConfig{
			Issuer:        cfg.OIDCIssuer,
			Audience:      cfg.OIDCAudience,
			JWKSURL:       cfg.OIDCJWKSURL,
			AllowInsecure: cfg.AppEnv == "local" || cfg.AppEnv == "test",
		})
		if err != nil {
			logger.Error("failed to initialize OIDC verifier", "error", err)
			os.Exit(1)
		}
	}

	router := newRouter(logger, pool, verifier)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("http server starting", "address", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("http server stopped")
}

func newRouter(logger *slog.Logger, pool *pgxpool.Pool, verifier auth.TokenVerifier) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.CorrelationID)
	r.Use(middleware.Recoverer)
	r.Use(httpx.RequestLogger(logger))

	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ready(r.Context(), pool); err != nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "database is not reachable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Get("/meta", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"service":"semaian-api","version":"v1"}`))
		})

		if pool != nil {
			txRunner := tx.NewPGXRunner(pool)
			auditRec := audit.NewPGXRecorder(pool)
			idSvc := identity.NewService(identity.NewPGXRepository(pool), auditRec).WithTxRunner(txRunner)
			orgSvc := organization.NewService(organization.NewPGXRepository(pool), auditRec).WithTxRunner(txRunner)
			knowRoles, err := newEnvEditorialRoleResolver()
			if err != nil {
				logger.Error("invalid editorial role configuration", "error", err)
				os.Exit(1)
			}
			knowQuery := knowledge.NewQueryRepository(pool)
			knowStruct := knowledge.NewPGXRepository(pool)
			knowEd := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool).WithAudit(auditRec), auditRec).WithStructuredRepo(knowStruct).WithTxRunner(txRunner)
			knowHandler := knowledge.NewHandler(knowQuery, knowEd, knowRoles).WithRepository(knowStruct)

			knowHandler.RegisterPublicRoutes(v1)

			v1.Group(func(protected chi.Router) {
				protected.Use(auth.RequireAuth(verifier))
				identity.NewHandler(idSvc).RegisterRoutes(protected)
				organization.NewHandler(orgSvc, identityUserResolver{svc: idSvc}).RegisterRoutes(protected)
				knowHandler.RegisterEditorialRoutes(protected)
			})
		}
	})

	return r
}
