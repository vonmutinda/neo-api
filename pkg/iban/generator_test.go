package iban

import (
	"strings"
	"testing"
)

func TestGenerate_Format(t *testing.T) {
	iban := Generate("1000000001", "ETB")

	if !strings.HasPrefix(iban, "ET") {
		t.Fatalf("expected IBAN to start with ET, got %s", iban)
	}
	if !strings.Contains(iban, "NEO0001") {
		t.Fatalf("expected IBAN to contain NEO0001, got %s", iban)
	}
	if !strings.HasSuffix(iban, "ETB") {
		t.Fatalf("expected IBAN to end with ETB, got %s", iban)
	}
	// ET (2) + check (2) + NEO0001 (7) + account (10) + currency (3) = 24
	if len(iban) != 24 {
		t.Fatalf("expected IBAN length 24, got %d: %s", len(iban), iban)
	}
}

func TestGenerate_ValidatesWithMOD97(t *testing.T) {
	cases := []struct {
		account  string
		currency string
	}{
		{"1000000001", "ETB"},
		{"1000000002", "USD"},
		{"1000000003", "EUR"},
		{"9999999999", "ETB"},
	}

	for _, tc := range cases {
		iban := Generate(tc.account, tc.currency)
		if !Validate(iban) {
			t.Errorf("Generated IBAN %s failed validation", iban)
		}
	}
}

func TestValidate_RejectsCorrupted(t *testing.T) {
	iban := Generate("1000000001", "ETB")
	corrupted := iban[:len(iban)-1] + "X"
	if Validate(corrupted) {
		t.Errorf("expected corrupted IBAN %s to fail validation", corrupted)
	}
}

func TestValidate_RejectsShort(t *testing.T) {
	if Validate("ET00") {
		t.Error("expected short IBAN to fail validation")
	}
}

func TestFormatAccountNumber(t *testing.T) {
	got := FormatAccountNumber(42)
	if got != "0000000042" {
		t.Errorf("expected 0000000042, got %s", got)
	}
	got = FormatAccountNumber(1000000001)
	if got != "1000000001" {
		t.Errorf("expected 1000000001, got %s", got)
	}
}

func TestGenerate_RoundTrip(t *testing.T) {
	// Generate IBAN then validate - round-trip ensures generated IBANs pass validation
	cases := []struct {
		account  string
		currency string
	}{
		{"1000000001", "ETB"},
		{"1000000002", "USD"},
		{"1000000003", "EUR"},
		{"9999999999", "ETB"},
		{"0000000042", "USD"},
		{"1234567890", "EUR"},
	}

	for _, tc := range cases {
		iban := Generate(tc.account, tc.currency)
		if !Validate(iban) {
			t.Errorf("round-trip failed: generated IBAN %q for account=%s currency=%s did not validate", iban, tc.account, tc.currency)
		}
	}
}
