package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type FXRateHandler struct {
	repo    repository.FXRateRepository
	audit   repository.AuditRepository
	refresh *convert.RateRefreshService
	spread  float64
	cache   cache.Cache
}

func NewFXRateHandler(
	repo repository.FXRateRepository,
	audit repository.AuditRepository,
	refresh *convert.RateRefreshService,
	spreadPercent float64,
	c cache.Cache,
) *FXRateHandler {
	return &FXRateHandler{repo: repo, audit: audit, refresh: refresh, spread: spreadPercent, cache: c}
}

// ListCurrent returns the latest rate for every currency pair.
func (h *FXRateHandler) ListCurrent(w http.ResponseWriter, r *http.Request) {
	rates, err := h.repo.GetLatestAll(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if rates == nil {
		rates = []domain.FXRate{}
	}
	httputil.WriteJSON(w, http.StatusOK, rates)
}

// ListHistory returns historical rates for a currency pair.
func (h *FXRateHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httputil.WriteError(w, http.StatusBadRequest, "from and to query params required")
		return
	}

	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-30 * 24 * time.Hour)
	if sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	rates, err := h.repo.ListHistory(r.Context(), from, to, since, limit)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if rates == nil {
		rates = []domain.FXRate{}
	}
	httputil.WriteJSON(w, http.StatusOK, rates)
}

// Override allows an admin to manually set a rate for a currency pair.
func (h *FXRateHandler) Override(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromCurrency string  `json:"fromCurrency"`
		ToCurrency   string  `json:"toCurrency"`
		MidRate      float64 `json:"midRate"`
		Reason       string  `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if req.FromCurrency == "" || req.ToCurrency == "" || req.MidRate <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "fromCurrency, toCurrency, and positive midRate required")
		return
	}

	spreadFactor := h.spread / 100.0
	rate := &domain.FXRate{
		FromCurrency:  req.FromCurrency,
		ToCurrency:    req.ToCurrency,
		MidRate:       req.MidRate,
		BidRate:       req.MidRate * (1 - spreadFactor/2),
		AskRate:       req.MidRate * (1 + spreadFactor/2),
		SpreadPercent: h.spread,
		Source:        domain.FXRateSourceManual,
		FetchedAt:     time.Now(),
	}

	if err := h.repo.Insert(r.Context(), rate); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	staffID := middleware.UserIDFromContext(r.Context())
	meta, _ := json.Marshal(map[string]any{
		"fromCurrency": req.FromCurrency,
		"toCurrency":   req.ToCurrency,
		"midRate":      req.MidRate,
		"reason":       req.Reason,
	})
	_ = h.audit.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditFXRateManualOverride,
		ActorType:    "staff",
		ActorID:      &staffID,
		ResourceType: "fx_rates",
		ResourceID:   rate.ID,
		Metadata:     meta,
	})

	_ = h.cache.DeleteByPrefix(r.Context(), "neo:fx:")
	httputil.WriteJSON(w, http.StatusCreated, rate)
}

func (h *FXRateHandler) TriggerRefresh(w http.ResponseWriter, r *http.Request) {
	if err := h.refresh.Refresh(r.Context()); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	_ = h.cache.DeleteByPrefix(r.Context(), "neo:fx:")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "refreshed"})
}
