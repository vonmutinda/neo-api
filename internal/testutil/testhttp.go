package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vonmutinda/neo/internal/domain"
	authsvc "github.com/vonmutinda/neo/internal/services/auth"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const testSigningKey = "test-signing-key-for-tests-32b!"

// TestJWTConfig returns a JWTConfig using a fixed test secret.
func TestJWTConfig() *authsvc.JWTConfig {
	return authsvc.NewJWTConfig(testSigningKey)
}

// TestAdminJWTConfig returns an admin JWTConfig using a fixed test secret.
func TestAdminJWTConfig() middleware.JWTConfig {
	return middleware.JWTConfig{
		Secret: []byte(testSigningKey),
	}
}

// MustCreateToken generates a signed user JWT for use in tests.
func MustCreateToken(t *testing.T, userID string) string {
	t.Helper()
	token, err := TestJWTConfig().CreateToken(userID, "test-session", time.Hour)
	require.NoError(t, err)
	return token
}

// MustCreateAdminToken generates a signed admin JWT for use in tests.
func MustCreateAdminToken(t *testing.T, staffID string, role domain.StaffRole) string {
	t.Helper()
	cfg := TestAdminJWTConfig()
	token, err := middleware.GenerateAdminJWT(staffID, role, cfg.Secret, cfg.Issuer, cfg.Audience, time.Hour)
	require.NoError(t, err)
	return token
}

// NewAuthRequest creates an http.Request with a Bearer token.
// Pass a signed JWT as the token parameter (use MustCreateToken or
// MustCreateAdminToken to generate one). Pass "" for unauthenticated requests.
func NewAuthRequest(t *testing.T, method, url string, body any, token string) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// DoRequest sends a request via http.DefaultClient and returns the response.
func DoRequest(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// MustDecodeJSON decodes the {"data": ...} envelope from a response body into dst.
func MustDecodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	err := json.NewDecoder(resp.Body).Decode(&envelope)
	require.NoError(t, err)
	if dst != nil && len(envelope.Data) > 0 {
		require.NoError(t, json.Unmarshal(envelope.Data, dst))
	}
}
