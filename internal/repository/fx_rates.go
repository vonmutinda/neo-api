package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type FXRateRepository interface {
	Insert(ctx context.Context, rate *domain.FXRate) error
	GetLatest(ctx context.Context, from, to string) (*domain.FXRate, error)
	GetLatestAll(ctx context.Context) ([]domain.FXRate, error)
	ListHistory(ctx context.Context, from, to string, since time.Time, limit int) ([]domain.FXRate, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

type pgFXRateRepo struct{ db DBTX }

func NewFXRateRepository(db DBTX) FXRateRepository {
	return &pgFXRateRepo{db: db}
}

func (r *pgFXRateRepo) Insert(ctx context.Context, rate *domain.FXRate) error {
	query := `
		INSERT INTO fx_rates (from_currency, to_currency, mid_rate, bid_rate, ask_rate,
			spread_percent, source, fetched_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		rate.FromCurrency, rate.ToCurrency, rate.MidRate, rate.BidRate, rate.AskRate,
		rate.SpreadPercent, rate.Source, rate.FetchedAt,
	).Scan(&rate.ID, &rate.CreatedAt)
}

func (r *pgFXRateRepo) GetLatest(ctx context.Context, from, to string) (*domain.FXRate, error) {
	query := `
		SELECT id, from_currency, to_currency, mid_rate, bid_rate, ask_rate,
			spread_percent, source, fetched_at, created_at
		FROM fx_rates
		WHERE from_currency = $1 AND to_currency = $2
		ORDER BY created_at DESC
		LIMIT 1`
	return r.scanOne(ctx, query, from, to)
}

func (r *pgFXRateRepo) GetLatestAll(ctx context.Context) ([]domain.FXRate, error) {
	query := `
		SELECT DISTINCT ON (from_currency, to_currency)
			id, from_currency, to_currency, mid_rate, bid_rate, ask_rate,
			spread_percent, source, fetched_at, created_at
		FROM fx_rates
		ORDER BY from_currency, to_currency, created_at DESC`
	return r.scanMany(ctx, query)
}

func (r *pgFXRateRepo) ListHistory(ctx context.Context, from, to string, since time.Time, limit int) ([]domain.FXRate, error) {
	query := `
		SELECT id, from_currency, to_currency, mid_rate, bid_rate, ask_rate,
			spread_percent, source, fetched_at, created_at
		FROM fx_rates
		WHERE from_currency = $1 AND to_currency = $2 AND created_at >= $3
		ORDER BY created_at DESC
		LIMIT $4`
	return r.scanMany(ctx, query, from, to, since, limit)
}

func (r *pgFXRateRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM fx_rates WHERE created_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("deleting old fx rates: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *pgFXRateRepo) scanOne(ctx context.Context, query string, args ...any) (*domain.FXRate, error) {
	row := r.db.QueryRow(ctx, query, args...)
	var rate domain.FXRate
	err := row.Scan(
		&rate.ID, &rate.FromCurrency, &rate.ToCurrency,
		&rate.MidRate, &rate.BidRate, &rate.AskRate,
		&rate.SpreadPercent, &rate.Source, &rate.FetchedAt, &rate.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrFXRateNotFound
		}
		return nil, fmt.Errorf("scanning fx rate: %w", err)
	}
	return &rate, nil
}

func (r *pgFXRateRepo) scanMany(ctx context.Context, query string, args ...any) ([]domain.FXRate, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying fx rates: %w", err)
	}
	defer rows.Close()

	var rates []domain.FXRate
	for rows.Next() {
		var rate domain.FXRate
		if err := rows.Scan(
			&rate.ID, &rate.FromCurrency, &rate.ToCurrency,
			&rate.MidRate, &rate.BidRate, &rate.AskRate,
			&rate.SpreadPercent, &rate.Source, &rate.FetchedAt, &rate.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning fx rate row: %w", err)
		}
		rates = append(rates, rate)
	}
	return rates, rows.Err()
}
