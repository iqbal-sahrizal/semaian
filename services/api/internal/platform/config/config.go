package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds validated application configuration.
type Config struct {
	AppEnv             string
	HTTPAddr           string
	DatabaseURL        string
	OIDCIssuer         string
	OIDCAudience       string
	OIDCJWKSURL        string
	CORSAllowedOrigins []string
}

// Load reads configuration from environment variables and validates invariants.
// APP_ENV must be explicitly set to one of: local, test, staging, production.
func Load() (Config, error) {
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	switch appEnv {
	case "local", "test", "staging", "production":
	case "":
		return Config{}, errors.New("APP_ENV must be explicitly set (local, test, staging, or production)")
	default:
		return Config{}, fmt.Errorf("invalid APP_ENV %q: must be local, test, staging, or production", appEnv)
	}

	cfg := Config{
		AppEnv:       appEnv,
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		OIDCIssuer:   os.Getenv("OIDC_ISSUER"),
		OIDCAudience: os.Getenv("OIDC_AUDIENCE"),
		OIDCJWKSURL:  os.Getenv("OIDC_JWKS_URL"),
	}

	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, trimmed)
			}
		}
	}

	if cfg.AppEnv == "staging" || cfg.AppEnv == "production" {
		if cfg.DatabaseURL == "" {
			return Config{}, errors.New("DATABASE_URL is required in staging and production")
		}
		if cfg.OIDCIssuer == "" || cfg.OIDCAudience == "" || cfg.OIDCJWKSURL == "" {
			return Config{}, errors.New("OIDC_ISSUER, OIDC_AUDIENCE, and OIDC_JWKS_URL are required in staging and production")
		}
		if !strings.HasPrefix(cfg.OIDCIssuer, "https://") || !strings.HasPrefix(cfg.OIDCJWKSURL, "https://") {
			return Config{}, errors.New("OIDC_ISSUER and OIDC_JWKS_URL must use https:// in staging and production")
		}
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
