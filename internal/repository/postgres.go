package repository

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the minimal interface satisfied by both *pgxpool.Pool and pgx.Tx,
// allowing repository methods to work inside or outside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// DB wraps a pgxpool.Pool and provides transaction helpers.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new DB wrapper around the given connection pool.
func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{Pool: pool}
}

// Cfg holds the configuration for the connection pool.
type Cfg struct {
	Host              string
	Port              int
	Username          string
	Password          string
	Database          string
	SSLMode           string
	DisableTLS        bool
	MaxConns          int
	MinConns          int
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// ConnectPool creates a new connection pool.
func ConnectPool(ctx context.Context, cfg *Cfg) (*pgxpool.Pool, error) {
	q := make(url.Values)
	if cfg.DisableTLS {
		q.Set("sslmode", "disable")
	} else {
		q.Set("sslmode", cfg.SSLMode)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.Username, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:     cfg.Database,
		RawQuery: q.Encode(),
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("creating postgres pool: %w", err)
	}
	pool.Config().MaxConns = int32(cfg.MaxConns)
	pool.Config().MinConns = int32(cfg.MinConns)
	pool.Config().MaxConnLifetime = cfg.MaxConnLifetime
	pool.Config().MaxConnIdleTime = cfg.MaxConnIdleTime
	pool.Config().HealthCheckPeriod = cfg.HealthCheckPeriod
	return pool, nil
}

// WithTx executes fn inside a database transaction. If fn returns an error,
// the transaction is rolled back. Otherwise, it is committed.
func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rolling back transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Close shuts down the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}
