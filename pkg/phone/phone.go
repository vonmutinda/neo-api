package phone

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/nyaruka/phonenumbers"
)

// PhoneNumber is a parsed, validated phone number value type.
// It stores the ITU country calling code and the national (subscriber) number
// separately, enabling formatted output in any style while keeping the
// canonical E.164 form as the storage and wire representation.
type PhoneNumber struct {
	CountryCode string // ITU calling code, e.g. "251" (Ethiopia), "1" (US/CA)
	Number      string // national number without leading zero, e.g. "911223344"
}

var errEmpty = errors.New("phone number is empty")

// RegionForCountryCode returns the primary ISO 3166-1 alpha-2 region
// for a given ITU calling code string, e.g. "251" -> "ET", "1" -> "US".
func RegionForCountryCode(cc string) string {
	n, err := strconv.Atoi(cc)
	if err != nil {
		return ""
	}
	return phonenumbers.GetRegionCodeForCountryCode(n)
}

// Parse parses a raw E.164 phone string into a PhoneNumber.
func Parse(raw string) (PhoneNumber, error) {
	if raw == "" {
		return PhoneNumber{}, errEmpty
	}

	parsed, err := phonenumbers.Parse(raw, "")
	if err != nil {
		return PhoneNumber{}, fmt.Errorf("invalid phone number %q: %w", raw, err)
	}

	if !phonenumbers.IsValidNumber(parsed) {
		return PhoneNumber{}, fmt.Errorf("phone number %q is not valid for region %s",
			raw, phonenumbers.GetRegionCodeForNumber(parsed))
	}

	return PhoneNumber{
		CountryCode: strconv.Itoa(int(parsed.GetCountryCode())),
		Number:      strconv.FormatUint(parsed.GetNationalNumber(), 10),
	}, nil
}

// MustParse is like Parse but panics on error. Intended for tests and static data.
func MustParse(raw string) PhoneNumber {
	p, err := Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("phone.MustParse(%q): %v", raw, err))
	}
	return p
}

// parsed reconstructs the underlying phonenumbers object for formatting.
func (p PhoneNumber) parsed() *phonenumbers.PhoneNumber {
	nat, _ := strconv.ParseUint(p.Number, 10, 64)
	cc, _ := strconv.Atoi(p.CountryCode)
	cc32 := int32(cc)
	return &phonenumbers.PhoneNumber{
		CountryCode:    &cc32,
		NationalNumber: &nat,
	}
}

// E164 returns the canonical E.164 representation, e.g. "+251911223344".
func (p PhoneNumber) E164() string {
	return phonenumbers.Format(p.parsed(), phonenumbers.E164)
}

// International returns the international format, e.g. "+251 91 122 3344".
func (p PhoneNumber) International() string {
	return phonenumbers.Format(p.parsed(), phonenumbers.INTERNATIONAL)
}

// National returns the national format, e.g. "091 122 3344".
func (p PhoneNumber) National() string {
	return phonenumbers.Format(p.parsed(), phonenumbers.NATIONAL)
}

// Region returns the ISO 3166-1 alpha-2 region code, e.g. "ET", "US".
func (p PhoneNumber) Region() string {
	return RegionForCountryCode(p.CountryCode)
}

// IsMobile reports whether the number is a mobile line.
func (p PhoneNumber) IsMobile() bool {
	t := phonenumbers.GetNumberType(p.parsed())
	return t == phonenumbers.MOBILE || t == phonenumbers.FIXED_LINE_OR_MOBILE
}

// IsValid reports whether the number is valid for its detected region.
func (p PhoneNumber) IsValid() bool {
	return phonenumbers.IsValidNumber(p.parsed())
}

// String implements fmt.Stringer; returns E.164.
func (p PhoneNumber) String() string {
	return p.E164()
}

// IsZero reports whether the PhoneNumber is the zero value (unpopulated).
func (p PhoneNumber) IsZero() bool {
	return p.CountryCode == "" && p.Number == ""
}

// --- JSON marshaling (wire format = E.164 string) ---

func (p PhoneNumber) MarshalJSON() ([]byte, error) {
	if p.IsZero() {
		return json.Marshal("")
	}
	return json.Marshal(p.E164())
}

func (p *PhoneNumber) UnmarshalJSON(data []byte) error {
	// Try E.164 string first: "+251911223344"
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		if raw == "" {
			*p = PhoneNumber{}
			return nil
		}
		parsed, err := Parse(raw)
		if err != nil {
			return fmt.Errorf("phone: %w", err)
		}
		*p = parsed
		return nil
	}

	// Try structured object: { "countryCode": "251", "number": "911223344" }
	var obj struct {
		CountryCode string `json:"countryCode"`
		Number      string `json:"number"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("phone: expected JSON string or {countryCode, number} object: %w", err)
	}
	if obj.CountryCode == "" && obj.Number == "" {
		*p = PhoneNumber{}
		return nil
	}
	e164 := "+" + obj.CountryCode + obj.Number
	parsed, err := Parse(e164)
	if err != nil {
		return fmt.Errorf("phone: %w", err)
	}
	*p = parsed
	return nil
}

// --- database/sql interfaces (storage format = E.164 text) ---

// Value implements driver.Valuer for database writes.
func (p PhoneNumber) Value() (driver.Value, error) {
	if p.IsZero() {
		return nil, nil
	}
	return p.E164(), nil
}

// Scan implements sql.Scanner for database reads.
func (p *PhoneNumber) Scan(src any) error {
	if src == nil {
		*p = PhoneNumber{}
		return nil
	}
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("phone: cannot scan %T into PhoneNumber", src)
	}
	if s == "" {
		*p = PhoneNumber{}
		return nil
	}
	parsed, err := Parse(s)
	if err != nil {
		return fmt.Errorf("phone: scan: %w", err)
	}
	*p = parsed
	return nil
}
