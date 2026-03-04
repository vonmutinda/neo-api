package personal

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type ConvertHandler struct {
	svc *convert.Service
}

func NewConvertHandler(svc *convert.Service) *ConvertHandler {
	return &ConvertHandler{svc: svc}
}

func (h *ConvertHandler) Convert(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req domain.ConvertRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	resp, err := h.svc.Convert(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *ConvertHandler) GetRate(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		httputil.WriteError(w, http.StatusBadRequest, "from and to query params required")
		return
	}

	rate, err := h.svc.GetRate(r.Context(), from, to)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"from":      rate.From,
		"to":        rate.To,
		"mid":       rate.Mid,
		"bid":       rate.Bid,
		"ask":       rate.Ask,
		"spread":    rate.Spread,
		"timestamp": rate.Timestamp,
		"source":    rate.Source,
	})
}
