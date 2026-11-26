package knowledge

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/semaian/platform/services/api/internal/platform/auth"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

// EditorialRoleResolver resolves a platform-level editorial role for an actor.
type EditorialRoleResolver interface {
	ResolveEditorialRole(actor auth.Actor) EditorialRole
}

type ipRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string][]time.Time
	limit      int
	window     time.Duration
	maxEntries int
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		buckets:    make(map[string][]time.Time),
		limit:      limit,
		window:     window,
		maxEntries: 10000,
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)

	// Bounded memory: evict expired buckets when map grows large.
	if len(l.buckets) >= l.maxEntries {
		for k, ts := range l.buckets {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(l.buckets, k)
			}
		}
	}

	times := l.buckets[ip]
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.buckets[ip] = kept
		return false
	}
	l.buckets[ip] = append(kept, now)
	return true
}

type Handler struct {
	query     *QueryRepository
	repo      Repository
	editorial *EditorialService
	roles     EditorialRoleResolver
	limiter   *ipRateLimiter
}

func NewHandler(query *QueryRepository, editorial *EditorialService, roles EditorialRoleResolver) *Handler {
	return &Handler{
		query:     query,
		editorial: editorial,
		roles:     roles,
		limiter:   newIPRateLimiter(60, time.Minute),
	}
}

// WithRepository attaches the structured knowledge Repository for editorial translation writes.
func (h *Handler) WithRepository(repo Repository) *Handler {
	h.repo = repo
	return h
}

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/knowledge/origins", h.listEntities(EntityRegion))
	r.Get("/knowledge/origins/{slug}", h.getEntity(EntityRegion))
	r.Get("/knowledge/varieties", h.listEntities(EntityVariety))
	r.Get("/knowledge/varieties/{slug}", h.getEntity(EntityVariety))
	r.Get("/knowledge/processes", h.listEntities(EntityProcessingMethod))
	r.Get("/knowledge/roast-levels", h.listEntities(EntityRoastLevel))
	r.Get("/knowledge/flavors", h.listEntities(EntityFlavorDescriptor))
	r.Get("/knowledge/articles", h.listArticles)
	r.Get("/knowledge/articles/{slug}", h.getArticle)
	r.Get("/knowledge/search", h.search)
}

func (h *Handler) RegisterEditorialRoutes(r chi.Router) {
	r.Post("/editorial/knowledge/sources", h.createSource)
	r.Post("/editorial/knowledge/claims", h.createClaim)
	r.Post("/editorial/knowledge/translations", h.insertTranslation)
	r.Post("/editorial/knowledge/countries", h.createCountry)
	r.Post("/editorial/knowledge/regions", h.createRegion)
	r.Post("/editorial/knowledge/producers", h.createProducer)
	r.Post("/editorial/knowledge/species", h.createSpecies)
	r.Post("/editorial/knowledge/varieties", h.createVariety)
	r.Post("/editorial/knowledge/processing-methods", h.createProcessingMethod)
	r.Post("/editorial/knowledge/roast-levels", h.createRoastLevel)
	r.Post("/editorial/knowledge/flavors", h.createFlavorDescriptor)
	r.Post("/editorial/knowledge/articles", h.createArticle)
	r.Post("/editorial/knowledge/articles/{articleID}/revisions", h.createRevision)
	r.Post("/editorial/knowledge/articles/{articleID}/submit", h.submitArticle)
	r.Post("/editorial/knowledge/articles/{articleID}/revisions/{revisionID}/review", h.reviewRevision)
	r.Post("/editorial/knowledge/articles/{articleID}/revisions/{revisionID}/publish", h.publishArticle)
	r.Post("/editorial/knowledge/articles/{articleID}/supersede", h.supersedeArticle)
	r.Post("/editorial/knowledge/articles/{articleID}/emergency-unpublish", h.emergencyUnpublish)
}

func parseLocaleAndPagination(r *http.Request) (Locale, int, int, error) {
	loc := Locale(r.URL.Query().Get("locale"))
	if loc == "" {
		loc = LocaleID
	}
	if !loc.Valid() {
		return "", 0, 0, ErrInvalidInput
	}
	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l <= 0 || l > 50 {
			return "", 0, 0, ErrInvalidInput
		}
		limit = l
	}
	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 {
			return "", 0, 0, ErrInvalidInput
		}
		offset = o
	}
	return loc, limit, offset, nil
}

func (h *Handler) listEntities(et EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loc, limit, offset, err := parseLocaleAndPagination(r)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid locale or pagination parameter")
			return
		}
		items, err := h.query.ListPublishedEntities(r.Context(), et, loc, limit, offset)
		if err != nil {
			writeKnowledgeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
	}
}

func (h *Handler) getEntity(et EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loc, _, _, err := parseLocaleAndPagination(r)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid locale parameter")
			return
		}
		item, err := h.query.GetPublishedEntityBySlug(r.Context(), et, chi.URLParam(r, "slug"), loc)
		if err != nil {
			writeKnowledgeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func (h *Handler) listArticles(w http.ResponseWriter, r *http.Request) {
	_, limit, offset, err := parseLocaleAndPagination(r)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid locale or pagination parameter")
		return
	}
	items, err := h.query.ListPublishedArticles(r.Context(), limit, offset)
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"articles": items, "limit": limit, "offset": offset})
}

func (h *Handler) getArticle(w http.ResponseWriter, r *http.Request) {
	art, err := h.query.GetPublishedArticleBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !h.limiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		httpx.WriteProblem(w, r, http.StatusTooManyRequests, "Too Many Requests", "search rate limit exceeded")
		return
	}
	loc, limit, offset, err := parseLocaleAndPagination(r)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid locale or pagination parameter")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 || len(q) > 100 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "query parameter q must be between 2 and 100 characters")
		return
	}
	results, err := h.query.SearchPublished(r.Context(), q, loc, limit, offset)
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "limit": limit, "offset": offset})
}

func (h *Handler) actorAndRole(r *http.Request) (string, EditorialRole, bool) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		return "", "", false
	}
	role := RoleReader
	if h.roles != nil {
		role = h.roles.ResolveEditorialRole(actor)
	}
	return actor.Subject, role, true
}

func (h *Handler) insertTranslation(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	if h.editorial == nil {
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "editorial service not configured")
		return
	}
	var req struct {
		EntityType EntityType `json:"entity_type"`
		EntityID   string     `json:"entity_id"`
		Locale     Locale     `json:"locale"`
		Title      string     `json:"title"`
		Summary    string     `json:"summary"`
		Body       string     `json:"body"`
		Status     string     `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	tr, err := h.editorial.InsertTranslation(r.Context(), actorID, role, Translation{
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		Locale:     req.Locale,
		Title:      req.Title,
		Summary:    req.Summary,
		Body:       req.Body,
		Status:     req.Status,
	}, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, tr)
}

func (h *Handler) createCountry(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		ISOCode string `json:"iso_code"`
		Slug    string `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	c, err := h.editorial.CreateCountry(r.Context(), actorID, role, req.ISOCode, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) createRegion(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		CountryID string  `json:"country_id"`
		ParentID  *string `json:"parent_id"`
		Slug      string  `json:"slug"`
		MinMASL   *int    `json:"min_masl"`
		MaxMASL   *int    `json:"max_masl"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	reg, err := h.editorial.CreateRegion(r.Context(), actorID, role, req.CountryID, req.ParentID, req.Slug, req.MinMASL, req.MaxMASL, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, reg)
}

func (h *Handler) createProducer(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		RegionID     string `json:"region_id"`
		ProducerType string `json:"producer_type"`
		Slug         string `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	p, err := h.editorial.CreateProducer(r.Context(), actorID, role, req.RegionID, req.ProducerType, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) createSpecies(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		ScientificName string `json:"scientific_name"`
		Slug           string `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	sp, err := h.editorial.CreateSpecies(r.Context(), actorID, role, req.ScientificName, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, sp)
}

func (h *Handler) createVariety(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		SpeciesID string `json:"species_id"`
		Slug      string `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	v, err := h.editorial.CreateVariety(r.Context(), actorID, role, req.SpeciesID, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *Handler) createProcessingMethod(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	pm, err := h.editorial.CreateProcessingMethod(r.Context(), actorID, role, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, pm)
}

func (h *Handler) createRoastLevel(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		Slug      string `json:"slug"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	rl, err := h.editorial.CreateRoastLevel(r.Context(), actorID, role, req.Slug, req.SortOrder, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, rl)
}

func (h *Handler) createFlavorDescriptor(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		ParentID *string `json:"parent_id"`
		Slug     string  `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	fd, err := h.editorial.CreateFlavorDescriptor(r.Context(), actorID, role, req.ParentID, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, fd)
}

func (h *Handler) createRevision(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	rev, err := h.editorial.CreateRevision(r.Context(), actorID, role, chi.URLParam(r, "articleID"), req.Title, req.Body, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, rev)
}

func (h *Handler) createSource(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		SourceType              string      `json:"source_type"`
		Title                   string      `json:"title"`
		CanonicalURL            string      `json:"canonical_url"`
		Publisher               string      `json:"publisher"`
		LicenseType             LicenseType `json:"license_type"`
		CommercialUseCompatible bool        `json:"commercial_use_compatible"`
		AttributionText         string      `json:"attribution_text"`
		Notes                   string      `json:"notes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	src, err := h.editorial.CreateSource(r.Context(), actorID, role, Source{
		SourceType:              req.SourceType,
		Title:                   req.Title,
		CanonicalURL:            req.CanonicalURL,
		Publisher:               req.Publisher,
		LicenseType:             req.LicenseType,
		CommercialUseCompatible: req.CommercialUseCompatible,
		AttributionText:         req.AttributionText,
		Notes:                   req.Notes,
	}, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

func (h *Handler) createClaim(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		SourceID    string `json:"source_id"`
		SubjectType string `json:"subject_type"`
		SubjectID   string `json:"subject_id"`
		ClaimText   string `json:"claim_text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	cl, err := h.editorial.CreateClaim(r.Context(), actorID, role, Claim{
		SourceID:    req.SourceID,
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
		ClaimText:   req.ClaimText,
	}, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, cl)
}

func (h *Handler) supersedeArticle(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		NewArticleID string `json:"new_article_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	art, err := h.editorial.Supersede(r.Context(), actorID, role, chi.URLParam(r, "articleID"), req.NewArticleID, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (h *Handler) createArticle(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	art, rev, err := h.editorial.CreateArticle(r.Context(), actorID, role, req.Slug, req.Title, req.Body, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"article": art, "revision": rev})
}

func (h *Handler) submitArticle(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	art, err := h.editorial.Submit(r.Context(), actorID, role, chi.URLParam(r, "articleID"), httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (h *Handler) reviewRevision(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	var req struct {
		Decision ReviewDecision `json:"decision"`
		Reason   string         `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	art, rev, err := h.editorial.Review(r.Context(), actorID, role, chi.URLParam(r, "articleID"), chi.URLParam(r, "revisionID"), req.Decision, req.Reason, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"article": art, "revision": rev})
}

func (h *Handler) publishArticle(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	art, err := h.editorial.Publish(r.Context(), actorID, role, chi.URLParam(r, "articleID"), chi.URLParam(r, "revisionID"), httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (h *Handler) emergencyUnpublish(w http.ResponseWriter, r *http.Request) {
	actorID, role, ok := h.actorAndRole(r)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	art, err := h.editorial.EmergencyUnpublish(r.Context(), actorID, role, chi.URLParam(r, "articleID"), httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeKnowledgeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func writeKnowledgeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrSelfApproval):
		httpx.WriteProblem(w, r, http.StatusForbidden, "Forbidden", "access denied")
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "Not Found", "resource not found")
	case errors.Is(err, ErrConflict), errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrLicenseBlocked), errors.Is(err, ErrAttributionMissing):
		httpx.WriteProblem(w, r, http.StatusConflict, "Conflict", err.Error())
	case errors.Is(err, ErrInvalidInput):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid input")
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
