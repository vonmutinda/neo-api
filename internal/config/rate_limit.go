package config

type RateLimit struct {
	RequestsPerSecond float64 `conf:"default:50"`
	Burst             int     `conf:"default:100"`
}
