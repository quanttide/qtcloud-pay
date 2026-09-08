package security

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
)

type Handler struct {
	store    *Store
	verifier *TokenVerifier
}

func NewHandler(store *Store, verifier *TokenVerifier) *Handler {
	return &Handler{store: store, verifier: verifier}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/security/permissions", h.handlePermissions)
	mux.HandleFunc("GET /admin/security/users/{user_id}/roles", h.handleUserRoles)
	mux.HandleFunc("PUT /admin/security/users/{user_id}/roles", h.handleSetUserRoles)
	mux.HandleFunc("POST /admin/security/service-tokens", h.handleServiceToken)
}

func (h *Handler) handlePermissions(w http.ResponseWriter, r *http.Request) {
	roles, err := h.store.ListRoles(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "list roles failed")
		return
	}
	perms, err := h.store.ListRolePermissions(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "list permissions failed")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"roles": roles, "permissions": perms})
}

func (h *Handler) handleUserRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.store.RolesForUser(r.Context(), r.PathValue("user_id"))
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (h *Handler) handleSetUserRoles(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("user_id"))
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.SetUserRoles(r.Context(), userID, req.Roles); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid roles")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"user_id": userID, "roles": req.Roles})
}

func (h *Handler) handleServiceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subject string `json:"subject"`
		TTL     int64  `json:"ttl_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TTL <= 0 || req.TTL > int64((24*time.Hour).Seconds()) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid ttl")
		return
	}
	token, err := h.verifier.SignServiceToken(req.Subject, time.Duration(req.TTL)*time.Second)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid subject")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"token": token, "token_type": "Bearer", "expires_in": req.TTL})
}
