package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type TaxPotRepository interface {
	Create(ctx context.Context, tp *domain.TaxPot) error
	GetByID(ctx context.Context, id string) (*domain.TaxPot, error)
	Update(ctx context.Context, tp *domain.TaxPot) error
	Delete(ctx context.Context, id string) error
	ListByBusiness(ctx context.Context, businessID string) ([]domain.TaxPot, error)
	GetByBusinessAndType(ctx context.Context, businessID string, taxType domain.TaxType) (*domain.TaxPot, error)
	ListWithAutoSweep(ctx context.Context, businessID string) ([]domain.TaxPot, error)
}

type pgTaxPotRepo struct{ db DBTX }

func NewTaxPotRepository(db DBTX) TaxPotRepository {
	return &pgTaxPotRepo{db: db}
}

func (r *pgTaxPotRepo) Create(ctx context.Context, tp *domain.TaxPot) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO tax_pots (business_id, pot_id, tax_type, auto_sweep_percent, due_date, notes)
		VALUES ($1,$2,$3,$4,$5::date,$6) RETURNING id, created_at, updated_at`,
		tp.BusinessID, tp.PotID, tp.TaxType, tp.AutoSweepPercent, tp.DueDate, tp.Notes,
	).Scan(&tp.ID, &tp.CreatedAt, &tp.UpdatedAt)
}

func (r *pgTaxPotRepo) GetByID(ctx context.Context, id string) (*domain.TaxPot, error) {
	var tp domain.TaxPot
	err := r.db.QueryRow(ctx,
		`SELECT id, business_id, pot_id, tax_type, auto_sweep_percent, due_date, notes, created_at, updated_at
		FROM tax_pots WHERE id = $1`, id).Scan(
		&tp.ID, &tp.BusinessID, &tp.PotID, &tp.TaxType,
		&tp.AutoSweepPercent, &tp.DueDate, &tp.Notes, &tp.CreatedAt, &tp.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrTaxPotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting tax pot: %w", err)
	}
	return &tp, nil
}

func (r *pgTaxPotRepo) Update(ctx context.Context, tp *domain.TaxPot) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE tax_pots SET auto_sweep_percent=$2, due_date=$3::date, notes=$4, updated_at=NOW()
		WHERE id=$1`, tp.ID, tp.AutoSweepPercent, tp.DueDate, tp.Notes)
	if err != nil {
		return fmt.Errorf("updating tax pot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTaxPotNotFound
	}
	return nil
}

func (r *pgTaxPotRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM tax_pots WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("deleting tax pot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTaxPotNotFound
	}
	return nil
}

func (r *pgTaxPotRepo) ListByBusiness(ctx context.Context, businessID string) ([]domain.TaxPot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, business_id, pot_id, tax_type, auto_sweep_percent, due_date, notes, created_at, updated_at
		FROM tax_pots WHERE business_id = $1 ORDER BY tax_type`, businessID)
	if err != nil {
		return nil, fmt.Errorf("listing tax pots: %w", err)
	}
	defer rows.Close()
	var result []domain.TaxPot
	for rows.Next() {
		var tp domain.TaxPot
		if err := rows.Scan(&tp.ID, &tp.BusinessID, &tp.PotID, &tp.TaxType,
			&tp.AutoSweepPercent, &tp.DueDate, &tp.Notes, &tp.CreatedAt, &tp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning tax pot: %w", err)
		}
		result = append(result, tp)
	}
	return result, rows.Err()
}

func (r *pgTaxPotRepo) GetByBusinessAndType(ctx context.Context, businessID string, taxType domain.TaxType) (*domain.TaxPot, error) {
	var tp domain.TaxPot
	err := r.db.QueryRow(ctx,
		`SELECT id, business_id, pot_id, tax_type, auto_sweep_percent, due_date, notes, created_at, updated_at
		FROM tax_pots WHERE business_id = $1 AND tax_type = $2`, businessID, taxType).Scan(
		&tp.ID, &tp.BusinessID, &tp.PotID, &tp.TaxType,
		&tp.AutoSweepPercent, &tp.DueDate, &tp.Notes, &tp.CreatedAt, &tp.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrTaxPotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting tax pot by type: %w", err)
	}
	return &tp, nil
}

func (r *pgTaxPotRepo) ListWithAutoSweep(ctx context.Context, businessID string) ([]domain.TaxPot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, business_id, pot_id, tax_type, auto_sweep_percent, due_date, notes, created_at, updated_at
		FROM tax_pots WHERE business_id = $1 AND auto_sweep_percent IS NOT NULL AND auto_sweep_percent > 0
		ORDER BY tax_type`, businessID)
	if err != nil {
		return nil, fmt.Errorf("listing auto-sweep pots: %w", err)
	}
	defer rows.Close()
	var result []domain.TaxPot
	for rows.Next() {
		var tp domain.TaxPot
		if err := rows.Scan(&tp.ID, &tp.BusinessID, &tp.PotID, &tp.TaxType,
			&tp.AutoSweepPercent, &tp.DueDate, &tp.Notes, &tp.CreatedAt, &tp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning auto-sweep pot: %w", err)
		}
		result = append(result, tp)
	}
	return result, rows.Err()
}

var _ TaxPotRepository = (*pgTaxPotRepo)(nil)
