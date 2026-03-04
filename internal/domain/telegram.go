package domain

import "time"

// TelegramLinkToken is a short-lived, cryptographically random token used
// for the /start <token> deep-link binding flow between the Next.js app
// and the Telegram bot. Created when the user requests linking; consumed once.
type TelegramLinkToken struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	Consumed  bool      `json:"consumed"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// IsValid returns true if the token has not been consumed and has not expired.
func (t *TelegramLinkToken) IsValid() bool {
	return !t.Consumed && time.Now().Before(t.ExpiresAt)
}
