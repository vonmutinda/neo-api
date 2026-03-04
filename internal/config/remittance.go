package config

type WiseConfig struct {
	APIToken      string
	ProfileID     string
	BaseURL       string `conf:"default:https://api.transferwise.com"`
	WebhookSecret string
}
