package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type clientIPKey struct{}

// ClientIP is middleware that sets the client IP on the request context.
// Use GetClientIP(ctx) from handlers or services to read it. IP is stripped
// of port (e.g. from RemoteAddr) so it is suitable for audit and geo lookup.
func ClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIPFromRequest(r)
		ctx := context.WithValue(r.Context(), clientIPKey{}, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return stripPort(strings.TrimSpace(xff))
	}
	return stripPort(r.RemoteAddr)
}

func stripPort(s string) string {
	if s == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		return s
	}
	return host
}

// GetClientIP returns the client IP from the context, or empty string if not set.
func GetClientIP(ctx context.Context) string {
	if v, ok := ctx.Value(clientIPKey{}).(string); ok {
		return v
	}
	return ""
}
