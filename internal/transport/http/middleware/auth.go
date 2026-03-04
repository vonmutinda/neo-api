package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/vonmutinda/neo/pkg/httputil"
)

// TokenParser validates a JWT string and returns the user ID.
// Implemented by auth.JWTConfig.
type TokenParser interface {
	ParseTokenUserID(tokenString string) (string, error)
}

// Auth creates JWT validation middleware. It extracts the Bearer token from
// the Authorization header, validates it via the TokenParser, and injects the
// user ID into the request context.
func Auth(tp TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				httputil.WriteError(w, http.StatusUnauthorized, "invalid authorization format, expected: Bearer <token>")
				return
			}

			userID, err := tp.ParseTokenUserID(token)
			if err != nil {
				httputil.WriteError(w, http.StatusUnauthorized, fmt.Sprintf("invalid token: %v", err))
				return
			}

			r = SetUserID(r, userID)
			next.ServeHTTP(w, r)
		})
	}
}
