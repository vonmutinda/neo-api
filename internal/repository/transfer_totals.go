package repository

import (
	"context"
	"fmt"
)

// TransferTotalsRepository tracks daily transfer volumes per user for limit enforcement.
type TransferTotalsRepository interface {
	// GetDailyTotal returns today's total_cents for a user/currency/direction.
	GetDailyTotal(ctx context.Context, userID, currency, direction string) (int64, error)
	// GetMonthlyTotal returns this calendar month's total_cents.
	GetMonthlyTotal(ctx context.Context, userID, currency, direction string) (int64, error)
	// Increment atomically adds to today's counter (upsert).
	Increment(ctx context.Context, userID, currency, direction string, amountCents int64) error
}

type pgTransferTotalsRepo struct{ db DBTX }

func NewTransferTotalsRepository(db DBTX) TransferTotalsRepository {
	return &pgTransferTotalsRepo{db: db}
}

func (r *pgTransferTotalsRepo) GetDailyTotal(ctx context.Context, userID, currency, direction string) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(total_cents, 0) FROM transfer_daily_totals
		 WHERE user_id = $1 AND currency = $2 AND direction = $3 AND date = CURRENT_DATE`,
		userID, currency, direction,
	).Scan(&total)
	if err != nil {
		// No row means zero
		return 0, nil
	}
	return total, nil
}

func (r *pgTransferTotalsRepo) GetMonthlyTotal(ctx context.Context, userID, currency, direction string) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_cents), 0) FROM transfer_daily_totals
		 WHERE user_id = $1 AND currency = $2 AND direction = $3
		   AND date >= date_trunc('month', CURRENT_DATE)`,
		userID, currency, direction,
	).Scan(&total)
	if err != nil {
		return 0, nil
	}
	return total, nil
}

func (r *pgTransferTotalsRepo) Increment(ctx context.Context, userID, currency, direction string, amountCents int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO transfer_daily_totals (user_id, currency, direction, date, total_cents, tx_count, updated_at)
		 VALUES ($1, $2, $3, CURRENT_DATE, $4, 1, NOW())
		 ON CONFLICT (user_id, currency, direction, date)
		 DO UPDATE SET total_cents = transfer_daily_totals.total_cents + $4,
		              tx_count = transfer_daily_totals.tx_count + 1,
		              updated_at = NOW()`,
		userID, currency, direction, amountCents,
	)
	if err != nil {
		return fmt.Errorf("incrementing transfer total: %w", err)
	}
	return nil
}

var _ TransferTotalsRepository = (*pgTransferTotalsRepo)(nil)
