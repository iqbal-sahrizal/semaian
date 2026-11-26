package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicItem struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	EntityType     string `json:"entity_type"`
	Title          string `json:"title"`
	Summary        string `json:"summary,omitempty"`
	Body           string `json:"body,omitempty"`
	ResolvedLocale string `json:"resolved_locale"`
}

type PublicArticle struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	ResolvedLocale string    `json:"resolved_locale"`
	PublishedAt    time.Time `json:"published_at"`
}

type SearchResult struct {
	EntityType     string `json:"entity_type"`
	EntityID       string `json:"entity_id"`
	Title          string `json:"title"`
	Summary        string `json:"summary,omitempty"`
	ResolvedLocale string `json:"resolved_locale"`
}

type QueryRepository struct {
	pool *pgxpool.Pool
}

func NewQueryRepository(pool *pgxpool.Pool) *QueryRepository {
	return &QueryRepository{pool: pool}
}

// ResolvePublishedTranslation applies deterministic fallback: requested -> id -> en -> ErrNotFound.
// Only translations with status = 'published' are returned.
func (r *QueryRepository) ResolvePublishedTranslation(ctx context.Context, entityType EntityType, entityID string, requested Locale) (Translation, error) {
	if !entityType.Valid() || !requested.Valid() {
		return Translation{}, ErrInvalidInput
	}

	var out Translation
	err := r.pool.QueryRow(ctx,
		`SELECT id, entity_type, entity_id, locale, title, summary, body, status, created_at, updated_at
		 FROM knowledge.translations
		 WHERE entity_type = $1 AND entity_id = $2 AND status = 'published'
		 ORDER BY CASE
		     WHEN locale = $3 THEN 0
		     WHEN locale = 'id' THEN 1
		     WHEN locale = 'en' THEN 2
		     ELSE 3
		 END
		 LIMIT 1`,
		string(entityType), entityID, string(requested),
	).Scan(&out.ID, &out.EntityType, &out.EntityID, &out.Locale, &out.Title, &out.Summary, &out.Body, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Translation{}, ErrNotFound
	}
	return out, err
}

// ListPublishedEntities returns paginated structured entities that have at least one published translation.
func (r *QueryRepository) ListPublishedEntities(ctx context.Context, entityType EntityType, requested Locale, limit, offset int) ([]PublicItem, error) {
	if !requested.Valid() || limit <= 0 || limit > 50 || offset < 0 {
		return nil, ErrInvalidInput
	}

	var table string
	switch entityType {
	case EntityRegion:
		table = "knowledge.regions"
	case EntityVariety:
		table = "knowledge.varieties"
	case EntityProcessingMethod:
		table = "knowledge.processing_methods"
	case EntityRoastLevel:
		table = "knowledge.roast_levels"
	case EntityFlavorDescriptor:
		table = "knowledge.flavor_descriptors"
	default:
		return nil, ErrInvalidInput
	}

	query := `
		SELECT e.id, e.slug, t.locale, t.title, t.summary
		FROM ` + table + ` e
		JOIN LATERAL (
			SELECT locale, title, summary
			FROM knowledge.translations
			WHERE entity_type = $1 AND entity_id = e.id AND status = 'published'
			ORDER BY CASE WHEN locale = $2 THEN 0 WHEN locale = 'id' THEN 1 ELSE 2 END
			LIMIT 1
		) t ON TRUE
		ORDER BY e.slug ASC, e.id ASC
		LIMIT $3 OFFSET $4`

	rows, err := r.pool.Query(ctx, query, string(entityType), string(requested), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PublicItem
	for rows.Next() {
		var it PublicItem
		it.EntityType = string(entityType)
		if err := rows.Scan(&it.ID, &it.Slug, &it.ResolvedLocale, &it.Title, &it.Summary); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetPublishedEntityBySlug returns a single published structured entity by slug with deterministic locale fallback.
func (r *QueryRepository) GetPublishedEntityBySlug(ctx context.Context, entityType EntityType, slug string, requested Locale) (PublicItem, error) {
	if !requested.Valid() || strings.TrimSpace(slug) == "" {
		return PublicItem{}, ErrInvalidInput
	}

	var table string
	switch entityType {
	case EntityRegion:
		table = "knowledge.regions"
	case EntityVariety:
		table = "knowledge.varieties"
	default:
		return PublicItem{}, ErrInvalidInput
	}

	query := `
		SELECT e.id, e.slug, t.locale, t.title, t.summary, t.body
		FROM ` + table + ` e
		JOIN LATERAL (
			SELECT locale, title, summary, body
			FROM knowledge.translations
			WHERE entity_type = $1 AND entity_id = e.id AND status = 'published'
			ORDER BY CASE WHEN locale = $2 THEN 0 WHEN locale = 'id' THEN 1 ELSE 2 END
			LIMIT 1
		) t ON TRUE
		WHERE e.slug = $3
		LIMIT 1`

	var it PublicItem
	it.EntityType = string(entityType)
	err := r.pool.QueryRow(ctx, query, string(entityType), string(requested), slug).
		Scan(&it.ID, &it.Slug, &it.ResolvedLocale, &it.Title, &it.Summary, &it.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicItem{}, ErrNotFound
	}
	return it, err
}

// ListPublishedArticles returns only articles with status = 'published'.
func (r *QueryRepository) ListPublishedArticles(ctx context.Context, limit, offset int) ([]PublicArticle, error) {
	if limit <= 0 || limit > 50 || offset < 0 {
		return nil, ErrInvalidInput
	}
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.slug, r.title, r.body, a.updated_at
		 FROM knowledge.articles a
		 JOIN knowledge.revisions r ON r.id = a.published_revision_id
		 WHERE a.status = 'published'
		 ORDER BY a.updated_at DESC, a.id ASC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PublicArticle
	for rows.Next() {
		var a PublicArticle
		a.ResolvedLocale = "id"
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Body, &a.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetPublishedArticleBySlug returns a published article or ErrNotFound for draft/submitted/archived/superseded.
func (r *QueryRepository) GetPublishedArticleBySlug(ctx context.Context, slug string) (PublicArticle, error) {
	var a PublicArticle
	a.ResolvedLocale = "id"
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.slug, r.title, r.body, a.updated_at
		 FROM knowledge.articles a
		 JOIN knowledge.revisions r ON r.id = a.published_revision_id
		 WHERE a.slug = $1 AND a.status = 'published'`,
		slug,
	).Scan(&a.ID, &a.Slug, &a.Title, &a.Body, &a.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicArticle{}, ErrNotFound
	}
	return a, err
}

// SearchPublished executes PostgreSQL FTS over published translations with <=2s timeout.
func (r *QueryRepository) SearchPublished(ctx context.Context, q string, requested Locale, limit, offset int) ([]SearchResult, error) {
	q = strings.TrimSpace(q)
	if len(q) < 2 || len(q) > 100 || !requested.Valid() || limit <= 0 || limit > 50 || offset < 0 {
		return nil, ErrInvalidInput
	}

	qCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	rows, err := r.pool.Query(qCtx,
		`SELECT entity_type, entity_id, title, summary, locale
		 FROM knowledge.translations
		 WHERE status = 'published'
		   AND search_vector @@ plainto_tsquery('simple', $1)
		 ORDER BY ts_rank(search_vector, plainto_tsquery('simple', $1)) DESC,
		          CASE WHEN locale = $2 THEN 0 WHEN locale = 'id' THEN 1 ELSE 2 END,
		          id ASC
		 LIMIT $3 OFFSET $4`,
		q, string(requested), limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		var sr SearchResult
		if err := rows.Scan(&sr.EntityType, &sr.EntityID, &sr.Title, &sr.Summary, &sr.ResolvedLocale); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}
