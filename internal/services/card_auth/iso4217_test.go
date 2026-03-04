package cardauth_test

import (
	"testing"

	cardauth "github.com/vonmutinda/neo/internal/services/card_auth"
	"github.com/vonmutinda/neo/pkg/money"
)

func TestStaticISO4217Resolver_KnownCodes(t *testing.T) {
	r := cardauth.StaticISO4217Resolver(cardauth.DefaultISO4217Map)

	tests := []struct {
		code string
		want string
	}{
		{"840", "USD"},
		{"978", "EUR"},
		{"230", "ETB"},
	}

	for _, tt := range tests {
		got := r.Resolve(tt.code)
		if got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestStaticISO4217Resolver_UnknownCode_FallsBackToETB(t *testing.T) {
	r := cardauth.StaticISO4217Resolver{"840": "USD"}

	got := r.Resolve("999")
	if got != money.CurrencyETB {
		t.Errorf("Resolve(999) = %q, want %q", got, money.CurrencyETB)
	}
}

func TestStaticISO4217Resolver_EmptyMap(t *testing.T) {
	r := cardauth.StaticISO4217Resolver{}

	got := r.Resolve("840")
	if got != money.CurrencyETB {
		t.Errorf("Resolve(840) on empty map = %q, want %q", got, money.CurrencyETB)
	}
}

func TestStaticISO4217Resolver_EmptyString(t *testing.T) {
	r := cardauth.StaticISO4217Resolver{"840": "USD"}

	got := r.Resolve("")
	if got != money.CurrencyETB {
		t.Errorf("Resolve(\"\") = %q, want %q", got, money.CurrencyETB)
	}
}

func TestDefaultISO4217Map_ContainsExpectedEntries(t *testing.T) {
	m := cardauth.DefaultISO4217Map
	if len(m) != 3 {
		t.Errorf("expected 3 entries, got %d", len(m))
	}
	expected := map[string]string{"230": "ETB", "840": "USD", "978": "EUR"}
	for k, v := range expected {
		if m[k] != v {
			t.Errorf("DefaultISO4217Map[%q] = %q, want %q", k, m[k], v)
		}
	}
}
