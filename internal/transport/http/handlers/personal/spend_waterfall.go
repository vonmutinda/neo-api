package personal

import (
	"fmt"
	"net/http"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/money"
)

type SpendWaterfallHandler struct {
	users    repository.UserRepository
	balances repository.CurrencyBalanceRepository
}

func NewSpendWaterfallHandler(users repository.UserRepository, balances repository.CurrencyBalanceRepository) *SpendWaterfallHandler {
	return &SpendWaterfallHandler{users: users, balances: balances}
}

func (h *SpendWaterfallHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	waterfall := user.SpendWaterfallOrder
	isDefault := len(waterfall) == 0
	if isDefault {
		waterfall = domain.DefaultSpendWaterfall
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"waterfall": waterfall,
		"isDefault": isDefault,
	})
}

func (h *SpendWaterfallHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Waterfall domain.SpendWaterfall `json:"waterfall"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if len(req.Waterfall) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "waterfall must not be empty")
		return
	}

	if req.Waterfall[len(req.Waterfall)-1] != money.CurrencyETB {
		httputil.WriteError(w, http.StatusBadRequest, "ETB must be the last entry in the waterfall")
		return
	}

	activeBalances, _ := h.balances.ListActiveByUser(r.Context(), userID)
	activeCurrencies := make(map[string]bool, len(activeBalances))
	for _, b := range activeBalances {
		activeCurrencies[b.CurrencyCode] = true
	}

	for _, entry := range req.Waterfall {
		if entry == "merchant_currency" {
			continue
		}
		if _, err := money.LookupCurrency(entry); err != nil {
			httputil.HandleError(w, r, fmt.Errorf("invalid currency %q in waterfall: %w", entry, domain.ErrInvalidInput))
			return
		}
	}

	if err := h.users.UpdateSpendWaterfall(r.Context(), userID, req.Waterfall); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"waterfall": req.Waterfall,
		"isDefault": false,
	})
}
