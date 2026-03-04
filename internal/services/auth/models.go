package auth

import "time"

// Claims holds the parsed JWT claims extracted by JWTConfig.ParseToken.
type Claims struct {
	UserID    string
	SessionID string
	ExpiresAt time.Time
}
