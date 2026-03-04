package personal

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/services/balances"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type BalanceHandler struct{ svc *balances.Service }

func NewBalanceHandler(svc *balances.Service) *BalanceHandler {
	return &BalanceHandler{svc: svc}
}

func (h *BalanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req balances.CreateBalanceRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	req.CurrencyCode = strings.ToUpper(strings.TrimSpace(req.CurrencyCode))

	result, err := h.svc.CreateCurrencyBalance(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result)
}

func (h *BalanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	code := strings.ToUpper(chi.URLParam(r, "code"))

	if err := h.svc.DeleteCurrencyBalance(r.Context(), userID, code); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *BalanceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	list, err := h.svc.ListActiveCurrencyBalances(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, list)
}
