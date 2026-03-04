package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type CardHandler struct {
	svc *adminsvc.CardService
}

func NewCardHandler(svc *adminsvc.CardService) *CardHandler {
	return &CardHandler{svc: svc}
}

func (h *CardHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := parseCardFilter(r.URL.Query())
	result, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *CardHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	card, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, card)
}

func (h *CardHandler) ListAuthorizations(w http.ResponseWriter, r *http.Request) {
	filter := parseCardAuthFilter(r.URL.Query())
	cardID := chi.URLParam(r, "id")
	filter.CardID = &cardID
	result, err := h.svc.ListAuthorizations(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *CardHandler) Freeze(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	cardID := chi.URLParam(r, "id")
	if err := h.svc.Freeze(r.Context(), staffID, cardID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "frozen"})
}

func (h *CardHandler) Unfreeze(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	cardID := chi.URLParam(r, "id")
	if err := h.svc.Unfreeze(r.Context(), staffID, cardID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "unfrozen"})
}

func (h *CardHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	cardID := chi.URLParam(r, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Cancel(r.Context(), staffID, cardID, req.Reason); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *CardHandler) UpdateLimits(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	cardID := chi.URLParam(r, "id")
	var req struct {
		DailyLimitCents   int64 `json:"dailyLimitCents"`
		MonthlyLimitCents int64 `json:"monthlyLimitCents"`
		PerTxnLimitCents  int64 `json:"perTxnLimitCents"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.UpdateLimits(r.Context(), staffID, cardID, req.DailyLimitCents, req.MonthlyLimitCents, req.PerTxnLimitCents); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "limits_updated"})
}
