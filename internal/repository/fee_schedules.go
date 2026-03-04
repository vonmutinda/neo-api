package repository

import (
	"context"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type FeeScheduleRepository interface {
	Create(ctx context.Context, fs *domain.FeeSchedule) error
	GetByID(ctx context.Context, id string) (*domain.FeeSchedule, error)
	Update(ctx context.Context, fs *domain.FeeSchedule) error
	Deactivate(ctx context.Context, id string) error
	ListActive(ctx context.Context, txType domain.TransactionType) ([]domain.FeeSchedule, error)
	ListAll(ctx context.Context) ([]domain.FeeSchedule, error)
	FindMatching(ctx context.Context, txType domain.TransactionType, currency, channel *string) ([]domain.FeeSchedule, error)
}

type pgFeeScheduleRepo struct {
	db DBTX
}

func NewFeeScheduleRepository(db DBTX) FeeScheduleRepository {
	return &pgFeeScheduleRepo{db: db}
}

func (r *pgFeeScheduleRepo) Create(ctx context.Context, fs *domain.FeeSchedule) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO fee_schedules (name, fee_type, transaction_type, currency, channel,
			flat_amount_cents, percent_bps, min_fee_cents, max_fee_cents,
			is_active, effective_from, effective_to)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`,
		fs.Name, fs.FeeType, fs.TransactionType, fs.Currency, fs.Channel,
		fs.FlatAmountCents, fs.PercentBps, fs.MinFeeCents, fs.MaxFeeCents,
		fs.IsActive, fs.EffectiveFrom, fs.EffectiveTo,
	).Scan(&fs.ID, &fs.CreatedAt, &fs.UpdatedAt)
}

func (r *pgFeeScheduleRepo) GetByID(ctx context.Context, id string) (*domain.FeeSchedule, error) {
	var fs domain.FeeSchedule
	err := r.db.QueryRow(ctx,
		`SELECT id, name, fee_type, transaction_type, currency, channel,
			flat_amount_cents, percent_bps, min_fee_cents, max_fee_cents,
			is_active, effective_from, effective_to, created_at, updated_at
		FROM fee_schedules WHERE id = $1`, id,
	).Scan(&fs.ID, &fs.Name, &fs.FeeType, &fs.TransactionType, &fs.Currency, &fs.Channel,
		&fs.FlatAmountCents, &fs.PercentBps, &fs.MinFeeCents, &fs.MaxFeeCents,
		&fs.IsActive, &fs.EffectiveFrom, &fs.EffectiveTo, &fs.CreatedAt, &fs.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrFeeScheduleNotFound
	}
	if err != nil {
		return nil, err
	}
	return &fs, nil
}

func (r *pgFeeScheduleRepo) Update(ctx context.Context, fs *domain.FeeSchedule) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE fee_schedules SET name=$2, fee_type=$3, transaction_type=$4, currency=$5, channel=$6,
			flat_amount_cents=$7, percent_bps=$8, min_fee_cents=$9, max_fee_cents=$10,
			is_active=$11, effective_from=$12, effective_to=$13, updated_at=NOW()
		WHERE id=$1`,
		fs.ID, fs.Name, fs.FeeType, fs.TransactionType, fs.Currency, fs.Channel,
		fs.FlatAmountCents, fs.PercentBps, fs.MinFeeCents, fs.MaxFeeCents,
		fs.IsActive, fs.EffectiveFrom, fs.EffectiveTo,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFeeScheduleNotFound
	}
	return nil
}

func (r *pgFeeScheduleRepo) Deactivate(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE fee_schedules SET is_active=false, updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFeeScheduleNotFound
	}
	return nil
}

func (r *pgFeeScheduleRepo) ListActive(ctx context.Context, txType domain.TransactionType) ([]domain.FeeSchedule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, fee_type, transaction_type, currency, channel,
			flat_amount_cents, percent_bps, min_fee_cents, max_fee_cents,
			is_active, effective_from, effective_to, created_at, updated_at
		FROM fee_schedules
		WHERE transaction_type=$1 AND is_active=true
			AND effective_from <= NOW()
			AND (effective_to IS NULL OR effective_to > NOW())
		ORDER BY created_at DESC`, txType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeSchedules(rows)
}

func (r *pgFeeScheduleRepo) ListAll(ctx context.Context) ([]domain.FeeSchedule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, fee_type, transaction_type, currency, channel,
			flat_amount_cents, percent_bps, min_fee_cents, max_fee_cents,
			is_active, effective_from, effective_to, created_at, updated_at
		FROM fee_schedules ORDER BY transaction_type, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeSchedules(rows)
}

func (r *pgFeeScheduleRepo) FindMatching(ctx context.Context, txType domain.TransactionType, currency, channel *string) ([]domain.FeeSchedule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, fee_type, transaction_type, currency, channel,
			flat_amount_cents, percent_bps, min_fee_cents, max_fee_cents,
			is_active, effective_from, effective_to, created_at, updated_at
		FROM fee_schedules
		WHERE transaction_type=$1 AND is_active=true
			AND effective_from <= NOW()
			AND (effective_to IS NULL OR effective_to > NOW())
			AND (currency IS NULL OR currency=$2)
			AND (channel IS NULL OR channel=$3)
		ORDER BY
			(CASE WHEN currency IS NOT NULL AND channel IS NOT NULL THEN 0
			      WHEN currency IS NOT NULL THEN 1
			      WHEN channel IS NOT NULL THEN 2
			      ELSE 3 END),
			created_at DESC`,
		txType, currency, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeSchedules(rows)
}

func scanFeeSchedules(rows pgx.Rows) ([]domain.FeeSchedule, error) {
	var result []domain.FeeSchedule
	for rows.Next() {
		var fs domain.FeeSchedule
		if err := rows.Scan(&fs.ID, &fs.Name, &fs.FeeType, &fs.TransactionType, &fs.Currency, &fs.Channel,
			&fs.FlatAmountCents, &fs.PercentBps, &fs.MinFeeCents, &fs.MaxFeeCents,
			&fs.IsActive, &fs.EffectiveFrom, &fs.EffectiveTo, &fs.CreatedAt, &fs.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, fs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
