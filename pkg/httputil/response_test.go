package httputil

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON_WrapsInEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"id": "123"}
	WriteJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var body Envelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.NotNil(t, body.Data)
	assert.Equal(t, "123", body.Data.(map[string]interface{})["id"])
	assert.Empty(t, body.Error)
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusNoContent, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Empty(t, w.Body.Bytes())
}

func TestWriteError_WrapsInEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "bad request")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var body Envelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "bad request", body.Error)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestHandleError_AppError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	appErr := domain.NewAppError(http.StatusForbidden, "access denied", nil)

	HandleError(w, r, appErr)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var body Envelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "access denied", body.Error)
}

func TestHandleError_SentinelError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	HandleError(w, r, domain.ErrUserNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body Envelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "user not found", body.Error)
}

func TestHandleError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	HandleError(w, r, errors.New("boom"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body Envelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "internal error", body.Error)
}

func TestDecodeJSON_Success(t *testing.T) {
	body := map[string]string{"id": "123"}
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")

	var dst struct {
		ID string `json:"id"`
	}
	err := DecodeJSON(r, &dst)
	require.NoError(t, err)
	assert.Equal(t, "123", dst.ID)
}

func TestDecodeJSON_RejectsUnknownFields(t *testing.T) {
	body := `{"id":"123","unknown":"field"}`
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")

	var dst struct {
		ID string `json:"id"`
	}
	err := DecodeJSON(r, &dst)
	require.Error(t, err)
}

func TestDecodeJSON_WrongContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"id":"123"}`))
	r.Header.Set("Content-Type", "text/plain")

	var dst struct {
		ID string `json:"id"`
	}
	err := DecodeJSON(r, &dst)
	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, http.StatusUnsupportedMediaType, appErr.Code)
	assert.Equal(t, "Content-Type must be application/json", appErr.Message)
}
