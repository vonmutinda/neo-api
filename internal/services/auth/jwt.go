package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig handles JWT token creation and validation using HMAC-SHA256.
type JWTConfig struct {
	signingKey []byte
	keyFunc    func(*jwt.Token) (any, error)
}

// NewJWTConfig creates a JWTConfig with the given HMAC signing key.
func NewJWTConfig(signingKey string) *JWTConfig {
	cfg := &JWTConfig{
		signingKey: []byte(signingKey),
	}
	cfg.keyFunc = func(*jwt.Token) (any, error) {
		return cfg.signingKey, nil
	}
	return cfg
}

// CreateToken generates a signed JWT for the given user and session.
func (c *JWTConfig) CreateToken(userID, sessionID string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":        userID,
		"session_id": sessionID,
		"iat":        now.Unix(),
		"exp":        now.Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(c.signingKey)
}

// ParseToken validates a JWT string and returns the extracted claims.
func (c *JWTConfig) ParseToken(tokenString string) (*Claims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	token, err := parser.Parse(tokenString, c.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, err := mapClaims.GetSubject()
	if err != nil || sub == "" {
		return nil, fmt.Errorf("missing subject claim")
	}

	sessionID, _ := mapClaims["session_id"].(string)

	exp, _ := mapClaims.GetExpirationTime()
	var expiresAt time.Time
	if exp != nil {
		expiresAt = exp.Time
	}

	return &Claims{
		UserID:    sub,
		SessionID: sessionID,
		ExpiresAt: expiresAt,
	}, nil
}

// ParseTokenUserID validates the token and returns just the user ID.
func (c *JWTConfig) ParseTokenUserID(tokenString string) (string, error) {
	claims, err := c.ParseToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
