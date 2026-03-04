package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_UnderLimit(t *testing.T) {
	c := cache.NewMemoryCache()
	handler := RateLimit(RateLimitConfig{Burst: 10}, c)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req = SetUserID(req, "user-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_OverLimit(t *testing.T) {
	c := cache.NewMemoryCache()
	handler := RateLimit(RateLimitConfig{Burst: 2}, c)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req = SetUserID(req, "user-rate-test")

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if i < 2 {
			assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, rec.Code, "request %d should be rate limited", i+1)
			assert.Equal(t, "1", rec.Header().Get("Retry-After"))
		}
	}
}
