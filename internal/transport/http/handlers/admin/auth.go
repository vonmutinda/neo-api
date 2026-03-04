package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type AuthHandler struct {
	authSvc  *adminsvc.AuthService
	staffSvc *adminsvc.StaffService
}

func NewAuthHandler(authSvc *adminsvc.AuthService, staffSvc *adminsvc.StaffService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, staffSvc: staffSvc}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req adminsvc.LoginRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	resp, err := h.authSvc.Login(r.Context(), req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	var req adminsvc.ChangePasswordRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.authSvc.ChangePassword(r.Context(), staffID, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	staff, err := h.staffSvc.GetByID(r.Context(), staffID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, staff)
}

func (h *AuthHandler) ListStaff(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := parseStaffFilter(q)
	result, err := h.staffSvc.List(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *AuthHandler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	var req adminsvc.CreateStaffRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	staff, err := h.authSvc.CreateStaff(r.Context(), staffID, req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, staff)
}

func (h *AuthHandler) GetStaff(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	staff, err := h.staffSvc.GetByID(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, staff)
}

func (h *AuthHandler) UpdateStaff(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.StaffIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req adminsvc.UpdateStaffRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	staff, err := h.staffSvc.Update(r.Context(), actorID, id, req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, staff)
}

func (h *AuthHandler) DeactivateStaff(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.StaffIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.staffSvc.Deactivate(r.Context(), actorID, id); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
