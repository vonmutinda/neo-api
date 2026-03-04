package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type FlagHandler struct {
	svc *adminsvc.FlagService
}

func NewFlagHandler(svc *adminsvc.FlagService) *FlagHandler {
	return &FlagHandler{svc: svc}
}

func (h *FlagHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := parseFlagFilter(r.URL.Query())
	result, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *FlagHandler) Create(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	var req adminsvc.CreateFlagRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	flag, err := h.svc.Create(r.Context(), staffID, req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, flag)
}

func (h *FlagHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	flagID := chi.URLParam(r, "id")
	var req adminsvc.ResolveFlagRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Resolve(r.Context(), staffID, flagID, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (h *FlagHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	flags, err := h.svc.ListByUser(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, flags)
}
