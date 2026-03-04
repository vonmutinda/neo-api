package personal

import (
	"net/http"
	"strings"

	"github.com/vonmutinda/neo/internal/services/wallet"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type WalletHandler struct {
	svc *wallet.Service
}

func NewWalletHandler(svc *wallet.Service) *WalletHandler {
	return &WalletHandler{svc: svc}
}

func (h *WalletHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	currencyCode := r.URL.Query().Get("currency")

	view, err := h.svc.GetBalance(r.Context(), userID, currencyCode)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, view)
}

func (h *WalletHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	summary, err := h.svc.GetSummary(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, summary)
}

func (h *WalletHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	pg := httputil.ParsePagination(r)
	currencyFilter := strings.ToUpper(r.URL.Query().Get("currency"))

	var currency *string
	if currencyFilter != "" {
		currency = &currencyFilter
	}

	views, err := h.svc.ListTransactions(r.Context(), userID, currency, pg.Limit, pg.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if views == nil {
		views = []wallet.TransactionView{}
	}

	httputil.WriteJSON(w, http.StatusOK, views)
}
