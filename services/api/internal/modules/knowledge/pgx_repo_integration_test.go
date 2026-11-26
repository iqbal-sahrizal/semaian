//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/platform/db"
)

func TestKnowledgePGXIntegration(t *testing.T) {
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

	repo := knowledge.NewPGXRepository(pool)

	// 1. Country + Region + Producer
	_, _ = pool.Exec(ctx, `DELETE FROM knowledge.producers WHERE region_id IN (SELECT id FROM knowledge.regions WHERE country_id IN (SELECT id FROM knowledge.countries WHERE iso_code = 'ZZ'))`)
	_, _ = pool.Exec(ctx, `DELETE FROM knowledge.regions WHERE country_id IN (SELECT id FROM knowledge.countries WHERE iso_code = 'ZZ')`)
	_, _ = pool.Exec(ctx, `DELETE FROM knowledge.countries WHERE iso_code = 'ZZ'`)
	c, err := repo.CreateCountry(ctx, "ZZ", "test-country-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateCountry failed: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge.producers WHERE region_id IN (SELECT id FROM knowledge.regions WHERE country_id = $1)`, c.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge.regions WHERE country_id = $1`, c.ID)
		_ = repo.DeleteCountry(ctx, c.ID)
	}()

	regA, err := repo.CreateRegion(ctx, c.ID, nil, "gayo-a-"+uuid.NewString()[:6], nil, nil)
	if err != nil {
		t.Fatalf("CreateRegion A failed: %v", err)
	}
	regB, err := repo.CreateRegion(ctx, c.ID, &regA.ID, "gayo-b-"+uuid.NewString()[:6], nil, nil)
	if err != nil {
		t.Fatalf("CreateRegion B failed: %v", err)
	}
	regC, err := repo.CreateRegion(ctx, c.ID, &regB.ID, "gayo-c-"+uuid.NewString()[:6], nil, nil)
	if err != nil {
		t.Fatalf("CreateRegion C failed: %v", err)
	}

	// Producer tests: valid, invalid region_id, invalid producer_type, duplicate slug
	prodSlug := "farm-" + uuid.NewString()[:6]
	if _, err := repo.CreateProducer(ctx, regA.ID, "farm", prodSlug); err != nil {
		t.Fatalf("CreateProducer valid failed: %v", err)
	}
	if _, err := repo.CreateProducer(ctx, regA.ID, "farm", prodSlug); !errors.Is(err, knowledge.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate producer slug, got %v", err)
	}
	if _, err := repo.CreateProducer(ctx, uuid.NewString(), "farm", "bad-reg-"+uuid.NewString()[:6]); !errors.Is(err, knowledge.ErrForeignKey) {
		t.Fatalf("expected ErrForeignKey for invalid region_id, got %v", err)
	}
	if _, err := repo.CreateProducer(ctx, regA.ID, "invalid_type", "bad-type-"+uuid.NewString()[:6]); !errors.Is(err, knowledge.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid producer_type, got %v", err)
	}

	// 2. Hierarchy cycle prevention: self-cycle (A->A) and multi-hop cycle (A->B->C, C->A)
	if _, err := pool.Exec(ctx, `UPDATE knowledge.regions SET parent_id = $1 WHERE id = $1`, regA.ID); err == nil {
		t.Fatal("expected self-referencing region cycle A->A to fail")
	}
	if _, err := pool.Exec(ctx, `UPDATE knowledge.regions SET parent_id = $1 WHERE id = $2`, regC.ID, regA.ID); err == nil {
		t.Fatal("expected multi-hop region cycle A->B->C->A to fail")
	}

	// 3. Species, Variety, Process, Flavor & Relationships
	sp, err := repo.CreateSpecies(ctx, "Coffea test-"+uuid.NewString()[:6], "arabica-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateSpecies failed: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DELETE FROM knowledge.species WHERE id = $1`, sp.ID) }()

	vr, err := repo.CreateVariety(ctx, sp.ID, "timtim-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateVariety failed: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DELETE FROM knowledge.varieties WHERE id = $1`, vr.ID) }()

	pm1, err := repo.CreateProcessingMethod(ctx, "wet-hulled-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateProcessingMethod 1 failed: %v", err)
	}
	pm2, err := repo.CreateProcessingMethod(ctx, "natural-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateProcessingMethod 2 failed: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge.processing_methods WHERE id IN ($1, $2)`, pm1.ID, pm2.ID)
	}()

	fd, err := repo.CreateFlavorDescriptor(ctx, nil, "citrus-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateFlavorDescriptor failed: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DELETE FROM knowledge.flavor_descriptors WHERE id = $1`, fd.ID) }()

	if err := repo.LinkRegionVariety(ctx, regA.ID, vr.ID, knowledge.ConfidenceHigh); err != nil {
		t.Fatalf("LinkRegionVariety failed: %v", err)
	}
	if err := repo.LinkRegionVariety(ctx, regA.ID, vr.ID, knowledge.ConfidenceHigh); !errors.Is(err, knowledge.ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate region_variety, got %v", err)
	}
	if err := repo.LinkVarietyFlavor(ctx, vr.ID, fd.ID, knowledge.ConfidenceMedium); err != nil {
		t.Fatalf("LinkVarietyFlavor failed: %v", err)
	}
	if err := repo.LinkProcessFlavor(ctx, pm1.ID, fd.ID, knowledge.ConfidenceMedium); err != nil {
		t.Fatalf("LinkProcessFlavor failed: %v", err)
	}

	// 4. Polymorphic translations: valid, duplicate, orphan rejection, and cascade delete isolation
	if _, err := repo.InsertTranslation(ctx, knowledge.Translation{
		EntityType: knowledge.EntityProcessingMethod,
		EntityID:   pm1.ID,
		Locale:     knowledge.LocaleID,
		Title:      "Giling Basah",
	}); err != nil {
		t.Fatalf("InsertTranslation pm1 failed: %v", err)
	}
	if _, err := repo.InsertTranslation(ctx, knowledge.Translation{
		EntityType: knowledge.EntityProcessingMethod,
		EntityID:   pm2.ID,
		Locale:     knowledge.LocaleID,
		Title:      "Natural",
	}); err != nil {
		t.Fatalf("InsertTranslation pm2 failed: %v", err)
	}
	if _, err := repo.InsertTranslation(ctx, knowledge.Translation{
		EntityType: knowledge.EntityProcessingMethod,
		EntityID:   pm1.ID,
		Locale:     knowledge.LocaleID,
		Title:      "Duplicate",
	}); !errors.Is(err, knowledge.ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate translation, got %v", err)
	}
	if _, err := repo.InsertTranslation(ctx, knowledge.Translation{
		EntityType: knowledge.EntityRegion,
		EntityID:   uuid.NewString(),
		Locale:     knowledge.LocaleEN,
		Title:      "Orphan",
	}); !errors.Is(err, knowledge.ErrForeignKey) {
		t.Fatalf("expected ErrForeignKey on orphan translation, got %v", err)
	}

	// Delete pm1 and verify pm1's translation is deleted while pm2's translation remains
	if _, err := pool.Exec(ctx, `DELETE FROM knowledge.processing_methods WHERE id = $1`, pm1.ID); err != nil {
		t.Fatalf("delete pm1 failed: %v", err)
	}
	var pm1Count, pm2Count int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM knowledge.translations WHERE entity_type = 'processing_method' AND entity_id = $1`, pm1.ID).Scan(&pm1Count)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM knowledge.translations WHERE entity_type = 'processing_method' AND entity_id = $1`, pm2.ID).Scan(&pm2Count)
	if pm1Count != 0 || pm2Count != 1 {
		t.Fatalf("expected pm1Count=0, pm2Count=1 after cascade delete, got %d, %d", pm1Count, pm2Count)
	}
}
