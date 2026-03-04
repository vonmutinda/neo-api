package personal

import (
	"net/http"
	"strconv"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/services/pricing"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type FeeHandler struct {
	pricing *pricing.Service
}

func NewFeeHandler(pricing *pricing.Service) *FeeHandler {
	return &FeeHandler{pricing: pricing}
}

func (h *FeeHandler) GetQuote(w http.ResponseWriter, r *http.Request) {
	txType := r.URL.Query().Get("type")
	amountStr := r.URL.Query().Get("amountCents")
	currency := r.URL.Query().Get("currency")
	channel := r.URL.Query().Get("channel")

	if txType == "" || amountStr == "" || currency == "" {
		httputil.WriteError(w, http.StatusBadRequest, "type, amountCents, and currency are required")
		return
	}

	amountCents, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil || amountCents <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "amountCents must be a positive integer")
		return
	}

	var ch *string
	if channel != "" {
		ch = &channel
	}

	fee, err := h.pricing.CalculateFee(r.Context(), domain.TransactionType(txType), amountCents, currency, ch)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"ourFee":     fee.OurFeeCents,
		"partnerFee": fee.PartnerFeeCents,
		"totalFee":   fee.TotalFeeCents,
		"totalDebit": amountCents + fee.TotalFeeCents,
		"details":    fee.Details,
	})
}

func (h *FeeHandler) GetInternationalQuote(w http.ResponseWriter, r *http.Request) {
	fromCurrency := r.URL.Query().Get("fromCurrency")
	toCurrency := r.URL.Query().Get("toCurrency")
	amountStr := r.URL.Query().Get("amountCents")

	if fromCurrency == "" || toCurrency == "" || amountStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "fromCurrency, toCurrency, and amountCents are required")
		return
	}

	amountCents, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil || amountCents <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "amountCents must be a positive integer")
		return
	}

	quote, err := h.pricing.GetQuote(r.Context(), fromCurrency, toCurrency, amountCents)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	resp := map[string]any{
		"provider":         quote.ProviderName,
		"quoteId":          quote.QuoteID,
		"sourceCurrency":   quote.SourceCurrency,
		"targetCurrency":   quote.TargetCurrency,
		"sourceAmount":     quote.SourceAmount,
		"targetAmount":     quote.TargetAmount,
		"exchangeRate":     quote.ExchangeRate,
		"deliveryEstimate": quote.DeliveryEstimate,
		"expiresAt":        quote.ExpiresAt,
	}
	if quote.Fee != nil {
		resp["ourFee"] = quote.Fee.OurFeeCents
		resp["partnerFee"] = quote.Fee.PartnerFeeCents
		resp["totalFee"] = quote.Fee.TotalFeeCents
		resp["totalDebit"] = quote.SourceAmount + quote.Fee.TotalFeeCents
		resp["details"] = quote.Fee.Details
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}
