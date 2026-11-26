package config_test

import (
	"testing"

	"github.com/semaian/platform/services/api/internal/platform/config"
)

func TestConfigLoadFailClosed(t *testing.T) {
	t.Run("missing APP_ENV fails", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error when APP_ENV is unset")
		}
	})

	t.Run("invalid APP_ENV fails", func(t *testing.T) {
		t.Setenv("APP_ENV", "prod")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error for invalid APP_ENV")
		}
	})

	t.Run("production without OIDC or DB fails", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("DATABASE_URL", "postgres://localhost/db")
		t.Setenv("OIDC_ISSUER", "")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error when OIDC config is missing in production")
		}
	})

	t.Run("production with non-https OIDC fails", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("DATABASE_URL", "postgres://localhost/db")
		t.Setenv("OIDC_ISSUER", "http://issuer.example")
		t.Setenv("OIDC_AUDIENCE", "semaian-api")
		t.Setenv("OIDC_JWKS_URL", "https://issuer.example/jwks.json")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error for non-https OIDC_ISSUER in production")
		}
	})

	t.Run("local allows minimal config", func(t *testing.T) {
		t.Setenv("APP_ENV", "local")
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("unexpected error in local env: %v", err)
		}
		if cfg.AppEnv != "local" {
			t.Fatalf("expected local, got %q", cfg.AppEnv)
		}
	})
}
