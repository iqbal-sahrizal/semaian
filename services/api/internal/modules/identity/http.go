package identity

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/semaian/platform/services/api/internal/platform/auth"
	"github.com/semaian/platform/services/api/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.getMe)
	r.Get("/me/profile", h.getProfile)
	r.Patch("/me/profile", h.updateProfile)
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	u, err := h.svc.ResolveExternalIdentity(r.Context(), actor.Issuer, actor.Subject)
	if err != nil {
		writeIdentityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	u, err := h.svc.ResolveExternalIdentity(r.Context(), actor.Issuer, actor.Subject)
	if err != nil {
		writeIdentityError(w, r, err)
		return
	}
	p, err := h.svc.GetProfile(r.Context(), u.ID)
	if err != nil {
		writeIdentityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing actor")
		return
	}
	u, err := h.svc.ResolveExternalIdentity(r.Context(), actor.Issuer, actor.Subject)
	if err != nil {
		writeIdentityError(w, r, err)
		return
	}
	var upd ProfileUpdate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&upd); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON payload")
		return
	}
	p, err := h.svc.UpdateProfile(r.Context(), u.ID, upd)
	if err != nil {
		writeIdentityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func writeIdentityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInactiveIdentity):
		httpx.WriteProblem(w, r, http.StatusForbidden, "Forbidden", "identity is suspended or deactivated")
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "Not Found", "identity not found")
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
