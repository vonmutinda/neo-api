package config

type RedisConfig struct {
	URL      string `conf:"default:redis://localhost:6379"`
	Password string
	DB       int    `conf:"default:0"`
	PoolSize int    `conf:"default:10"`
}
