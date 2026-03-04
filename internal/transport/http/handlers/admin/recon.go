package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type ReconHandler struct {
	svc *adminsvc.ReconService
}

func NewReconHandler(svc *adminsvc.ReconService) *ReconHandler {
	return &ReconHandler{svc: svc}
}

func (h *ReconHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	result, err := h.svc.ListRuns(r.Context(), p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *ReconHandler) ListExceptions(w http.ResponseWriter, r *http.Request) {
	filter := parseExceptionFilter(r.URL.Query())
	result, err := h.svc.ListExceptions(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *ReconHandler) Assign(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	excID := chi.URLParam(r, "id")
	var req struct {
		AssignedTo string `json:"assignedTo"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Assign(r.Context(), staffID, excID, req.AssignedTo); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (h *ReconHandler) Investigate(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	excID := chi.URLParam(r, "id")
	if err := h.svc.Investigate(r.Context(), staffID, excID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "investigating"})
}

func (h *ReconHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	excID := chi.URLParam(r, "id")
	var req adminsvc.ResolveExceptionRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Resolve(r.Context(), staffID, excID, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (h *ReconHandler) Escalate(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	excID := chi.URLParam(r, "id")
	var req struct {
		Notes string `json:"notes"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Escalate(r.Context(), staffID, excID, req.Notes); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "escalated"})
}
