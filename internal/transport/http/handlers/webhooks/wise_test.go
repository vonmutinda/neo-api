package webhooks_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vonmutinda/neo/internal/transport/http/handlers/webhooks"
)

func TestWiseWebhook_ValidPayload(t *testing.T) {
	handler := webhooks.NewWiseWebhookHandler(nil, "test-secret")
	r := chi.NewRouter()
	r.Post("/webhooks/wise", handler.HandleEvent)
	server := httptest.NewServer(r)
	defer server.Close()

	body := strings.NewReader(`{"event":"transfer.completed","data":{"id":"123"}}`)
	resp, err := http.Post(server.URL+"/webhooks/wise", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWiseWebhook_EmptyBody(t *testing.T) {
	handler := webhooks.NewWiseWebhookHandler(nil, "test-secret")
	r := chi.NewRouter()
	r.Post("/webhooks/wise", handler.HandleEvent)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Post(server.URL+"/webhooks/wise", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
