package domain

import "time"

// Session represents an active user authentication session backed by a
// refresh token. Access tokens are short-lived JWTs; the refresh token is
// stored server-side and rotated on each refresh.
type Session struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	RefreshToken string     `json:"-"`
	UserAgent    *string    `json:"userAgent,omitempty"`
	IPAddress    *string    `json:"ipAddress,omitempty"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}
