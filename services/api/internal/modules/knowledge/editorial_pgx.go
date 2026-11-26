package knowledge

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type EditorialPGXRepository struct {
	pool  *pgxpool.Pool
	audit audit.Recorder
}

func NewEditorialPGXRepository(pool *pgxpool.Pool) *EditorialPGXRepository {
	return &EditorialPGXRepository{pool: pool}
}

// WithAudit attaches a transaction-aware audit.Recorder to EditorialPGXRepository.
func (r *EditorialPGXRepository) WithAudit(rec audit.Recorder) *EditorialPGXRepository {
	r.audit = rec
	return r
}

func (r *EditorialPGXRepository) CreateSource(ctx context.Context, s Source) (Source, error) {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.VerificationStatus == "" {
		s.VerificationStatus = "draft"
	}
	var out Source
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.sources (id, source_type, title, canonical_url, publisher, license_type, commercial_use_compatible, attribution_text, verification_status, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, source_type, title, canonical_url, publisher, license_type, commercial_use_compatible, attribution_text, verification_status, notes, created_at, updated_at`,
		s.ID, s.SourceType, s.Title, s.CanonicalURL, s.Publisher, string(s.LicenseType), s.CommercialUseCompatible, s.AttributionText, s.VerificationStatus, s.Notes,
	).Scan(&out.ID, &out.SourceType, &out.Title, &out.CanonicalURL, &out.Publisher, &out.LicenseType, &out.CommercialUseCompatible, &out.AttributionText, &out.VerificationStatus, &out.Notes, &out.CreatedAt, &out.UpdatedAt)
	return out, mapPGError(err)
}

func (r *EditorialPGXRepository) CreateClaim(ctx context.Context, c Claim) (Claim, error) {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.VerificationStatus == "" {
		c.VerificationStatus = "draft"
	}
	var out Claim
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.claims (id, source_id, subject_type, subject_id, claim_text, verification_status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, source_id, subject_type, subject_id, claim_text, verification_status, created_by, created_at, updated_at`,
		c.ID, c.SourceID, c.SubjectType, c.SubjectID, c.ClaimText, c.VerificationStatus, c.CreatedBy,
	).Scan(&out.ID, &out.SourceID, &out.SubjectType, &out.SubjectID, &out.ClaimText, &out.VerificationStatus, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	return out, mapPGError(err)
}

func (r *EditorialPGXRepository) CreateArticleWithRevision(ctx context.Context, slug, title, body, authorID string) (Article, Revision, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Article{}, Revision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	artID := uuid.NewString()
	revID := uuid.NewString()

	var art Article
	err = tx.QueryRow(ctx,
		`INSERT INTO knowledge.articles (id, slug, status, author_id) VALUES ($1, $2, 'draft', $3)
		 RETURNING id, slug, status, author_id, published_revision_id, superseded_by_article_id, created_at, updated_at`,
		artID, slug, authorID,
	).Scan(&art.ID, &art.Slug, &art.Status, &art.AuthorID, &art.PublishedRevisionID, &art.SupersededByArticleID, &art.CreatedAt, &art.UpdatedAt)
	if err != nil {
		return Article{}, Revision{}, mapPGError(err)
	}

	var rev Revision
	err = tx.QueryRow(ctx,
		`INSERT INTO knowledge.revisions (id, article_id, revision_number, title, body, author_id)
		 VALUES ($1, $2, 1, $3, $4, $5)
		 RETURNING id, article_id, revision_number, title, body, author_id, reviewer_id, review_decision, rejection_reason, created_at`,
		revID, artID, title, body, authorID,
	).Scan(&rev.ID, &rev.ArticleID, &rev.RevisionNumber, &rev.Title, &rev.Body, &rev.AuthorID, &rev.ReviewerID, &rev.ReviewDecision, &rev.RejectionReason, &rev.CreatedAt)
	if err != nil {
		return Article{}, Revision{}, mapPGError(err)
	}

	if r.audit != nil {
		if err := r.audit.Record(audit.WithTx(ctx, tx), audit.Event{
			ActorID: &authorID, Action: "knowledge.article.created", TargetType: "article", TargetID: art.ID,
		}); err != nil {
			return Article{}, Revision{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Article{}, Revision{}, err
	}
	return art, rev, nil
}

func (r *EditorialPGXRepository) CreateRevision(ctx context.Context, articleID, title, body, authorID string, isSuperAdmin bool) (Revision, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Revision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var artAuthorID string
	err = tx.QueryRow(ctx, `SELECT author_id FROM knowledge.articles WHERE id = $1 FOR UPDATE`, articleID).Scan(&artAuthorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	if err != nil {
		return Revision{}, err
	}
	if !isSuperAdmin && artAuthorID != authorID {
		return Revision{}, ErrForbidden
	}

	var nextRev int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision_number), 0) + 1 FROM knowledge.revisions WHERE article_id = $1`, articleID).Scan(&nextRev); err != nil {
		return Revision{}, err
	}

	var rev Revision
	err = tx.QueryRow(ctx,
		`INSERT INTO knowledge.revisions (id, article_id, revision_number, title, body, author_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, article_id, revision_number, title, body, author_id, reviewer_id, review_decision, rejection_reason, created_at`,
		uuid.NewString(), articleID, nextRev, title, body, authorID,
	).Scan(&rev.ID, &rev.ArticleID, &rev.RevisionNumber, &rev.Title, &rev.Body, &rev.AuthorID, &rev.ReviewerID, &rev.ReviewDecision, &rev.RejectionReason, &rev.CreatedAt)
	if err != nil {
		return Revision{}, mapPGError(err)
	}

	if r.audit != nil {
		if err := r.audit.Record(audit.WithTx(ctx, tx), audit.Event{
			ActorID: &authorID, Action: "knowledge.revision.created", TargetType: "revision", TargetID: rev.ID,
			Metadata: map[string]any{"article_id": articleID, "revision_number": rev.RevisionNumber},
		}); err != nil {
			return Revision{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Revision{}, err
	}
	return rev, nil
}

func (r *EditorialPGXRepository) SubmitArticle(ctx context.Context, articleID, actorID string) (Article, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Article{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status ArticleStatus
	var authorID string
	err = tx.QueryRow(ctx, `SELECT status, author_id FROM knowledge.articles WHERE id = $1 FOR UPDATE`, articleID).Scan(&status, &authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, err
	}
	if authorID != actorID {
		return Article{}, ErrForbidden
	}
	if status != ArticleDraft {
		return Article{}, ErrInvalidTransition
	}

	var art Article
	err = tx.QueryRow(ctx,
		`UPDATE knowledge.articles SET status = 'submitted', updated_at = now() WHERE id = $1
		 RETURNING id, slug, status, author_id, published_revision_id, superseded_by_article_id, created_at, updated_at`,
		articleID,
	).Scan(&art.ID, &art.Slug, &art.Status, &art.AuthorID, &art.PublishedRevisionID, &art.SupersededByArticleID, &art.CreatedAt, &art.UpdatedAt)
	if err != nil {
		return Article{}, mapPGError(err)
	}
	if r.audit != nil {
		if err := r.audit.Record(audit.WithTx(ctx, tx), audit.Event{
			Action: "knowledge.article.submitted", TargetType: "article", TargetID: articleID,
		}); err != nil {
			return Article{}, err
		}
	}
	return art, tx.Commit(ctx)
}

func (r *EditorialPGXRepository) ReviewRevision(ctx context.Context, articleID, revisionID, reviewerID string, decision ReviewDecision, reason string) (Article, Revision, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Article{}, Revision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status ArticleStatus
	err = tx.QueryRow(ctx, `SELECT status FROM knowledge.articles WHERE id = $1 FOR UPDATE`, articleID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, Revision{}, ErrNotFound
	}
	if err != nil {
		return Article{}, Revision{}, err
	}
	if status != ArticleSubmitted && status != ArticleUnderReview {
		return Article{}, Revision{}, ErrInvalidTransition
	}

	var authorID string
	err = tx.QueryRow(ctx, `SELECT author_id FROM knowledge.revisions WHERE id = $1 AND article_id = $2 FOR UPDATE`, revisionID, articleID).Scan(&authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, Revision{}, ErrNotFound
	}
	if err != nil {
		return Article{}, Revision{}, err
	}
	if authorID == reviewerID && decision == DecisionApproved {
		return Article{}, Revision{}, ErrSelfApproval
	}

	nextArtStatus := ArticleApproved
	if decision == DecisionRejected {
		nextArtStatus = ArticleDraft
	}

	var rev Revision
	err = tx.QueryRow(ctx,
		`UPDATE knowledge.revisions SET reviewer_id = $2, review_decision = $3, rejection_reason = $4 WHERE id = $1
		 RETURNING id, article_id, revision_number, title, body, author_id, reviewer_id, review_decision, rejection_reason, created_at`,
		revisionID, reviewerID, string(decision), reason,
	).Scan(&rev.ID, &rev.ArticleID, &rev.RevisionNumber, &rev.Title, &rev.Body, &rev.AuthorID, &rev.ReviewerID, &rev.ReviewDecision, &rev.RejectionReason, &rev.CreatedAt)
	if err != nil {
		return Article{}, Revision{}, mapPGError(err)
	}

	var art Article
	err = tx.QueryRow(ctx,
		`UPDATE knowledge.articles SET status = $2, updated_at = now() WHERE id = $1
		 RETURNING id, slug, status, author_id, published_revision_id, superseded_by_article_id, created_at, updated_at`,
		articleID, string(nextArtStatus),
	).Scan(&art.ID, &art.Slug, &art.Status, &art.AuthorID, &art.PublishedRevisionID, &art.SupersededByArticleID, &art.CreatedAt, &art.UpdatedAt)
	if err != nil {
		return Article{}, Revision{}, mapPGError(err)
	}
	if r.audit != nil {
		if err := r.audit.Record(audit.WithTx(ctx, tx), audit.Event{
			ActorID: &reviewerID, Action: "knowledge.revision." + string(decision), TargetType: "revision", TargetID: revisionID,
		}); err != nil {
			return Article{}, Revision{}, err
		}
	}

	return art, rev, tx.Commit(ctx)
}

func (r *EditorialPGXRepository) beginOrReuse(ctx context.Context) (pgx.Tx, bool, error) {
	if t, ok := tx.TxFromContext(ctx); ok {
		return t, false, nil
	}
	t, err := r.pool.Begin(ctx)
	return t, true, err
}

func (r *EditorialPGXRepository) PublishArticle(ctx context.Context, articleID, revisionID, _ string) (Article, error) {
	t, owned, err := r.beginOrReuse(ctx)
	if err != nil {
		return Article{}, err
	}
	if owned {
		defer func() { _ = t.Rollback(ctx) }()
	}

	var status ArticleStatus
	err = t.QueryRow(ctx, `SELECT status FROM knowledge.articles WHERE id = $1 FOR UPDATE`, articleID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, err
	}
	if status != ArticleApproved {
		return Article{}, ErrInvalidTransition
	}

	var decision ReviewDecision
	err = t.QueryRow(ctx, `SELECT review_decision FROM knowledge.revisions WHERE id = $1 AND article_id = $2`, revisionID, articleID).Scan(&decision)
	if errors.Is(err, pgx.ErrNoRows) || decision != DecisionApproved {
		return Article{}, ErrInvalidTransition
	}
	if err != nil {
		return Article{}, err
	}

	// SEC-01: Atomically lock and validate all linked claims & sources (TOCTOU-safe).
	rows, err := t.Query(ctx,
		`SELECT s.license_type, s.commercial_use_compatible, s.attribution_text
		 FROM knowledge.claims c
		 JOIN knowledge.sources s ON s.id = c.source_id
		 WHERE c.subject_type = 'article' AND c.subject_id = $1
		 FOR SHARE OF c, s`,
		articleID,
	)
	if err != nil {
		return Article{}, err
	}
	claimCount := 0
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.LicenseType, &src.CommercialUseCompatible, &src.AttributionText); err != nil {
			rows.Close()
			return Article{}, err
		}
		claimCount++
		if err := ValidateSourceForPublish(src); err != nil {
			rows.Close()
			return Article{}, err
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Article{}, err
	}
	if claimCount == 0 {
		return Article{}, ErrLicenseBlocked
	}

	var art Article
	err = t.QueryRow(ctx,
		`UPDATE knowledge.articles SET status = 'published', published_revision_id = $2, updated_at = now() WHERE id = $1
		 RETURNING id, slug, status, author_id, published_revision_id, superseded_by_article_id, created_at, updated_at`,
		articleID, revisionID,
	).Scan(&art.ID, &art.Slug, &art.Status, &art.AuthorID, &art.PublishedRevisionID, &art.SupersededByArticleID, &art.CreatedAt, &art.UpdatedAt)
	if err != nil {
		return Article{}, mapPGError(err)
	}
	if owned {
		return art, t.Commit(ctx)
	}
	return art, nil
}

func (r *EditorialPGXRepository) SupersedeArticle(ctx context.Context, oldArticleID, newArticleID, _ string) (Article, error) {
	var art Article
	err := tx.Executor(ctx, r.pool).QueryRow(ctx,
		`UPDATE knowledge.articles SET status = 'superseded', superseded_by_article_id = $2, updated_at = now()
		 WHERE id = $1 AND status = 'published'
		 RETURNING id, slug, status, author_id, published_revision_id, superseded_by_article_id, created_at, updated_at`,
		oldArticleID, newArticleID,
	).Scan(&art.ID, &art.Slug, &art.Status, &art.AuthorID, &art.PublishedRevisionID, &art.SupersededByArticleID, &art.CreatedAt, &art.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, ErrInvalidTransition
	}
	return art, mapPGError(err)
}

func (r *EditorialPGXRepository) EmergencyUnpublish(ctx context.Context, articleID, _ string) (Article, error) {
	var art Article
	err := tx.Executor(ctx, r.pool).QueryRow(ctx,
		`UPDATE knowledge.articles SET status = 'archived', updated_at = now()
		 WHERE id = $1 AND status = 'published'
		 RETURNING id, slug, status, author_id, published_revision_id, superseded_by_article_id, created_at, updated_at`,
		articleID,
	).Scan(&art.ID, &art.Slug, &art.Status, &art.AuthorID, &art.PublishedRevisionID, &art.SupersededByArticleID, &art.CreatedAt, &art.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, ErrInvalidTransition
	}
	return art, mapPGError(err)
}
