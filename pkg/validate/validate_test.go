package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhoneE164(t *testing.T) {
	tests := []struct {
		name    string
		phone   string
		wantErr bool
	}{
		{"valid", "+251911223344", false},
		{"invalid_no_plus", "0911223344", true},
		{"invalid_short", "+0123", true},
		{"invalid_empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PhoneE164(tt.phone)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAmountCents(t *testing.T) {
	tests := []struct {
		name    string
		cents   int64
		wantErr bool
	}{
		{"valid_one", 1, false},
		{"valid_100k", 100000, false},
		{"invalid_zero", 0, true},
		{"invalid_negative", -1, true},
		{"invalid_over_max", 10_000_000_001, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AmountCents(tt.cents)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStringLength(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		min     int
		max     int
		wantErr bool
	}{
		{"within_bounds", "hello", 1, 10, false},
		{"too_short", "hi", 5, 10, true},
		{"too_long", "hello world", 1, 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := StringLength(tt.val, "field", tt.min, tt.max)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRequired(t *testing.T) {
	require.NoError(t, Required("non-empty", "field"))
	err := Required("", "field")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestCurrencyCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid_ETB", "ETB", false},
		{"valid_USD", "USD", false},
		{"valid_EUR", "EUR", false},
		{"invalid_GBP", "GBP", true},
		{"invalid_empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CurrencyCode(tt.code)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEnumValue(t *testing.T) {
	allowed := []string{"a", "b", "c"}
	require.NoError(t, EnumValue("a", "field", allowed))
	require.NoError(t, EnumValue("b", "field", allowed))
	err := EnumValue("d", "field", allowed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be one of")
}

func TestStruct(t *testing.T) {
	type phoneStruct struct {
		Phone string `validate:"required,e164"`
	}

	// Valid struct
	v := phoneStruct{Phone: "+251911223344"}
	require.NoError(t, Struct(&v))

	// Invalid: empty required
	inv := phoneStruct{Phone: ""}
	err := Struct(&inv)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)

	// Invalid: bad e164 format
	inv2 := phoneStruct{Phone: "0911223344"}
	err = Struct(&inv2)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}
