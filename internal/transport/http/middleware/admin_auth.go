package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/httputil"
)

// JWTConfig holds the configuration for admin JWT validation.
type JWTConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
}

type adminStaffIDKey struct{}
type adminStaffRoleKey struct{}

// StaffIDFromContext extracts the authenticated staff ID from the request context.
func StaffIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(adminStaffIDKey{}).(string); ok {
		return v
	}
	return ""
}

// StaffRoleFromContext extracts the authenticated staff role from the request context.
func StaffRoleFromContext(ctx context.Context) domain.StaffRole {
	if v, ok := ctx.Value(adminStaffRoleKey{}).(domain.StaffRole); ok {
		return v
	}
	return ""
}

// AdminAuth creates JWT validation middleware for admin routes. It validates the
// token using a separate secret/issuer/audience from the customer JWT, and
// extracts the staff ID and role into the request context.
func AdminAuth(cfg JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				httputil.WriteError(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			staffID, role, err := validateAdminJWT(token, cfg)
			if err != nil {
				httputil.WriteError(w, http.StatusUnauthorized, fmt.Sprintf("invalid token: %v", err))
				return
			}

			ctx := context.WithValue(r.Context(), adminStaffIDKey{}, staffID)
			ctx = context.WithValue(ctx, adminStaffRoleKey{}, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission returns middleware that checks whether the staff member's
// role includes the required permission. Returns 403 if not.
func RequirePermission(perm domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := StaffRoleFromContext(r.Context())
			if role == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "missing staff context")
				return
			}
			if !domain.HasPermission(role, perm) {
				httputil.WriteError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GenerateAdminJWT creates a signed JWT for a staff member.
func GenerateAdminJWT(staffID string, role domain.StaffRole, secret []byte, issuer, audience string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  staffID,
		"role": string(role),
		"iss":  issuer,
		"aud":  []string{audience},
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func validateAdminJWT(tokenString string, cfg JWTConfig) (string, domain.StaffRole, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)

	token, err := parser.Parse(tokenString, func(t *jwt.Token) (any, error) {
		return cfg.Secret, nil
	})
	if err != nil {
		return "", "", fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", fmt.Errorf("invalid token claims")
	}

	if cfg.Issuer != "" {
		iss, _ := claims.GetIssuer()
		if iss != cfg.Issuer {
			return "", "", fmt.Errorf("invalid issuer")
		}
	}

	if cfg.Audience != "" {
		aud, _ := claims.GetAudience()
		found := false
		for _, a := range aud {
			if a == cfg.Audience {
				found = true
				break
			}
		}
		if !found {
			return "", "", fmt.Errorf("invalid audience")
		}
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", "", fmt.Errorf("missing subject claim")
	}

	roleStr, _ := claims["role"].(string)
	if roleStr == "" {
		return "", "", fmt.Errorf("missing role claim")
	}

	return sub, domain.StaffRole(roleStr), nil
}
