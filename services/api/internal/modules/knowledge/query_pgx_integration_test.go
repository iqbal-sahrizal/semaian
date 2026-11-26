//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/platform/db"
)

func TestPublicKnowledgeSearchFallbackAndVisibility(t *testing.T) {
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
	qRepo := knowledge.NewQueryRepository(pool)
	edSvc := knowledge.NewEditorialService(knowledge.NewEditorialPGXRepository(pool), audit.NewPGXRecorder(pool))

	pmPub, err := repo.CreateProcessingMethod(ctx, "pub-proc-"+uuid.NewString()[:6])
	if err != nil {
		t.Fatalf("CreateProcessingMethod pub failed: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge.processing_methods WHERE id = $1`, pmPub.ID)
	}()

	// Insert published ID translation and draft EN translation
	if _, err := repo.InsertTranslation(ctx, knowledge.Translation{
		EntityType: knowledge.EntityProcessingMethod,
		EntityID:   pmPub.ID,
		Locale:     knowledge.LocaleID,
		Title:      "Kopi Gayo Spesialti",
		Summary:    "Proses giling basah khas Aceh",
		Status:     "published",
	}); err != nil {
		t.Fatalf("InsertTranslation id failed: %v", err)
	}
	if _, err := repo.InsertTranslation(ctx, knowledge.Translation{
		EntityType: knowledge.EntityProcessingMethod,
		EntityID:   pmPub.ID,
		Locale:     knowledge.LocaleEN,
		Title:      "Draft English Title",
		Status:     "draft",
	}); err != nil {
		t.Fatalf("InsertTranslation en draft failed: %v", err)
	}

	// Fallback: requesting EN when EN is draft must fall back to published ID
	tr, err := qRepo.ResolvePublishedTranslation(ctx, knowledge.EntityProcessingMethod, pmPub.ID, knowledge.LocaleEN)
	if err != nil || tr.Locale != knowledge.LocaleID {
		t.Fatalf("expected fallback to 'id', got err=%v locale=%v", err, tr.Locale)
	}

	// Search: published ID is found; draft EN is excluded
	res, err := qRepo.SearchPublished(ctx, "Gayo", knowledge.LocaleID, 20, 0)
	if err != nil || len(res) == 0 {
		t.Fatalf("expected search results for 'Gayo', got err=%v len=%d", err, len(res))
	}
	draftRes, err := qRepo.SearchPublished(ctx, "Draft English Title", knowledge.LocaleEN, 20, 0)
	if err != nil || len(draftRes) != 0 {
		t.Fatalf("expected 0 results for draft translation, got err=%v len=%d", err, len(draftRes))
	}

	// Draft article excluded from public GetPublishedArticleBySlug
	editorID := uuid.NewString()
	draftArt, _, err := edSvc.CreateArticle(ctx, editorID, knowledge.RoleKnowledgeEditor, "draft-art-"+uuid.NewString()[:6], "Draft", "Body", "cid-q-1")
	if err != nil {
		t.Fatalf("CreateArticle failed: %v", err)
	}
	if _, err := qRepo.GetPublishedArticleBySlug(ctx, draftArt.Slug); !errors.Is(err, knowledge.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for draft article, got %v", err)
	}

	// HTTP handler tests: invalid locale -> 400, short search -> 400, valid search -> 200
	h := knowledge.NewHandler(qRepo, edSvc, nil)
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		h.RegisterPublicRoutes(v1)
	})

	reqBadLoc := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/processes?locale=fr", nil)
	recBadLoc := httptest.NewRecorder()
	r.ServeHTTP(recBadLoc, reqBadLoc)
	if recBadLoc.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid locale, got %d", recBadLoc.Code)
	}

	reqShortQ := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search?q=a", nil)
	recShortQ := httptest.NewRecorder()
	r.ServeHTTP(recShortQ, reqShortQ)
	if recShortQ.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short query, got %d", recShortQ.Code)
	}

	reqValidQ := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search?q=Gayo&locale=id", nil)
	recValidQ := httptest.NewRecorder()
	r.ServeHTTP(recValidQ, reqValidQ)
	if recValidQ.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid search, got %d", recValidQ.Code)
	}
}
