package config

import "time"

type Web struct {
	Port              int           `conf:"default:8080"`
	ReadTimeout       time.Duration `conf:"default:10s"`
	ReadHeaderTimeout time.Duration `conf:"default:0s"`
	WriteTimeout      time.Duration `conf:"default:10s"`
	IdleTimeout       time.Duration `conf:"default:120s"`
	HTTPTimeout       time.Duration `conf:"default:20s"`
	ShutdownTimeout   time.Duration `conf:"default:20s"`
}

type CorsSettings struct {
	AllowedOrigins []string `conf:"default:*"`
	AllowedMethods []string `conf:"default:GET;POST;PUT;PATCH;DELETE;OPTIONS"`
	AllowedHeaders []string `conf:"default:Accept;Authorization;Content-Type;Idempotency-Key;X-Request-Id"`
	ExposedHeaders []string `conf:"default:Idempotency-Replayed;Retry-After"`
}

type JWT struct {
	SigningKey string
}
