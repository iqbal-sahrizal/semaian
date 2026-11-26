package organization

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/semaian/platform/services/api/internal/platform/auth"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

// UserResolver resolves an internal user ID from verified OIDC claims.
type UserResolver interface {
	ResolveUserID(ctx context.Context, issuer, subject string) (string, error)
}

type Handler struct {
	svc      *Service
	resolver UserResolver
}

func NewHandler(svc *Service, resolver UserResolver) *Handler {
	return &Handler{svc: svc, resolver: resolver}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/organizations", h.list)
	r.Post("/organizations", h.create)
	r.Get("/organizations/{orgID}", h.get)
	r.Patch("/organizations/{orgID}", h.update)
	r.Get("/organizations/{orgID}/members", h.listMembers)
	r.Patch("/organizations/{orgID}/members/{userID}/role", h.changeRole)
	r.Delete("/organizations/{orgID}/members/{userID}", h.removeMember)
	r.Post("/organizations/{orgID}/transfer-ownership", h.transferOwnership)
}

func (h *Handler) resolveUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return "", false
	}
	uid, err := h.resolver.ResolveUserID(r.Context(), actor.Issuer, actor.Subject)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusForbidden, "Forbidden", "inactive or invalid identity")
		return "", false
	}
	return uid, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	orgs, err := h.svc.ListAccessible(r.Context(), uid)
	if err != nil {
		writeOrgError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": orgs})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	org, err := h.svc.Create(r.Context(), uid, req.Name, req.Slug, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeOrgError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	org, err := h.svc.Get(r.Context(), uid, chi.URLParam(r, "orgID"))
	if err != nil {
		writeOrgError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	org, err := h.svc.Update(r.Context(), uid, chi.URLParam(r, "orgID"), req.Name, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeOrgError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	members, err := h.svc.ListMembers(r.Context(), uid, chi.URLParam(r, "orgID"))
	if err != nil {
		writeOrgError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) changeRole(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Role Role `json:"role"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	mem, err := h.svc.ChangeMemberRole(r.Context(), uid, chi.URLParam(r, "orgID"), chi.URLParam(r, "userID"), req.Role, httpx.GetCorrelationID(r.Context()))
	if err != nil {
		writeOrgError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mem)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(r.Context(), uid, chi.URLParam(r, "orgID"), chi.URLParam(r, "userID"), httpx.GetCorrelationID(r.Context())); err != nil {
		writeOrgError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) transferOwnership(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	var req struct {
		NewOwnerUserID string `json:"new_owner_user_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	if err := h.svc.TransferOwnership(r.Context(), uid, chi.URLParam(r, "orgID"), req.NewOwnerUserID, httpx.GetCorrelationID(r.Context())); err != nil {
		writeOrgError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeOrgError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrOrgInactive):
		httpx.WriteProblem(w, r, http.StatusForbidden, "Forbidden", "access denied")
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "Not Found", "resource not found")
	case errors.Is(err, ErrLastOwner), errors.Is(err, ErrConflict):
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
