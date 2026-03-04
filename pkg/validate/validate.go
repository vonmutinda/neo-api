package validate

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/go-playground/validator/v10"
)

// ErrValidation is the sentinel wrapped by all struct validation failures.
var ErrValidation = errors.New("validation failed")

var (
	once     sync.Once
	instance *validator.Validate
)

// Validator returns a package-level singleton *validator.Validate
// with all custom validators registered.
func Validator() *validator.Validate {
	once.Do(func() {
		instance = validator.New(validator.WithRequiredStructEnabled())
		instance.RegisterValidation("e164", phoneE164Validator)
		instance.RegisterValidation("currency", currencyValidator)
		instance.RegisterValidation("amount_cents", amountCentsValidator)
	})
	return instance
}

// Struct validates a struct using the singleton validator instance.
// Returns a user-friendly error combining all field violations.
func Struct(s any) error {
	if err := Validator().Struct(s); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			msgs := make([]string, 0, len(ve))
			for _, fe := range ve {
				msgs = append(msgs, formatFieldError(fe))
			}
			return fmt.Errorf("%w: %s", ErrValidation, strings.Join(msgs, "; "))
		}
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

func formatFieldError(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return field + " is required"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be at least %s", field, fe.Param())
	case "e164":
		return field + " must be a valid E.164 phone number (e.g. +251911223344)"
	case "currency":
		return field + " must be a supported currency (ETB, USD, EUR)"
	case "amount_cents":
		return field + " must be a positive amount in cents"
	default:
		return fmt.Sprintf("%s failed on '%s' validation", field, fe.Tag())
	}
}

// --- Custom validators for go-playground/validator ---

func phoneE164Validator(fl validator.FieldLevel) bool {
	_, err := phone.Parse(fl.Field().String())
	return err == nil
}

func currencyValidator(fl validator.FieldLevel) bool {
	return money.ValidateCurrency(fl.Field().String()) == nil
}

func amountCentsValidator(fl validator.FieldLevel) bool {
	return fl.Field().Int() > 0
}

// --- Standalone validation functions (backward-compatible) ---

// PhoneE164 validates an E.164 phone number using libphonenumber.
func PhoneE164(raw string) error {
	if _, err := phone.Parse(raw); err != nil {
		return fmt.Errorf("invalid phone number format: must be E.164 (e.g. +251911223344)")
	}
	return nil
}

// AmountCents validates a monetary amount in cents is positive and within bounds.
func AmountCents(cents int64) error {
	if cents <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if cents > 10_000_000_000 {
		return fmt.Errorf("amount exceeds maximum allowed (100,000,000.00)")
	}
	return nil
}

// StringLength validates a string field is within the expected length bounds.
func StringLength(val, field string, min, max int) error {
	if len(val) < min {
		return fmt.Errorf("%s must be at least %d characters", field, min)
	}
	if len(val) > max {
		return fmt.Errorf("%s must be at most %d characters", field, max)
	}
	return nil
}

// Required checks that a string field is non-empty.
func Required(val, field string) error {
	if val == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

// CurrencyCode validates that the given string is a supported ISO 4217 currency code.
func CurrencyCode(code string) error {
	if err := money.ValidateCurrency(code); err != nil {
		return fmt.Errorf("unsupported currency %q", code)
	}
	return nil
}

// EnumValue checks that val is one of the allowed values.
func EnumValue(val, field string, allowed []string) error {
	for _, a := range allowed {
		if val == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s: must be one of %v", field, allowed)
}
