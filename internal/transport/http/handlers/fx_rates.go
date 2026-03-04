package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/money"
)

// FXRateHandler serves the public (unauthenticated) FX rate endpoints.
type FXRateHandler struct {
	rates convert.RateProvider
}

func NewFXRateHandler(rates convert.RateProvider) *FXRateHandler {
	return &FXRateHandler{rates: rates}
}

// ListRates returns all current rates for every supported currency pair.
func (h *FXRateHandler) ListRates(w http.ResponseWriter, r *http.Request) {
	currencies := money.SupportedCurrencies

	type rateEntry struct {
		From      string  `json:"from"`
		To        string  `json:"to"`
		Mid       float64 `json:"mid"`
		Bid       float64 `json:"bid"`
		Ask       float64 `json:"ask"`
		Spread    float64 `json:"spread"`
		Source    string  `json:"source"`
		UpdatedAt string  `json:"updatedAt"`
	}

	type currencyInfo struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
		Flag   string `json:"flag"`
	}

	var rates []rateEntry
	for _, from := range currencies {
		for _, to := range currencies {
			if from.Code == to.Code {
				continue
			}
			rate, err := h.rates.GetRate(r.Context(), from.Code, to.Code)
			if err != nil {
				continue
			}
			rates = append(rates, rateEntry{
				From:      rate.From,
				To:        rate.To,
				Mid:       rate.Mid,
				Bid:       rate.Bid,
				Ask:       rate.Ask,
				Spread:    rate.Spread,
				Source:    rate.Source,
				UpdatedAt: rate.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	curInfos := make([]currencyInfo, len(currencies))
	for i, c := range currencies {
		curInfos[i] = currencyInfo{Code: c.Code, Name: c.Name, Symbol: c.Symbol, Flag: c.Flag}
	}

	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"rates":      rates,
		"currencies": curInfos,
	})
}

// GetRate returns a single pair rate detail.
func (h *FXRateHandler) GetRate(w http.ResponseWriter, r *http.Request) {
	from := strings.ToUpper(chi.URLParam(r, "from"))
	to := strings.ToUpper(chi.URLParam(r, "to"))

	if err := money.ValidateCurrency(from); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported currency: "+from)
		return
	}
	if err := money.ValidateCurrency(to); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported currency: "+to)
		return
	}

	rate, err := h.rates.GetRate(r.Context(), from, to)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	fromCur, _ := money.LookupCurrency(from)
	toCur, _ := money.LookupCurrency(to)

	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"from":       map[string]string{"code": fromCur.Code, "name": fromCur.Name, "symbol": fromCur.Symbol, "flag": fromCur.Flag},
		"to":         map[string]string{"code": toCur.Code, "name": toCur.Name, "symbol": toCur.Symbol, "flag": toCur.Flag},
		"mid":        rate.Mid,
		"bid":        rate.Bid,
		"ask":        rate.Ask,
		"spread":     rate.Spread,
		"inverseMid": 1.0 / rate.Mid,
		"source":     rate.Source,
		"updatedAt":  rate.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

// Convert is a read-only calculator endpoint. No money is moved.
func (h *FXRateHandler) Convert(w http.ResponseWriter, r *http.Request) {
	from := strings.ToUpper(r.URL.Query().Get("from"))
	to := strings.ToUpper(r.URL.Query().Get("to"))
	amountStr := r.URL.Query().Get("amount")

	if from == "" || to == "" || amountStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "from, to, and amount query params required")
		return
	}

	if err := money.ValidateCurrency(from); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported currency: "+from)
		return
	}
	if err := money.ValidateCurrency(to); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported currency: "+to)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "amount must be a positive number")
		return
	}

	rate, err := h.rates.GetRate(r.Context(), from, to)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	fromCents := int64(math.Round(amount * float64(money.CentsFactor)))
	toCentsAtAsk := int64(math.Round(float64(fromCents) * rate.Ask))
	toCentsAtMid := int64(math.Round(float64(fromCents) * rate.Mid))
	feeCents := toCentsAtMid - toCentsAtAsk

	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"from":          from,
		"to":            to,
		"fromAmount":    money.FormatMinorUnits(fromCents),
		"toAmount":      money.FormatMinorUnits(toCentsAtAsk),
		"mid":           rate.Mid,
		"effectiveRate": rate.Ask,
		"spread":        rate.Spread,
		"fee":           money.FormatMinorUnits(feeCents),
		"updatedAt":     rate.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
	})
}
