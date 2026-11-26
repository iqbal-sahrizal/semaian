package knowledge_test

import (
	"errors"
	"testing"

	"github.com/semaian/platform/services/api/internal/modules/knowledge"
)

func TestDomainValidation(t *testing.T) {
	if !knowledge.ValidateISOCode("ID") || knowledge.ValidateISOCode("id") || knowledge.ValidateISOCode("IDN") {
		t.Fatal("ISO code validation failed")
	}
	if !knowledge.ValidateSlug("aceh-gayo") || knowledge.ValidateSlug("-bad") || knowledge.ValidateSlug("Bad_Slug") {
		t.Fatal("slug validation failed")
	}
	if !knowledge.EntityCountry.Valid() || knowledge.EntityType("article").Valid() {
		t.Fatal("entity type validation failed for Phase 3.1.1")
	}
	if !knowledge.LocaleID.Valid() || !knowledge.LocaleEN.Valid() || knowledge.Locale("fr").Valid() {
		t.Fatal("locale validation failed")
	}
}

func TestValidateTranslationBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		tr      knowledge.Translation
		wantErr error
	}{
		{
			name:    "valid id translation",
			tr:      knowledge.Translation{EntityType: knowledge.EntityCountry, EntityID: "id-1", Locale: knowledge.LocaleID, Title: "Indonesia"},
			wantErr: nil,
		},
		{
			name:    "valid en translation",
			tr:      knowledge.Translation{EntityType: knowledge.EntityFlavorDescriptor, EntityID: "id-2", Locale: knowledge.LocaleEN, Title: "Citrus"},
			wantErr: nil,
		},
		{
			name:    "empty entity type",
			tr:      knowledge.Translation{EntityType: "", EntityID: "id-1", Locale: knowledge.LocaleID, Title: "Test"},
			wantErr: knowledge.ErrInvalidInput,
		},
		{
			name:    "unsupported entity type",
			tr:      knowledge.Translation{EntityType: "article", EntityID: "id-1", Locale: knowledge.LocaleID, Title: "Test"},
			wantErr: knowledge.ErrInvalidInput,
		},
		{
			name:    "empty entity id",
			tr:      knowledge.Translation{EntityType: knowledge.EntityRegion, EntityID: "", Locale: knowledge.LocaleID, Title: "Gayo"},
			wantErr: knowledge.ErrInvalidInput,
		},
		{
			name:    "whitespace entity id",
			tr:      knowledge.Translation{EntityType: knowledge.EntityRegion, EntityID: "   ", Locale: knowledge.LocaleID, Title: "Gayo"},
			wantErr: knowledge.ErrInvalidInput,
		},
		{
			name:    "invalid locale",
			tr:      knowledge.Translation{EntityType: knowledge.EntityRegion, EntityID: "id-1", Locale: "fr", Title: "Gayo"},
			wantErr: knowledge.ErrInvalidInput,
		},
		{
			name:    "empty title",
			tr:      knowledge.Translation{EntityType: knowledge.EntityRegion, EntityID: "id-1", Locale: knowledge.LocaleID, Title: ""},
			wantErr: knowledge.ErrInvalidInput,
		},
		{
			name:    "whitespace title",
			tr:      knowledge.Translation{EntityType: knowledge.EntityRegion, EntityID: "id-1", Locale: knowledge.LocaleID, Title: "   "},
			wantErr: knowledge.ErrInvalidInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := knowledge.ValidateTranslation(tc.tr)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}
