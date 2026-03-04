package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type TransactionCategoryRepository interface {
	Create(ctx context.Context, cat *domain.TransactionCategory) error
	GetByID(ctx context.Context, id string) (*domain.TransactionCategory, error)
	Update(ctx context.Context, cat *domain.TransactionCategory) error
	Delete(ctx context.Context, id string) error
	ListByBusiness(ctx context.Context, businessID string) ([]domain.TransactionCategory, error)
}

type TransactionLabelRepository interface {
	Create(ctx context.Context, label *domain.TransactionLabel) error
	GetByTransactionID(ctx context.Context, txID string) (*domain.TransactionLabel, error)
	Update(ctx context.Context, label *domain.TransactionLabel) error
	Delete(ctx context.Context, txID string) error
	ListLabeled(ctx context.Context, businessID string, categoryID *string, taxDeductible *bool, limit, offset int) ([]domain.TransactionLabel, error)
	TaxSummary(ctx context.Context, businessID string) ([]TaxSummaryRow, error)
}

type TaxSummaryRow struct {
	CategoryName string `json:"categoryName"`
	TotalCents   int64  `json:"totalCents"`
	Count        int    `json:"count"`
}

// --- Category Implementation ---

type pgTxCategoryRepo struct{ db DBTX }

func NewTransactionCategoryRepository(db DBTX) TransactionCategoryRepository {
	return &pgTxCategoryRepo{db: db}
}

func (r *pgTxCategoryRepo) Create(ctx context.Context, cat *domain.TransactionCategory) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO transaction_categories (business_id, name, color, icon, is_system)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
		cat.BusinessID, cat.Name, cat.Color, cat.Icon, cat.IsSystem,
	).Scan(&cat.ID, &cat.CreatedAt)
}

func (r *pgTxCategoryRepo) GetByID(ctx context.Context, id string) (*domain.TransactionCategory, error) {
	var cat domain.TransactionCategory
	err := r.db.QueryRow(ctx,
		`SELECT id, business_id, name, color, icon, is_system, created_at
		FROM transaction_categories WHERE id = $1`, id).Scan(
		&cat.ID, &cat.BusinessID, &cat.Name, &cat.Color, &cat.Icon, &cat.IsSystem, &cat.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting category: %w", err)
	}
	return &cat, nil
}

func (r *pgTxCategoryRepo) Update(ctx context.Context, cat *domain.TransactionCategory) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE transaction_categories SET name=$2, color=$3, icon=$4
		WHERE id=$1 AND NOT is_system`, cat.ID, cat.Name, cat.Color, cat.Icon)
	if err != nil {
		return fmt.Errorf("updating category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

func (r *pgTxCategoryRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM transaction_categories WHERE id=$1 AND NOT is_system`, id)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

func (r *pgTxCategoryRepo) ListByBusiness(ctx context.Context, businessID string) ([]domain.TransactionCategory, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, business_id, name, color, icon, is_system, created_at
		FROM transaction_categories
		WHERE business_id = $1 OR business_id IS NULL
		ORDER BY is_system DESC, name ASC`, businessID)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer rows.Close()
	var cats []domain.TransactionCategory
	for rows.Next() {
		var c domain.TransactionCategory
		if err := rows.Scan(&c.ID, &c.BusinessID, &c.Name, &c.Color, &c.Icon, &c.IsSystem, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

var _ TransactionCategoryRepository = (*pgTxCategoryRepo)(nil)

// --- Label Implementation ---

type pgTxLabelRepo struct{ db DBTX }

func NewTransactionLabelRepository(db DBTX) TransactionLabelRepository {
	return &pgTxLabelRepo{db: db}
}

func (r *pgTxLabelRepo) Create(ctx context.Context, l *domain.TransactionLabel) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO transaction_labels (transaction_id, category_id, custom_label, notes, tagged_by, tax_deductible)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, updated_at`,
		l.TransactionID, l.CategoryID, l.CustomLabel, l.Notes, l.TaggedBy, l.TaxDeductible,
	).Scan(&l.ID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *pgTxLabelRepo) GetByTransactionID(ctx context.Context, txID string) (*domain.TransactionLabel, error) {
	var l domain.TransactionLabel
	err := r.db.QueryRow(ctx,
		`SELECT id, transaction_id, category_id, custom_label, notes, tagged_by, tax_deductible, created_at, updated_at
		FROM transaction_labels WHERE transaction_id = $1`, txID).Scan(
		&l.ID, &l.TransactionID, &l.CategoryID, &l.CustomLabel, &l.Notes,
		&l.TaggedBy, &l.TaxDeductible, &l.CreatedAt, &l.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting label: %w", err)
	}
	return &l, nil
}

func (r *pgTxLabelRepo) Update(ctx context.Context, l *domain.TransactionLabel) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE transaction_labels SET category_id=$2, custom_label=$3, notes=$4, tax_deductible=$5, updated_at=NOW()
		WHERE transaction_id=$1`, l.TransactionID, l.CategoryID, l.CustomLabel, l.Notes, l.TaxDeductible)
	if err != nil {
		return fmt.Errorf("updating label: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *pgTxLabelRepo) Delete(ctx context.Context, txID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM transaction_labels WHERE transaction_id=$1`, txID)
	if err != nil {
		return fmt.Errorf("deleting label: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *pgTxLabelRepo) ListLabeled(ctx context.Context, businessID string, categoryID *string, taxDeductible *bool, limit, offset int) ([]domain.TransactionLabel, error) {
	query := `
		SELECT tl.id, tl.transaction_id, tl.category_id, tl.custom_label, tl.notes, tl.tagged_by, tl.tax_deductible, tl.created_at, tl.updated_at
		FROM transaction_labels tl
		JOIN transaction_receipts tr ON tr.id = tl.transaction_id
		WHERE tr.user_id IN (SELECT owner_user_id FROM businesses WHERE id = $1)
			AND ($2::uuid IS NULL OR tl.category_id = $2)
			AND ($3::boolean IS NULL OR tl.tax_deductible = $3)
		ORDER BY tl.created_at DESC LIMIT $4 OFFSET $5`
	rows, err := r.db.Query(ctx, query, businessID, categoryID, taxDeductible, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing labeled transactions: %w", err)
	}
	defer rows.Close()
	var result []domain.TransactionLabel
	for rows.Next() {
		var l domain.TransactionLabel
		if err := rows.Scan(&l.ID, &l.TransactionID, &l.CategoryID, &l.CustomLabel,
			&l.Notes, &l.TaggedBy, &l.TaxDeductible, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning label: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *pgTxLabelRepo) TaxSummary(ctx context.Context, businessID string) ([]TaxSummaryRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(tc.name, 'Uncategorized'), COALESCE(SUM(tr.amount_cents), 0), COUNT(*)
		FROM transaction_labels tl
		JOIN transaction_receipts tr ON tr.id = tl.transaction_id
		LEFT JOIN transaction_categories tc ON tc.id = tl.category_id
		WHERE tl.tax_deductible
			AND tr.user_id IN (SELECT owner_user_id FROM businesses WHERE id = $1)
		GROUP BY tc.name
		ORDER BY SUM(tr.amount_cents) DESC`, businessID)
	if err != nil {
		return nil, fmt.Errorf("tax summary: %w", err)
	}
	defer rows.Close()
	var result []TaxSummaryRow
	for rows.Next() {
		var row TaxSummaryRow
		if err := rows.Scan(&row.CategoryName, &row.TotalCents, &row.Count); err != nil {
			return nil, fmt.Errorf("scanning tax summary: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

var _ TransactionLabelRepository = (*pgTxLabelRepo)(nil)
