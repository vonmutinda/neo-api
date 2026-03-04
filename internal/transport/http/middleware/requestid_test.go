package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var capturedReqID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReqID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	respReqID := rec.Header().Get(HeaderXRequestID)
	require.NotEmpty(t, respReqID)
	assert.Equal(t, respReqID, capturedReqID)
	_, err := uuid.Parse(respReqID)
	assert.NoError(t, err)
}

func TestRequestID_PreservesExisting(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		assert.Equal(t, "abc-123", reqID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderXRequestID, "abc-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "abc-123", rec.Header().Get(HeaderXRequestID))
}

func TestRequestID_RejectsInvalid(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		assert.NotEqual(t, "invalid@#$%", reqID)
		_, err := uuid.Parse(reqID)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderXRequestID, "invalid@#$%")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	respReqID := rec.Header().Get(HeaderXRequestID)
	require.NotEmpty(t, respReqID)
	assert.NotEqual(t, "invalid@#$%", respReqID)
	_, err := uuid.Parse(respReqID)
	assert.NoError(t, err)
}
