package phone

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantCC     string
		wantNumber string
		wantE164   string
		wantRegion string
		wantMobile bool
	}{
		{
			name:       "Ethiopian mobile",
			raw:        "+251911223344",
			wantCC:     "251",
			wantNumber: "911223344",
			wantE164:   "+251911223344",
			wantRegion: "ET",
			wantMobile: true,
		},
		{
			name:       "US mobile",
			raw:        "+14155552671",
			wantCC:     "1",
			wantNumber: "4155552671",
			wantE164:   "+14155552671",
			wantRegion: "US",
			wantMobile: true,
		},
		{
			name:       "UK mobile",
			raw:        "+447400123456",
			wantCC:     "44",
			wantNumber: "7400123456",
			wantE164:   "+447400123456",
			wantRegion: "GB",
			wantMobile: true,
		},
		{
			name:       "Kenyan mobile",
			raw:        "+254712345678",
			wantCC:     "254",
			wantNumber: "712345678",
			wantE164:   "+254712345678",
			wantRegion: "KE",
			wantMobile: true,
		},
		{
			name:       "German landline",
			raw:        "+493012345678",
			wantCC:     "49",
			wantNumber: "3012345678",
			wantE164:   "+493012345678",
			wantRegion: "DE",
			wantMobile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(tt.raw)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCC, p.CountryCode)
			assert.Equal(t, tt.wantNumber, p.Number)
			assert.Equal(t, tt.wantE164, p.E164())
			assert.Equal(t, tt.wantRegion, p.Region())
			assert.Equal(t, tt.wantMobile, p.IsMobile())
			assert.True(t, p.IsValid())
			assert.Equal(t, tt.wantE164, p.String())
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty string", raw: ""},
		{name: "just a plus", raw: "+"},
		{name: "letters only", raw: "abcdefg"},
		{name: "too short", raw: "+2519"},
		{name: "invalid country code", raw: "+0911223344"},
		{name: "invalid for region", raw: "+25112345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.raw)
			assert.Error(t, err)
		})
	}
}

func TestMustParse(t *testing.T) {
	p := MustParse("+251911223344")
	assert.Equal(t, "251", p.CountryCode)
	assert.Equal(t, "911223344", p.Number)
}

func TestMustParsePanics(t *testing.T) {
	assert.Panics(t, func() {
		MustParse("invalid")
	})
}

func TestFormattingMethods(t *testing.T) {
	p := MustParse("+251911223344")

	assert.Equal(t, "+251911223344", p.E164())
	assert.NotEmpty(t, p.International())
	assert.NotEmpty(t, p.National())
	assert.Contains(t, p.International(), "+251")
}

func TestIsZero(t *testing.T) {
	var zero PhoneNumber
	assert.True(t, zero.IsZero())

	p := MustParse("+251911223344")
	assert.False(t, p.IsZero())
}

func TestRegionForCountryCode(t *testing.T) {
	tests := []struct {
		cc   string
		want string
	}{
		{"251", "ET"},
		{"1", "US"},
		{"44", "GB"},
		{"254", "KE"},
		{"49", "DE"},
		{"234", "NG"},
		{"", ""},
		{"abc", ""},
		{"99999", "ZZ"},
	}
	for _, tt := range tests {
		t.Run(tt.cc, func(t *testing.T) {
			assert.Equal(t, tt.want, RegionForCountryCode(tt.cc))
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := MustParse("+251911223344")

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"+251911223344"`, string(data))

	var decoded PhoneNumber
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original.CountryCode, decoded.CountryCode)
	assert.Equal(t, original.Number, decoded.Number)
	assert.Equal(t, original.E164(), decoded.E164())
}

func TestJSONInStruct(t *testing.T) {
	type contact struct {
		Name  string      `json:"name"`
		Phone PhoneNumber `json:"phone"`
	}

	c := contact{
		Name:  "Alice",
		Phone: MustParse("+14155552671"),
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var decoded contact
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "Alice", decoded.Name)
	assert.Equal(t, "+14155552671", decoded.Phone.E164())
	assert.Equal(t, "1", decoded.Phone.CountryCode)
}

func TestJSONZeroValue(t *testing.T) {
	var zero PhoneNumber
	data, err := json.Marshal(zero)
	require.NoError(t, err)
	assert.Equal(t, `""`, string(data))

	var decoded PhoneNumber
	err = json.Unmarshal([]byte(`""`), &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.IsZero())
}

func TestJSONUnmarshalInvalid(t *testing.T) {
	var p PhoneNumber
	err := json.Unmarshal([]byte(`"notaphone"`), &p)
	assert.Error(t, err)

	err = json.Unmarshal([]byte(`123`), &p)
	assert.Error(t, err)
}

func TestSQLValueAndScan(t *testing.T) {
	p := MustParse("+254712345678")

	val, err := p.Value()
	require.NoError(t, err)
	assert.Equal(t, "+254712345678", val)

	var scanned PhoneNumber
	err = scanned.Scan(val)
	require.NoError(t, err)
	assert.Equal(t, p.CountryCode, scanned.CountryCode)
	assert.Equal(t, p.Number, scanned.Number)
}

func TestSQLScanNil(t *testing.T) {
	var p PhoneNumber
	err := p.Scan(nil)
	require.NoError(t, err)
	assert.True(t, p.IsZero())
}

func TestSQLScanEmpty(t *testing.T) {
	var p PhoneNumber
	err := p.Scan("")
	require.NoError(t, err)
	assert.True(t, p.IsZero())
}

func TestSQLScanInvalidType(t *testing.T) {
	var p PhoneNumber
	err := p.Scan(12345)
	assert.Error(t, err)
}

func TestSQLScanInvalidNumber(t *testing.T) {
	var p PhoneNumber
	err := p.Scan("notaphone")
	assert.Error(t, err)
}

func TestSQLValueZero(t *testing.T) {
	var zero PhoneNumber
	val, err := zero.Value()
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestMultipleCountries(t *testing.T) {
	numbers := map[string]string{
		"ET": "+251911223344",
		"US": "+14155552671",
		"GB": "+447400123456",
		"KE": "+254712345678",
		"DE": "+493012345678",
		"NG": "+2348031234567",
	}

	for expectedRegion, raw := range numbers {
		t.Run(expectedRegion, func(t *testing.T) {
			p, err := Parse(raw)
			require.NoError(t, err)
			assert.Equal(t, expectedRegion, p.Region())
			assert.True(t, p.IsValid())
			assert.Equal(t, raw, p.E164())
		})
	}
}
