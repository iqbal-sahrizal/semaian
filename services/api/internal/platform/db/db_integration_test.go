//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/semaian/platform/services/api/internal/platform/db"
)

func TestDatabaseReadyIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	if err := db.Ready(ctx, pool); err != nil {
		t.Fatalf("Ready failed: %v", err)
	}
}
