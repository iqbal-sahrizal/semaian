package knowledge_test

import (
	"context"
	"errors"
	"testing"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
)

func TestLicenseCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		src     knowledge.Source
		wantErr error
	}{
		{"unknown blocked", knowledge.Source{LicenseType: knowledge.LicenseUnknown, CommercialUseCompatible: true}, knowledge.ErrLicenseBlocked},
		{"restricted blocked", knowledge.Source{LicenseType: knowledge.LicenseRestricted, CommercialUseCompatible: true}, knowledge.ErrLicenseBlocked},
		{"non-commercial blocked", knowledge.Source{LicenseType: knowledge.LicenseCC0, CommercialUseCompatible: false}, knowledge.ErrLicenseBlocked},
		{"cc_by missing attribution", knowledge.Source{LicenseType: knowledge.LicenseCCBY, CommercialUseCompatible: true}, knowledge.ErrAttributionMissing},
		{"cc_by with attribution", knowledge.Source{LicenseType: knowledge.LicenseCCBY, CommercialUseCompatible: true, AttributionText: "WCR 2024"}, nil},
		{"original valid", knowledge.Source{LicenseType: knowledge.LicenseOriginal, CommercialUseCompatible: true}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := knowledge.ValidateSourceForPublish(tc.src)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestEditorialRoleAuthorization(t *testing.T) {
	svc := knowledge.NewEditorialService(nil, audit.NoopRecorder{})
	ctx := context.Background()

	if _, _, err := svc.CreateArticle(ctx, "u-1", knowledge.RoleReader, "slug-1", "Title", "Body", "cid"); !errors.Is(err, knowledge.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for reader, got %v", err)
	}
	if _, _, err := svc.Review(ctx, "u-1", knowledge.RoleKnowledgeEditor, "a-1", "r-1", knowledge.DecisionApproved, "", "cid"); !errors.Is(err, knowledge.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for editor reviewing, got %v", err)
	}
	if _, err := svc.EmergencyUnpublish(ctx, "u-1", knowledge.RoleKnowledgeReviewer, "a-1", "cid"); !errors.Is(err, knowledge.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for reviewer emergency unpublish, got %v", err)
	}
}
