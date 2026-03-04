package personal

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type CardHandler struct {
	cards repository.CardRepository
}

func NewCardHandler(cards repository.CardRepository) *CardHandler {
	return &CardHandler{cards: cards}
}

func (h *CardHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Type               domain.CardType `json:"type"`
		DailyLimitCents    int64           `json:"dailyLimitCents"`
		MonthlyLimitCents  int64           `json:"monthlyLimitCents"`
		PerTxnLimitCents   int64           `json:"perTxnLimitCents"`
		AllowOnline        bool            `json:"allowOnline"`
		AllowContactless   bool            `json:"allowContactless"`
		AllowATM           bool            `json:"allowAtm"`
		AllowInternational bool            `json:"allowInternational"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	cardType := req.Type
	if cardType == "" {
		cardType = domain.CardTypeVirtual
	}
	if cardType != domain.CardTypeVirtual && cardType != domain.CardTypePhysical && cardType != domain.CardTypeEphemeral {
		httputil.WriteError(w, http.StatusBadRequest, "type must be virtual, physical, or ephemeral")
		return
	}

	// Apply defaults for any zero-value limits.
	if req.DailyLimitCents == 0 {
		req.DailyLimitCents = 500_000 // 5,000 ETB
	}
	if req.MonthlyLimitCents == 0 {
		req.MonthlyLimitCents = 5_000_000 // 50,000 ETB
	}
	if req.PerTxnLimitCents == 0 {
		req.PerTxnLimitCents = 200_000 // 2,000 ETB
	}

	now := time.Now()
	lastFour := fmt.Sprintf("%04d", rand.IntN(10000))
	token := fmt.Sprintf("tok_%s_%s_%d", cardType, lastFour, now.UnixNano())

	card := &domain.Card{
		UserID:             userID,
		TokenizedPAN:       token,
		LastFour:           lastFour,
		ExpiryMonth:        int(now.Month()),
		ExpiryYear:         now.Year() + 3,
		Type:               cardType,
		Status:             domain.CardStatusActive,
		AllowOnline:        req.AllowOnline,
		AllowContactless:   req.AllowContactless,
		AllowATM:           req.AllowATM,
		AllowInternational: req.AllowInternational,
		DailyLimitCents:    req.DailyLimitCents,
		MonthlyLimitCents:  req.MonthlyLimitCents,
		PerTxnLimitCents:   req.PerTxnLimitCents,
	}

	if err := h.cards.Create(r.Context(), card); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, card)
}

func (h *CardHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	cards, err := h.cards.ListByUserID(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, cards)
}

func (h *CardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "id")
	card, err := h.cards.GetByID(r.Context(), cardID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if card.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "card does not belong to you")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, card)
}

func (h *CardHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "id")
	var req struct {
		Status domain.CardStatus `json:"status"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	card, err := h.cards.GetByID(r.Context(), cardID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if card.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "card does not belong to you")
		return
	}

	if err := h.cards.UpdateStatus(r.Context(), cardID, req.Status); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": string(req.Status)})
}

func (h *CardHandler) UpdateLimits(w http.ResponseWriter, r *http.Request) {
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

	card, err := h.cards.GetByID(r.Context(), cardID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if card.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "card does not belong to you")
		return
	}

	if err := h.cards.UpdateLimits(r.Context(), cardID, req.DailyLimitCents, req.MonthlyLimitCents, req.PerTxnLimitCents); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *CardHandler) UpdateToggles(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "id")
	var req struct {
		AllowOnline        bool `json:"allowOnline"`
		AllowContactless   bool `json:"allowContactless"`
		AllowATM           bool `json:"allowAtm"`
		AllowInternational bool `json:"allowInternational"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	card, err := h.cards.GetByID(r.Context(), cardID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if card.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "card does not belong to you")
		return
	}

	if err := h.cards.UpdateToggles(r.Context(), cardID, req.AllowOnline, req.AllowContactless, req.AllowATM, req.AllowInternational); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
