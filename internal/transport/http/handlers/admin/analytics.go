package admin

import (
	"net/http"
	"strconv"
	"time"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type AnalyticsHandler struct {
	svc *adminsvc.AnalyticsService
}

func NewAnalyticsHandler(svc *adminsvc.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

func (h *AnalyticsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.Overview(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, overview)
}

const dateOnly = "2006-01-02"

func (h *AnalyticsHandler) MoneyFlowMap(w http.ResponseWriter, r *http.Request) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(dateOnly, v); err == nil {
			to = t.UTC()
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t.UTC()
		}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(dateOnly, v); err == nil {
			from = t.UTC()
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t.UTC()
		}
	}
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	resp, err := h.svc.MoneyFlowMap(r.Context(), from, to, limit)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}
