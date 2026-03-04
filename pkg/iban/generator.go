package iban

import (
	"fmt"
	"math/big"
	"strings"
)

const (
	countryCode = "ET"
	bankCode    = "NEO"
	branchCode  = "0001"
)

// Generate produces an Ethiopian-format IBAN for the given account number and
// currency code. Format: ET{checkDigits}NEO0001{accountNumber}{currencyCode}
//
// The check digits are computed per ISO 13616 using the MOD-97 algorithm.
func Generate(accountNumber, currencyCode string) string {
	bban := bankCode + branchCode + accountNumber + strings.ToUpper(currencyCode)
	checkDigits := computeCheckDigits(countryCode, bban)
	return fmt.Sprintf("%s%s%s", countryCode, checkDigits, bban)
}

// Validate checks whether an IBAN has a valid MOD-97 check digit.
func Validate(iban string) bool {
	if len(iban) < 5 {
		return false
	}
	rearranged := iban[4:] + iban[:4]
	numeric := lettersToDigits(rearranged)

	n, ok := new(big.Int).SetString(numeric, 10)
	if !ok {
		return false
	}
	mod := new(big.Int).Mod(n, big.NewInt(97))
	return mod.Int64() == 1
}

// computeCheckDigits calculates the 2-digit check per ISO 13616.
// Rearranges as BBAN + country letters + "00", converts to integer, MOD 97,
// then check = 98 - remainder.
func computeCheckDigits(country, bban string) string {
	rearranged := lettersToDigits(bban) + lettersToDigits(country) + "00"
	n, _ := new(big.Int).SetString(rearranged, 10)
	mod := new(big.Int).Mod(n, big.NewInt(97))
	check := 98 - mod.Int64()
	return fmt.Sprintf("%02d", check)
}

// lettersToDigits replaces each letter A-Z with its numeric value (A=10..Z=35).
func lettersToDigits(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToUpper(s) {
		if ch >= 'A' && ch <= 'Z' {
			fmt.Fprintf(&b, "%d", ch-'A'+10)
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// FormatAccountNumber zero-pads a sequence value to a 10-digit account number.
func FormatAccountNumber(seq int64) string {
	return fmt.Sprintf("%010d", seq)
}
