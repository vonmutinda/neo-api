package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyLimit_UnderLimit(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 100)
	handler := BodyLimit(1 << 20)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Len(t, read, 100)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimit_OverLimit(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 2*1024*1024) // 2MB
	handler := BodyLimit(1 << 20)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		require.Error(t, err)
		var maxBytesErr *http.MaxBytesError
		assert.True(t, errors.As(err, &maxBytesErr))
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
