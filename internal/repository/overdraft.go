package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

// OverdraftRepository persists overdraft facility state per user.
type OverdraftRepository interface {
	GetByUser(ctx context.Context, userID string) (*domain.Overdraft, error)
	CreateOrUpdate(ctx context.Context, o *domain.Overdraft) error
	UpdateUsedAndStatus(ctx context.Context, userID string, usedDeltaCents int64, status domain.OverdraftStatus, overdrawnSince *time.Time) error
	UpdateRepaid(ctx context.Context, userID string, usedCents, accruedFeeCents int64, status domain.OverdraftStatus) error
	UpdateFeeAccrual(ctx context.Context, userID string, accruedFeeCents int64, lastFeeAccrualAt time.Time) error
}

type pgOverdraftRepo struct{ db DBTX }

func NewOverdraftRepository(db DBTX) OverdraftRepository {
	return &pgOverdraftRepo{db: db}
}

func (r *pgOverdraftRepo) GetByUser(ctx context.Context, userID string) (*domain.Overdraft, error) {
	var o domain.Overdraft
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, limit_cents, used_cents, available_cents,
			daily_fee_basis_points, interest_free_days, accrued_fee_cents,
			status, overdrawn_since, last_fee_accrual_at, opted_in_at, created_at, updated_at
		FROM overdrafts WHERE user_id = $1`, userID).Scan(
		&o.ID, &o.UserID, &o.LimitCents, &o.UsedCents, &o.AvailableCents,
		&o.DailyFeeBasisPoints, &o.InterestFreeDays, &o.AccruedFeeCents,
		&o.Status, &o.OverdrawnSince, &o.LastFeeAccrualAt, &o.OptedInAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting overdraft: %w", err)
	}
	return &o, nil
}

func (r *pgOverdraftRepo) CreateOrUpdate(ctx context.Context, o *domain.Overdraft) error {
	query := `
		INSERT INTO overdrafts (
			user_id, limit_cents, daily_fee_basis_points, interest_free_days,
			status, opted_in_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			limit_cents = EXCLUDED.limit_cents,
			status = EXCLUDED.status,
			opted_in_at = COALESCE(EXCLUDED.opted_in_at, overdrafts.opted_in_at),
			updated_at = NOW()
		RETURNING id, used_cents, available_cents, accrued_fee_cents, overdrawn_since, last_fee_accrual_at, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		o.UserID, o.LimitCents, o.DailyFeeBasisPoints, o.InterestFreeDays,
		o.Status, o.OptedInAt,
	).Scan(&o.ID, &o.UsedCents, &o.AvailableCents, &o.AccruedFeeCents, &o.OverdrawnSince, &o.LastFeeAccrualAt, &o.CreatedAt, &o.UpdatedAt)
}

func (r *pgOverdraftRepo) UpdateUsedAndStatus(ctx context.Context, userID string, usedDeltaCents int64, status domain.OverdraftStatus, overdrawnSince *time.Time) error {
	if overdrawnSince != nil {
		_, err := r.db.Exec(ctx,
			`UPDATE overdrafts SET used_cents = used_cents + $2, status = $3, overdrawn_since = $4, updated_at = NOW() WHERE user_id = $1`,
			userID, usedDeltaCents, status, overdrawnSince)
		return err
	}
	_, err := r.db.Exec(ctx,
		`UPDATE overdrafts SET used_cents = used_cents + $2, status = $3, updated_at = NOW() WHERE user_id = $1`,
		userID, usedDeltaCents, status)
	return err
}

func (r *pgOverdraftRepo) UpdateRepaid(ctx context.Context, userID string, usedCents, accruedFeeCents int64, status domain.OverdraftStatus) error {
	query := `UPDATE overdrafts SET used_cents = $2, accrued_fee_cents = $3, status = $4, overdrawn_since = NULL, updated_at = NOW() WHERE user_id = $1`
	_, err := r.db.Exec(ctx, query, userID, usedCents, accruedFeeCents, status)
	return err
}

func (r *pgOverdraftRepo) UpdateFeeAccrual(ctx context.Context, userID string, accruedFeeCents int64, lastFeeAccrualAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE overdrafts SET accrued_fee_cents = accrued_fee_cents + $2, last_fee_accrual_at = $3, updated_at = NOW() WHERE user_id = $1`,
		userID, accruedFeeCents, lastFeeAccrualAt)
	return err
}

var _ OverdraftRepository = (*pgOverdraftRepo)(nil)
