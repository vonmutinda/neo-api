package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type CurrencyBalanceRepository interface {
	Create(ctx context.Context, balance *domain.CurrencyBalance) error
	SoftDelete(ctx context.Context, userID, currencyCode string) error
	Reactivate(ctx context.Context, userID, currencyCode string) (*domain.CurrencyBalance, error)
	ListActiveByUser(ctx context.Context, userID string) ([]domain.CurrencyBalance, error)
	GetByUserAndCurrency(ctx context.Context, userID, currencyCode string) (*domain.CurrencyBalance, error)
	GetSoftDeleted(ctx context.Context, userID, currencyCode string) (*domain.CurrencyBalance, error)
}

type pgCurrencyBalanceRepo struct{ db DBTX }

func NewCurrencyBalanceRepository(db DBTX) CurrencyBalanceRepository {
	return &pgCurrencyBalanceRepo{db: db}
}

func (r *pgCurrencyBalanceRepo) Create(ctx context.Context, bal *domain.CurrencyBalance) error {
	query := `
		INSERT INTO currency_balances (user_id, currency_code, is_primary, fx_source)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query, bal.UserID, bal.CurrencyCode, bal.IsPrimary, bal.FXSource).
		Scan(&bal.ID, &bal.CreatedAt)
}

func (r *pgCurrencyBalanceRepo) SoftDelete(ctx context.Context, userID, currencyCode string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE currency_balances SET deleted_at = NOW()
		 WHERE user_id = $1 AND currency_code = $2 AND deleted_at IS NULL`,
		userID, currencyCode)
	if err != nil {
		return fmt.Errorf("soft-deleting currency balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBalanceNotActive
	}
	return nil
}

func (r *pgCurrencyBalanceRepo) Reactivate(ctx context.Context, userID, currencyCode string) (*domain.CurrencyBalance, error) {
	var bal domain.CurrencyBalance
	err := r.db.QueryRow(ctx,
		`UPDATE currency_balances SET deleted_at = NULL
		 WHERE user_id = $1 AND currency_code = $2 AND deleted_at IS NOT NULL
		 RETURNING id, user_id, currency_code, is_primary, fx_source, created_at, deleted_at`,
		userID, currencyCode).
		Scan(&bal.ID, &bal.UserID, &bal.CurrencyCode, &bal.IsPrimary, &bal.FXSource, &bal.CreatedAt, &bal.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrBalanceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reactivating currency balance: %w", err)
	}
	return &bal, nil
}

func (r *pgCurrencyBalanceRepo) ListActiveByUser(ctx context.Context, userID string) ([]domain.CurrencyBalance, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, currency_code, is_primary, fx_source, created_at, deleted_at
		 FROM currency_balances
		 WHERE user_id = $1 AND deleted_at IS NULL
		 ORDER BY is_primary DESC, created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing active currency balances: %w", err)
	}
	defer rows.Close()

	var result []domain.CurrencyBalance
	for rows.Next() {
		var b domain.CurrencyBalance
		if err := rows.Scan(&b.ID, &b.UserID, &b.CurrencyCode, &b.IsPrimary, &b.FXSource, &b.CreatedAt, &b.DeletedAt); err != nil {
			return nil, fmt.Errorf("scanning currency balance: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (r *pgCurrencyBalanceRepo) GetByUserAndCurrency(ctx context.Context, userID, currencyCode string) (*domain.CurrencyBalance, error) {
	var b domain.CurrencyBalance
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, currency_code, is_primary, fx_source, created_at, deleted_at
		 FROM currency_balances
		 WHERE user_id = $1 AND currency_code = $2 AND deleted_at IS NULL`,
		userID, currencyCode).
		Scan(&b.ID, &b.UserID, &b.CurrencyCode, &b.IsPrimary, &b.FXSource, &b.CreatedAt, &b.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrBalanceNotActive
	}
	if err != nil {
		return nil, fmt.Errorf("getting currency balance: %w", err)
	}
	return &b, nil
}

func (r *pgCurrencyBalanceRepo) GetSoftDeleted(ctx context.Context, userID, currencyCode string) (*domain.CurrencyBalance, error) {
	var b domain.CurrencyBalance
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, currency_code, is_primary, fx_source, created_at, deleted_at
		 FROM currency_balances
		 WHERE user_id = $1 AND currency_code = $2 AND deleted_at IS NOT NULL`,
		userID, currencyCode).
		Scan(&b.ID, &b.UserID, &b.CurrencyCode, &b.IsPrimary, &b.FXSource, &b.CreatedAt, &b.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrBalanceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting soft-deleted currency balance: %w", err)
	}
	return &b, nil
}

var _ CurrencyBalanceRepository = (*pgCurrencyBalanceRepo)(nil)
