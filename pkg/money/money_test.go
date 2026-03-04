package money_test

import (
	"testing"

	"github.com/vonmutinda/neo/pkg/money"
)

func TestETBToCents(t *testing.T) {
	tests := []struct {
		input float64
		want  int64
	}{
		{100.00, 10000},
		{1500.75, 150075},
		{0.01, 1},
		{0, 0},
		{99999.99, 9999999},
	}
	for _, tt := range tests {
		got := money.ETBToCents(tt.input)
		if got != tt.want {
			t.Errorf("ETBToCents(%f) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCentsToETB(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{10000, "100.00"},
		{150075, "1500.75"},
		{1, "0.01"},
		{0, "0.00"},
		{9999999, "99999.99"},
	}
	for _, tt := range tests {
		got := money.CentsToETB(tt.input)
		if got != tt.want {
			t.Errorf("CentsToETB(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateAmountCents(t *testing.T) {
	if err := money.ValidateAmountCents(100); err != nil {
		t.Errorf("expected no error for positive amount, got %v", err)
	}
	if err := money.ValidateAmountCents(0); err == nil {
		t.Error("expected error for zero amount")
	}
	if err := money.ValidateAmountCents(-50); err == nil {
		t.Error("expected error for negative amount")
	}
}

func TestFormatAsset(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, "ETB/2"},
		{[]string{}, "ETB/2"},
		{[]string{""}, "ETB/2"},
		{[]string{"ETB"}, "ETB/2"},
		{[]string{"USD"}, "USD/2"},
		{[]string{"EUR"}, "EUR/2"},
		{[]string{"usd"}, "USD/2"},
	}
	for _, tt := range tests {
		got := money.FormatAsset(tt.args...)
		if got != tt.want {
			t.Errorf("FormatAsset(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestAllAssets(t *testing.T) {
	assets := money.AllAssets()
	if len(assets) != 3 {
		t.Fatalf("expected 3 assets, got %d", len(assets))
	}
	expected := []string{"ETB/2", "USD/2", "EUR/2"}
	for i, want := range expected {
		if assets[i] != want {
			t.Errorf("AllAssets()[%d] = %q, want %q", i, assets[i], want)
		}
	}
}

func TestLookupCurrency(t *testing.T) {
	// Valid currencies
	for _, code := range []string{"ETB", "USD", "EUR", "etb", "usd", "eur"} {
		c, err := money.LookupCurrency(code)
		if err != nil {
			t.Errorf("LookupCurrency(%q) returned error: %v", code, err)
		}
		if c.Code == "" {
			t.Errorf("LookupCurrency(%q) returned empty code", code)
		}
	}

	// Invalid currency
	_, err := money.LookupCurrency("GBP")
	if err == nil {
		t.Error("expected error for unsupported currency GBP")
	}
}

func TestValidateCurrency(t *testing.T) {
	if err := money.ValidateCurrency("ETB"); err != nil {
		t.Errorf("expected no error for ETB, got %v", err)
	}
	if err := money.ValidateCurrency("XYZ"); err == nil {
		t.Error("expected error for unsupported currency XYZ")
	}
}

func TestDisplay(t *testing.T) {
	tests := []struct {
		cents int64
		code  string
		want  string
	}{
		{150075, "ETB", "Br 1500.75"},
		{25000, "USD", "$ 250.00"},
		{10050, "EUR", "€ 100.50"},
		{0, "ETB", "Br 0.00"},
	}
	for _, tt := range tests {
		got := money.Display(tt.cents, tt.code)
		if got != tt.want {
			t.Errorf("Display(%d, %q) = %q, want %q", tt.cents, tt.code, got, tt.want)
		}
	}
}

func TestBuildAccountSummary(t *testing.T) {
	balances := map[string]int64{
		"ETB": 1000000, // 10,000.00 ETB
		"USD": 10000,   // 100.00 USD
		"EUR": 5000,    // 50.00 EUR
	}
	rates := map[string]float64{
		"USD": 57.50,
		"EUR": 62.30,
	}

	summary := money.BuildAccountSummary("wallet-1", balances, rates)

	if summary.WalletID != "wallet-1" {
		t.Errorf("expected walletId wallet-1, got %s", summary.WalletID)
	}
	if summary.PrimaryCurrency != "ETB" {
		t.Errorf("expected primary currency ETB, got %s", summary.PrimaryCurrency)
	}
	if len(summary.Balances) != 3 {
		t.Fatalf("expected 3 balances, got %d", len(summary.Balances))
	}

	// Verify ETB balance
	if summary.Balances[0].Currency != "ETB" {
		t.Errorf("expected first balance ETB, got %s", summary.Balances[0].Currency)
	}
	if summary.Balances[0].BalanceCents != 1000000 {
		t.Errorf("expected ETB balance 1000000, got %d", summary.Balances[0].BalanceCents)
	}

	// Total should be ETB + (USD * 57.50) + (EUR * 62.30)
	// = 1000000 + (10000 * 57.50) + (5000 * 62.30)
	// = 1000000 + 575000 + 311500 = 1886500
	expectedTotal := int64(1886500)
	if summary.TotalInPrimaryCents != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, summary.TotalInPrimaryCents)
	}
}

// --- Edge case tests ---

func TestETBToCents_RoundingEdgeCases(t *testing.T) {
	tests := []struct {
		input float64
		want  int64
	}{
		{0.005, 1},   // round half up
		{0.004, 0},   // round down
		{0.006, 1},   // round up
		{0.015, 2},   // round half up
		{0.025, 3},   // round half up
		{0.999, 100}, // round up
	}
	for _, tt := range tests {
		got := money.ETBToCents(tt.input)
		if got != tt.want {
			t.Errorf("ETBToCents(%f) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFormatMinorUnits_NegativeCents(t *testing.T) {
	// FormatMinorUnits handles negative frac by taking abs
	got := money.FormatMinorUnits(-150075)
	if got != "-1500.75" {
		t.Errorf("FormatMinorUnits(-150075) = %q, want %q", got, "-1500.75")
	}
}

func TestFormatMinorUnits_Zero(t *testing.T) {
	got := money.FormatMinorUnits(0)
	if got != "0.00" {
		t.Errorf("FormatMinorUnits(0) = %q, want %q", got, "0.00")
	}
}

func TestFormatMinorUnits_SingleCent(t *testing.T) {
	got := money.FormatMinorUnits(1)
	if got != "0.01" {
		t.Errorf("FormatMinorUnits(1) = %q, want %q", got, "0.01")
	}
}

func TestDisplay_UnknownCurrency(t *testing.T) {
	// Display with unknown currency falls back to FormatMinorUnits without symbol
	got := money.Display(150075, "XYZ")
	if got != "1500.75" {
		t.Errorf("Display(150075, XYZ) = %q, want %q", got, "1500.75")
	}
}

func TestConvertAmount_EdgeCases(t *testing.T) {
	rates := money.DefaultRates

	// Same currency
	_, _, err := money.ConvertAmount("ETB", "ETB", 10000, rates)
	if err == nil {
		t.Error("expected error when converting same currency")
	}

	// Invalid from currency
	_, _, err = money.ConvertAmount("GBP", "ETB", 10000, rates)
	if err == nil {
		t.Error("expected error for invalid from currency")
	}

	// Invalid to currency
	_, _, err = money.ConvertAmount("ETB", "GBP", 10000, rates)
	if err == nil {
		t.Error("expected error for invalid to currency")
	}

	// Zero amount
	_, _, err = money.ConvertAmount("ETB", "USD", 0, rates)
	if err == nil {
		t.Error("expected error for zero amount")
	}

	// Negative amount
	_, _, err = money.ConvertAmount("ETB", "USD", -100, rates)
	if err == nil {
		t.Error("expected error for negative amount")
	}

	// Missing rate
	_, _, err = money.ConvertAmount("ETB", "USD", 10000, map[string]float64{})
	if err == nil {
		t.Error("expected error when rate is missing")
	}

	// Valid conversion: 10000 ETB cents -> USD at ETB_USD rate
	converted, rate, err := money.ConvertAmount("ETB", "USD", 10000, rates)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// ETB_USD = 1/57.50, so 10000 * (1/57.50) ≈ 173.91 -> 174 cents
	if rate <= 0 {
		t.Errorf("rate = %f, want positive", rate)
	}
	if converted <= 0 {
		t.Errorf("converted = %d, want positive", converted)
	}
}
