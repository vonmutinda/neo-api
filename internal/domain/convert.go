package domain

type ConvertRequest struct {
	FromCurrency string `json:"fromCurrency"`
	ToCurrency   string `json:"toCurrency"`
	AmountCents  int64  `json:"amountCents"`
}

type ConvertResponse struct {
	FromCurrency    string   `json:"fromCurrency"`
	ToCurrency      string   `json:"toCurrency"`
	FromAmountCents int64    `json:"fromAmountCents"`
	ToAmountCents   int64    `json:"toAmountCents"`
	Rate            float64  `json:"rate"`
	Spread          *float64 `json:"spread,omitempty"`
	EffectiveRate   *float64 `json:"effectiveRate,omitempty"`
	TransactionID   string   `json:"transactionId"`
}
