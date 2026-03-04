package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
}

func RateLimit(cfg RateLimitConfig, c cache.Cache) func(http.Handler) http.Handler {
	windowSize := time.Second
	maxRequests := int64(cfg.Burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := UserIDFromContext(r.Context())
			if key == "" {
				key = r.RemoteAddr
			}

			window := time.Now().Unix()
			cacheKey := fmt.Sprintf("neo:rl:%s:%d", key, window)

			count, err := c.Increment(r.Context(), cacheKey, 2*windowSize)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if count > maxRequests {
				w.Header().Set("Retry-After", "1")
				httputil.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
