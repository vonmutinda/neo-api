package domain

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrToCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"ErrInvalidInput", ErrInvalidInput, 400},
		{"ErrUnauthorized", ErrUnauthorized, 401},
		{"ErrUserFrozen", ErrUserFrozen, 403},
		{"ErrUserNotFound", ErrUserNotFound, 404},
		{"ErrBalanceAlreadyExists", ErrBalanceAlreadyExists, 409},
		{"ErrInsufficientFunds", ErrInsufficientFunds, 422},
		{"ErrOverdraftNotEligible", ErrOverdraftNotEligible, 422},
		{"ErrOverdraftInUse", ErrOverdraftInUse, 422},
		{"ErrOverdraftETBOnly", ErrOverdraftETBOnly, 422},
		{"ErrEthSwitchTimeout", ErrEthSwitchTimeout, 503},
		{"wrapped ErrUserNotFound", fmt.Errorf("wrapped: %w", ErrUserNotFound), 404},
		{"unknown error", errors.New("unknown"), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _ := ErrToCode(tt.err)
			assert.Equal(t, tt.wantCode, code, "ErrToCode(%v) code", tt.err)
		})
	}
}

func TestAppError_Error(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		t.Parallel()
		underlying := errors.New("underlying")
		appErr := NewAppError(500, "message", underlying)
		require.Equal(t, "message: underlying", appErr.Error())
	})

	t.Run("without underlying error", func(t *testing.T) {
		t.Parallel()
		appErr := NewAppError(500, "message", nil)
		require.Equal(t, "message", appErr.Error())
	})
}

func TestAppError_Unwrap(t *testing.T) {
	t.Parallel()
	underlying := errors.New("underlying")
	appErr := NewAppError(500, "message", underlying)
	require.Equal(t, underlying, appErr.Unwrap())
}

func TestNewAppError(t *testing.T) {
	t.Parallel()
	underlying := errors.New("underlying")
	appErr := NewAppError(http.StatusBadRequest, "bad request", underlying)

	require.Equal(t, http.StatusBadRequest, appErr.Code)
	require.Equal(t, "bad request", appErr.Message)
	require.Equal(t, underlying, appErr.Err)
}
