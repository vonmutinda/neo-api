package money

import (
	"fmt"
	"math"
	"strings"
)

// Currency represents a supported ISO 4217 currency with its metadata.
type Currency struct {
	Code              string // ISO 4217 code: "ETB", "USD", "EUR"
	Exponent          int    // Decimal places: 2 for all current currencies
	Symbol            string // Display symbol: "Br", "$", "€"
	Name              string // Full name: "Ethiopian Birr", "US Dollar", "Euro"
	Flag              string // Country flag for UI: "🇪🇹", "🇺🇸", "🇪🇺"
	HasAccountDetails bool   // Whether this currency gets IBAN/account details on activation
}

// Supported currencies.
var (
	ETB = Currency{Code: "ETB", Exponent: 2, Symbol: "Br", Name: "Ethiopian Birr", Flag: "ET", HasAccountDetails: true}
	USD = Currency{Code: "USD", Exponent: 2, Symbol: "$", Name: "US Dollar", Flag: "US", HasAccountDetails: true}
	EUR = Currency{Code: "EUR", Exponent: 2, Symbol: "€", Name: "Euro", Flag: "EU", HasAccountDetails: true}
)

// SupportedCurrencies is the canonical list of currencies the neobank supports.
// Used by validation and the account summary endpoint.
var SupportedCurrencies = []Currency{ETB, USD, EUR}

// registry maps ISO codes to Currency structs for O(1) lookup.
var registry = map[string]Currency{
	"ETB": ETB,
	"USD": USD,
	"EUR": EUR,
}

// LookupCurrency returns the Currency for the given ISO 4217 code.
// Returns ErrInvalidCurrency if the code is not supported.
func LookupCurrency(code string) (Currency, error) {
	c, ok := registry[strings.ToUpper(code)]
	if !ok {
		return Currency{}, fmt.Errorf("unsupported currency %q: %w", code, ErrInvalidCurrency)
	}
	return c, nil
}

// ValidateCurrency returns nil if the currency code is supported.
func ValidateCurrency(code string) error {
	_, err := LookupCurrency(code)
	return err
}

// --- Conversion helpers ---

const (
	CurrencyETB = "ETB"
	CurrencyUSD = "USD"
	CurrencyEUR = "EUR"

	// ETBExponent is kept for backward compatibility.
	ETBExponent = 2

	// CentsFactor converts whole units to minor units: 1 unit = 100 cents.
	// All supported currencies use exponent 2.
	CentsFactor int64 = 100
)

// FormatAsset returns the Formance-style asset string: "ETB/2", "USD/2", "EUR/2".
// If no currency is provided, defaults to ETB.
func FormatAsset(codes ...string) string {
	code := CurrencyETB
	if len(codes) > 0 && codes[0] != "" {
		code = strings.ToUpper(codes[0])
	}
	c, err := LookupCurrency(code)
	if err != nil {
		return fmt.Sprintf("%s/%d", code, 2)
	}
	return fmt.Sprintf("%s/%d", c.Code, c.Exponent)
}

// AllAssets returns the Formance asset strings for all supported currencies.
func AllAssets() []string {
	assets := make([]string, len(SupportedCurrencies))
	for i, c := range SupportedCurrencies {
		assets[i] = fmt.Sprintf("%s/%d", c.Code, c.Exponent)
	}
	return assets
}

// ToCents converts a floating-point amount to its integer minor-unit representation.
func ToCents(amount float64) int64 {
	return int64(math.Round(amount * float64(CentsFactor)))
}

// ETBToCents is an alias for ToCents, kept for backward compatibility.
func ETBToCents(etb float64) int64 { return ToCents(etb) }

// Display formats a minor-unit amount into a human-readable string with the
// currency symbol. Examples: "Br 1,500.75", "$ 250.00", "€ 100.50"
func Display(cents int64, code string) string {
	c, err := LookupCurrency(code)
	if err != nil {
		return FormatMinorUnits(cents, code)
	}
	return fmt.Sprintf("%s %s", c.Symbol, FormatMinorUnits(cents, code))
}

// FormatMinorUnits converts cents to a decimal string: 150075 -> "1500.75"
func FormatMinorUnits(cents int64, _ ...string) string {
	whole := cents / CentsFactor
	frac := cents % CentsFactor
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}

// CentsToETB is kept for backward compatibility. Use Display(cents, "ETB") instead.
func CentsToETB(cents int64) string {
	return FormatMinorUnits(cents)
}

// ValidateAmountCents ensures the amount is strictly positive.
func ValidateAmountCents(cents int64) error {
	if cents < 0 {
		return fmt.Errorf("amount must be non-negative, got %d: %w", cents, ErrNegativeAmount)
	}
	if cents == 0 {
		return fmt.Errorf("amount must be greater than zero: %w", ErrZeroAmount)
	}
	return nil
}

// Deprecated: DefaultRates exists only for unit tests that don't need real
// rates. No production code path should reference this map. All rate lookups
// must go through convert.RateProvider (backed by DatabaseRateProvider).
var DefaultRates = map[string]float64{
	"USD_ETB": 57.50,
	"EUR_ETB": 62.25,
	"ETB_USD": 1.0 / 57.50,
	"ETB_EUR": 1.0 / 62.25,
	"USD_EUR": 62.25 / 57.50,
	"EUR_USD": 57.50 / 62.25,
}

// ConvertAmount converts amountCents from one currency to another using the given rates.
// Returns the converted amount in cents, the rate used, and any error.
func ConvertAmount(fromCode, toCode string, amountCents int64, rates map[string]float64) (int64, float64, error) {
	if fromCode == toCode {
		return 0, 0, fmt.Errorf("cannot convert %s to itself: %w", fromCode, ErrInvalidCurrency)
	}
	if err := ValidateCurrency(fromCode); err != nil {
		return 0, 0, err
	}
	if err := ValidateCurrency(toCode); err != nil {
		return 0, 0, err
	}
	if err := ValidateAmountCents(amountCents); err != nil {
		return 0, 0, err
	}
	key := fromCode + "_" + toCode
	rate, ok := rates[key]
	if !ok || rate <= 0 {
		return 0, 0, fmt.Errorf("no exchange rate for %s -> %s: %w", fromCode, toCode, ErrInvalidCurrency)
	}
	converted := int64(math.Round(float64(amountCents) * rate))
	return converted, rate, nil
}

// --- Errors ---

var (
	ErrNegativeAmount  = fmt.Errorf("negative amount")
	ErrZeroAmount      = fmt.Errorf("zero amount")
	ErrInvalidCurrency = fmt.Errorf("invalid currency")
)

// --- Balance Summary (Wise-style) ---

// CurrencyBalance represents a single currency balance within a multi-currency wallet.
type CurrencyBalance struct {
	Currency     string `json:"currency"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	BalanceCents int64  `json:"balanceCents"`
	Display      string `json:"display"`
}

// AccountSummary is the Wise-style multi-currency account overview.
type AccountSummary struct {
	WalletID            string            `json:"walletId"`
	PrimaryCurrency     string            `json:"primaryCurrency"`
	Balances            []CurrencyBalance `json:"balances"`
	TotalInPrimaryCents int64             `json:"totalInPrimaryCents"`
	TotalDisplay        string            `json:"totalDisplay"`
}

// BuildAccountSummary constructs a Wise-style summary from a map of currency -> cents.
// The primary currency is the user's home currency (ETB).
// Exchange rates are applied to compute the total in the primary currency.
// If currencies is nil, all SupportedCurrencies are used (backward-compatible).
func BuildAccountSummary(walletID string, balances map[string]int64, rates map[string]float64, currencies ...[]Currency) AccountSummary {
	summary := AccountSummary{
		WalletID:        walletID,
		PrimaryCurrency: CurrencyETB,
	}

	curList := SupportedCurrencies
	if len(currencies) > 0 && currencies[0] != nil {
		curList = currencies[0]
	}

	var totalETBCents int64

	for _, cur := range curList {
		cents := balances[cur.Code]
		cb := CurrencyBalance{
			Currency:     cur.Code,
			Symbol:       cur.Symbol,
			Name:         cur.Name,
			BalanceCents: cents,
			Display:      Display(cents, cur.Code),
		}
		summary.Balances = append(summary.Balances, cb)

		if cur.Code == CurrencyETB {
			totalETBCents += cents
		} else if rate, ok := rates[cur.Code]; ok && rate > 0 {
			totalETBCents += int64(math.Round(float64(cents) * rate))
		}
	}

	summary.TotalInPrimaryCents = totalETBCents
	summary.TotalDisplay = Display(totalETBCents, CurrencyETB)
	return summary
}
