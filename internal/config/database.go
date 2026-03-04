package config

import (
	"time"

	"github.com/vonmutinda/neo/internal/repository"
)

type Database struct {
	URL               string
	Username          string        `conf:"default:neobank"`
	Password          string        `conf:"default:neobank_dev"`
	Host              string        `conf:"default:localhost"`
	Port              int           `conf:"default:5432"`
	Database          string        `conf:"default:neobank"`
	HealthCheckPeriod time.Duration `conf:"default:30s"`
	MaxIdleConns      int           `conf:"default:25"`
	MaxOpenConns      int           `conf:"default:50"`
	DisableTLS        bool          `conf:"default:false"`
	MaxConnIdleTime   time.Duration `conf:"default:5m"`
	SSLMode           string        `conf:"default:disable"`
}

func (c *Database) BuildConfig() *repository.Cfg {
	return &repository.Cfg{
		Host:              c.Host,
		Port:              c.Port,
		Username:          c.Username,
		Password:          c.Password,
		Database:          c.Database,
		SSLMode:           c.SSLMode,
		DisableTLS:        c.DisableTLS,
		MaxConns:          c.MaxOpenConns,
		MinConns:          c.MaxIdleConns,
		MaxConnIdleTime:   c.MaxConnIdleTime,
		HealthCheckPeriod: c.HealthCheckPeriod,
	}
}
