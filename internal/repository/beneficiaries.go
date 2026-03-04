package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type BeneficiaryRepository interface {
	Create(ctx context.Context, b *domain.Beneficiary) error
	GetByID(ctx context.Context, id string) (*domain.Beneficiary, error)
	ListByUser(ctx context.Context, userID string) ([]domain.Beneficiary, error)
	SoftDelete(ctx context.Context, id, userID string) error
}

type pgBeneficiaryRepo struct{ db DBTX }

func NewBeneficiaryRepository(db DBTX) BeneficiaryRepository {
	return &pgBeneficiaryRepo{db: db}
}

func (r *pgBeneficiaryRepo) Create(ctx context.Context, b *domain.Beneficiary) error {
	query := `INSERT INTO beneficiaries (user_id, full_name, relationship, document_url, is_verified)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		b.UserID, b.FullName, b.Relationship, b.DocumentURL, b.IsVerified,
	).Scan(&b.ID, &b.CreatedAt)
}

func (r *pgBeneficiaryRepo) GetByID(ctx context.Context, id string) (*domain.Beneficiary, error) {
	var b domain.Beneficiary
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, full_name, relationship, document_url, is_verified, created_at, deleted_at
		 FROM beneficiaries WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&b.ID, &b.UserID, &b.FullName, &b.Relationship, &b.DocumentURL, &b.IsVerified, &b.CreatedAt, &b.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrBeneficiaryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting beneficiary: %w", err)
	}
	return &b, nil
}

func (r *pgBeneficiaryRepo) ListByUser(ctx context.Context, userID string) ([]domain.Beneficiary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, full_name, relationship, document_url, is_verified, created_at, deleted_at
		 FROM beneficiaries WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing beneficiaries: %w", err)
	}
	defer rows.Close()

	var result []domain.Beneficiary
	for rows.Next() {
		var b domain.Beneficiary
		if err := rows.Scan(&b.ID, &b.UserID, &b.FullName, &b.Relationship, &b.DocumentURL, &b.IsVerified, &b.CreatedAt, &b.DeletedAt); err != nil {
			return nil, fmt.Errorf("scanning beneficiary: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (r *pgBeneficiaryRepo) SoftDelete(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE beneficiaries SET deleted_at = NOW() WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		id, userID)
	if err != nil {
		return fmt.Errorf("soft-deleting beneficiary: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBeneficiaryNotFound
	}
	return nil
}

var _ BeneficiaryRepository = (*pgBeneficiaryRepo)(nil)
