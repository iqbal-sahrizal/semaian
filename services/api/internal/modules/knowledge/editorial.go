package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type LicenseType string

const (
	LicenseOriginal     LicenseType = "original"
	LicensePublicDomain LicenseType = "public_domain"
	LicenseCC0          LicenseType = "cc0"
	LicenseCCBY         LicenseType = "cc_by"
	LicensePermissive   LicenseType = "permissive"
	LicenseRestricted   LicenseType = "restricted"
	LicenseUnknown      LicenseType = "unknown"
)

type EditorialRole string

const (
	RoleSuperAdmin        EditorialRole = "super_admin"
	RoleKnowledgeEditor   EditorialRole = "knowledge_editor"
	RoleKnowledgeReviewer EditorialRole = "knowledge_reviewer"
	RoleReader            EditorialRole = "reader"
)

type ArticleStatus string

const (
	ArticleDraft       ArticleStatus = "draft"
	ArticleSubmitted   ArticleStatus = "submitted"
	ArticleUnderReview ArticleStatus = "under_review"
	ArticleApproved    ArticleStatus = "approved"
	ArticlePublished   ArticleStatus = "published"
	ArticleSuperseded  ArticleStatus = "superseded"
	ArticleArchived    ArticleStatus = "archived"
)

type ReviewDecision string

const (
	DecisionPending  ReviewDecision = "pending"
	DecisionApproved ReviewDecision = "approved"
	DecisionRejected ReviewDecision = "rejected"
)

var (
	ErrForbidden          = errors.New("editorial permission denied")
	ErrSelfApproval       = errors.New("author cannot approve their own revision")
	ErrInvalidTransition  = errors.New("invalid editorial lifecycle transition")
	ErrLicenseBlocked     = errors.New("source license is restricted, unknown, or lacks commercial compatibility")
	ErrAttributionMissing = errors.New("cc_by license requires attribution_text")
)

type Source struct {
	ID                      string
	SourceType              string
	Title                   string
	CanonicalURL            string
	Publisher               string
	LicenseType             LicenseType
	CommercialUseCompatible bool
	AttributionText         string
	VerificationStatus      string
	Notes                   string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Claim struct {
	ID                 string
	SourceID           string
	SubjectType        string
	SubjectID          string
	ClaimText          string
	VerificationStatus string
	CreatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Article struct {
	ID                    string
	Slug                  string
	Status                ArticleStatus
	AuthorID              string
	PublishedRevisionID   *string
	SupersededByArticleID *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type Revision struct {
	ID              string
	ArticleID       string
	RevisionNumber  int
	Title           string
	Body            string
	AuthorID        string
	ReviewerID      *string
	ReviewDecision  ReviewDecision
	RejectionReason string
	CreatedAt       time.Time
}

// ValidateSourceForPublish enforces licensing and attribution invariants.
func ValidateSourceForPublish(s Source) error {
	switch s.LicenseType {
	case LicenseRestricted, LicenseUnknown, "":
		return ErrLicenseBlocked
	case LicenseCCBY:
		if strings.TrimSpace(s.AttributionText) == "" {
			return ErrAttributionMissing
		}
	case LicenseOriginal, LicensePublicDomain, LicenseCC0, LicensePermissive:
	default:
		return ErrLicenseBlocked
	}
	if !s.CommercialUseCompatible {
		return ErrLicenseBlocked
	}
	return nil
}

type EditorialRepository interface {
	CreateSource(ctx context.Context, s Source) (Source, error)
	CreateClaim(ctx context.Context, c Claim) (Claim, error)
	CreateArticleWithRevision(ctx context.Context, slug, title, body, authorID string) (Article, Revision, error)
	CreateRevision(ctx context.Context, articleID, title, body, authorID string, isSuperAdmin bool) (Revision, error)
	SubmitArticle(ctx context.Context, articleID, actorID string) (Article, error)
	ReviewRevision(ctx context.Context, articleID, revisionID, reviewerID string, decision ReviewDecision, reason string) (Article, Revision, error)
	PublishArticle(ctx context.Context, articleID, revisionID, publisherID string) (Article, error)
	SupersedeArticle(ctx context.Context, oldArticleID, newArticleID, actorID string) (Article, error)
	EmergencyUnpublish(ctx context.Context, articleID, actorID string) (Article, error)
}

type EditorialService struct {
	repo       EditorialRepository
	structRepo Repository
	audit      audit.Recorder
	txRunner   tx.Runner
}

func NewEditorialService(repo EditorialRepository, auditRec audit.Recorder) *EditorialService {
	return &EditorialService{repo: repo, audit: auditRec, txRunner: tx.NoopRunner{}}
}

func (s *EditorialService) WithStructuredRepo(r Repository) *EditorialService {
	s.structRepo = r
	return s
}

func (s *EditorialService) WithTxRunner(r tx.Runner) *EditorialService {
	if r != nil {
		s.txRunner = r
	}
	return s
}

func (s *EditorialService) CreateSource(ctx context.Context, actorID string, role EditorialRole, src Source, cid string) (Source, error) {
	if role != RoleKnowledgeEditor && role != RoleKnowledgeReviewer && role != RoleSuperAdmin {
		return Source{}, ErrForbidden
	}
	if strings.TrimSpace(src.Title) == "" || strings.TrimSpace(src.SourceType) == "" {
		return Source{}, ErrInvalidInput
	}
	created, err := s.repo.CreateSource(ctx, src)
	if err != nil {
		return Source{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &actorID, Action: "knowledge.source.created", TargetType: "source", TargetID: created.ID, CorrelationID: cid,
	})
	return created, nil
}

func (s *EditorialService) CreateClaim(ctx context.Context, actorID string, role EditorialRole, c Claim, cid string) (Claim, error) {
	if role != RoleKnowledgeEditor && role != RoleKnowledgeReviewer && role != RoleSuperAdmin {
		return Claim{}, ErrForbidden
	}
	if strings.TrimSpace(c.SourceID) == "" || strings.TrimSpace(c.ClaimText) == "" {
		return Claim{}, ErrInvalidInput
	}
	c.CreatedBy = actorID
	created, err := s.repo.CreateClaim(ctx, c)
	if err != nil {
		return Claim{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &actorID, Action: "knowledge.claim.created", TargetType: "claim", TargetID: created.ID, CorrelationID: cid,
	})
	return created, nil
}

func (s *EditorialService) CreateArticle(ctx context.Context, actorID string, role EditorialRole, slug, title, body, cid string) (Article, Revision, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Article{}, Revision{}, ErrForbidden
	}
	if !ValidateSlug(slug) || strings.TrimSpace(title) == "" {
		return Article{}, Revision{}, ErrInvalidInput
	}
	art, rev, err := s.repo.CreateArticleWithRevision(ctx, slug, title, body, actorID)
	if err != nil {
		return Article{}, Revision{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &actorID, Action: "knowledge.article.created", TargetType: "article", TargetID: art.ID, CorrelationID: cid,
	})
	return art, rev, nil
}

func (s *EditorialService) Submit(ctx context.Context, actorID string, role EditorialRole, articleID, cid string) (Article, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Article{}, ErrForbidden
	}
	art, err := s.repo.SubmitArticle(ctx, articleID, actorID)
	if err != nil {
		return Article{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &actorID, Action: "knowledge.article.submitted", TargetType: "article", TargetID: articleID, CorrelationID: cid,
	})
	return art, nil
}

func (s *EditorialService) Review(ctx context.Context, reviewerID string, role EditorialRole, articleID, revisionID string, decision ReviewDecision, reason, cid string) (Article, Revision, error) {
	if role != RoleKnowledgeReviewer && role != RoleSuperAdmin {
		return Article{}, Revision{}, ErrForbidden
	}
	if decision == DecisionRejected && strings.TrimSpace(reason) == "" {
		return Article{}, Revision{}, ErrInvalidInput
	}
	if decision != DecisionApproved && decision != DecisionRejected {
		return Article{}, Revision{}, ErrInvalidInput
	}
	art, rev, err := s.repo.ReviewRevision(ctx, articleID, revisionID, reviewerID, decision, reason)
	if err != nil {
		return Article{}, Revision{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &reviewerID, Action: "knowledge.revision." + string(decision), TargetType: "revision", TargetID: revisionID, CorrelationID: cid,
	})
	return art, rev, nil
}

func (s *EditorialService) Publish(ctx context.Context, publisherID string, role EditorialRole, articleID, revisionID, cid string) (Article, error) {
	if role != RoleKnowledgeReviewer && role != RoleSuperAdmin {
		return Article{}, ErrForbidden
	}
	var art Article
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		var err error
		art, err = s.repo.PublishArticle(txCtx, articleID, revisionID, publisherID)
		if err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID: &publisherID, Action: "knowledge.article.published", TargetType: "article", TargetID: articleID, CorrelationID: cid,
		})
	})
	if err != nil {
		return Article{}, err
	}
	return art, nil
}

func (s *EditorialService) Supersede(ctx context.Context, actorID string, role EditorialRole, oldArticleID, newArticleID, cid string) (Article, error) {
	if role != RoleKnowledgeReviewer && role != RoleSuperAdmin {
		return Article{}, ErrForbidden
	}
	var art Article
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		var err error
		art, err = s.repo.SupersedeArticle(txCtx, oldArticleID, newArticleID, actorID)
		if err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID: &actorID, Action: "knowledge.article.superseded", TargetType: "article", TargetID: oldArticleID, CorrelationID: cid,
		})
	})
	if err != nil {
		return Article{}, err
	}
	return art, nil
}

func (s *EditorialService) EmergencyUnpublish(ctx context.Context, actorID string, role EditorialRole, articleID, cid string) (Article, error) {
	if role != RoleSuperAdmin {
		return Article{}, ErrForbidden
	}
	var art Article
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		var err error
		art, err = s.repo.EmergencyUnpublish(txCtx, articleID, actorID)
		if err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID: &actorID, Action: "knowledge.article.emergency_unpublished", TargetType: "article", TargetID: articleID, CorrelationID: cid,
		})
	})
	if err != nil {
		return Article{}, err
	}
	return art, nil
}

// CreateRevision creates a new pending revision for an existing article (P3.1.5-03).
func (s *EditorialService) CreateRevision(ctx context.Context, actorID string, role EditorialRole, articleID, title, body, cid string) (Revision, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Revision{}, ErrForbidden
	}
	if strings.TrimSpace(title) == "" {
		return Revision{}, ErrInvalidInput
	}
	rev, err := s.repo.CreateRevision(ctx, articleID, title, body, actorID, role == RoleSuperAdmin)
	if err != nil {
		return Revision{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &actorID, Action: "knowledge.revision.created", TargetType: "revision", TargetID: rev.ID, CorrelationID: cid,
		Metadata: map[string]any{"article_id": articleID, "revision_number": rev.RevisionNumber},
	})
	return rev, nil
}

// InsertTranslation inserts a localized translation for a structured entity and records an audit event (P3.1.5-01).
func (s *EditorialService) InsertTranslation(ctx context.Context, actorID string, role EditorialRole, t Translation, cid string) (Translation, error) {
	if role != RoleKnowledgeEditor && role != RoleKnowledgeReviewer && role != RoleSuperAdmin {
		return Translation{}, ErrForbidden
	}
	if s.structRepo == nil {
		return Translation{}, errors.New("structured repository not configured")
	}
	created, err := s.structRepo.InsertTranslation(ctx, t)
	if err != nil {
		return Translation{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID: &actorID, Action: "knowledge.translation.created", TargetType: "translation", TargetID: created.ID, CorrelationID: cid,
		Metadata: map[string]any{"entity_type": string(created.EntityType), "entity_id": created.EntityID, "locale": string(created.Locale)},
	})
	return created, nil
}

func (s *EditorialService) CreateCountry(ctx context.Context, actorID string, role EditorialRole, isoCode, slug, cid string) (Country, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Country{}, ErrForbidden
	}
	if !ValidateISOCode(isoCode) || !ValidateSlug(slug) || s.structRepo == nil {
		return Country{}, ErrInvalidInput
	}
	c, err := s.structRepo.CreateCountry(ctx, isoCode, slug)
	if err != nil {
		return Country{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.country.created", TargetType: "country", TargetID: c.ID, CorrelationID: cid})
	return c, nil
}

func (s *EditorialService) CreateRegion(ctx context.Context, actorID string, role EditorialRole, countryID string, parentID *string, slug string, minMASL, maxMASL *int, cid string) (Region, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Region{}, ErrForbidden
	}
	if !ValidateSlug(slug) || s.structRepo == nil {
		return Region{}, ErrInvalidInput
	}
	reg, err := s.structRepo.CreateRegion(ctx, countryID, parentID, slug, minMASL, maxMASL)
	if err != nil {
		return Region{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.region.created", TargetType: "region", TargetID: reg.ID, CorrelationID: cid})
	return reg, nil
}

func (s *EditorialService) CreateProducer(ctx context.Context, actorID string, role EditorialRole, regionID, producerType, slug, cid string) (Producer, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Producer{}, ErrForbidden
	}
	if !ValidateSlug(slug) || s.structRepo == nil {
		return Producer{}, ErrInvalidInput
	}
	p, err := s.structRepo.CreateProducer(ctx, regionID, producerType, slug)
	if err != nil {
		return Producer{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.producer.created", TargetType: "producer", TargetID: p.ID, CorrelationID: cid})
	return p, nil
}

func (s *EditorialService) CreateSpecies(ctx context.Context, actorID string, role EditorialRole, scientificName, slug, cid string) (Species, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Species{}, ErrForbidden
	}
	if strings.TrimSpace(scientificName) == "" || !ValidateSlug(slug) || s.structRepo == nil {
		return Species{}, ErrInvalidInput
	}
	sp, err := s.structRepo.CreateSpecies(ctx, scientificName, slug)
	if err != nil {
		return Species{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.species.created", TargetType: "species", TargetID: sp.ID, CorrelationID: cid})
	return sp, nil
}

func (s *EditorialService) CreateVariety(ctx context.Context, actorID string, role EditorialRole, speciesID, slug, cid string) (Variety, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return Variety{}, ErrForbidden
	}
	if !ValidateSlug(slug) || s.structRepo == nil {
		return Variety{}, ErrInvalidInput
	}
	v, err := s.structRepo.CreateVariety(ctx, speciesID, slug)
	if err != nil {
		return Variety{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.variety.created", TargetType: "variety", TargetID: v.ID, CorrelationID: cid})
	return v, nil
}

func (s *EditorialService) CreateProcessingMethod(ctx context.Context, actorID string, role EditorialRole, slug, cid string) (ProcessingMethod, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return ProcessingMethod{}, ErrForbidden
	}
	if !ValidateSlug(slug) || s.structRepo == nil {
		return ProcessingMethod{}, ErrInvalidInput
	}
	pm, err := s.structRepo.CreateProcessingMethod(ctx, slug)
	if err != nil {
		return ProcessingMethod{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.processing_method.created", TargetType: "processing_method", TargetID: pm.ID, CorrelationID: cid})
	return pm, nil
}

func (s *EditorialService) CreateRoastLevel(ctx context.Context, actorID string, role EditorialRole, slug string, sortOrder int, cid string) (RoastLevel, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return RoastLevel{}, ErrForbidden
	}
	if !ValidateSlug(slug) || s.structRepo == nil {
		return RoastLevel{}, ErrInvalidInput
	}
	rl, err := s.structRepo.CreateRoastLevel(ctx, slug, sortOrder)
	if err != nil {
		return RoastLevel{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.roast_level.created", TargetType: "roast_level", TargetID: rl.ID, CorrelationID: cid})
	return rl, nil
}

func (s *EditorialService) CreateFlavorDescriptor(ctx context.Context, actorID string, role EditorialRole, parentID *string, slug, cid string) (FlavorDescriptor, error) {
	if role != RoleKnowledgeEditor && role != RoleSuperAdmin {
		return FlavorDescriptor{}, ErrForbidden
	}
	if !ValidateSlug(slug) || s.structRepo == nil {
		return FlavorDescriptor{}, ErrInvalidInput
	}
	fd, err := s.structRepo.CreateFlavorDescriptor(ctx, parentID, slug)
	if err != nil {
		return FlavorDescriptor{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{ActorID: &actorID, Action: "knowledge.flavor_descriptor.created", TargetType: "flavor_descriptor", TargetID: fd.ID, CorrelationID: cid})
	return fd, nil
}
