package cardauth

import "github.com/vonmutinda/neo/pkg/money"

// ISO4217Resolver maps ISO 4217 numeric codes (e.g. "840") to our currency codes (e.g. "USD").
// Used to resolve merchant currency from ISO 8583 DE 49.
type ISO4217Resolver interface {
	Resolve(iso4217Numeric string) string
}

// DefaultISO4217Map contains the standard ISO 4217 numeric-to-currency mappings.
// These codes are an international standard that essentially never changes.
var DefaultISO4217Map = map[string]string{
	"230": money.CurrencyETB,
	"840": money.CurrencyUSD,
	"978": money.CurrencyEUR,
}

// StaticISO4217Resolver uses a fixed map.
type StaticISO4217Resolver map[string]string

// Resolve returns the mapped code or ETB as fallback.
func (s StaticISO4217Resolver) Resolve(iso4217Numeric string) string {
	if code, ok := s[iso4217Numeric]; ok {
		return code
	}
	return money.CurrencyETB
}
