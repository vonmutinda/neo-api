package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
)

type KYCRepository interface {
	Create(ctx context.Context, v *domain.KYCVerification) error
	GetByUserID(ctx context.Context, userID string) ([]domain.KYCVerification, error)
	UpdateStatus(ctx context.Context, id string, status domain.KYCVerificationStatus) error
}

type pgKYCRepo struct{ db DBTX }

func NewKYCRepository(db DBTX) KYCRepository { return &pgKYCRepo{db: db} }

func (r *pgKYCRepo) Create(ctx context.Context, v *domain.KYCVerification) error {
	query := `
		INSERT INTO kyc_verifications (user_id, fayda_fin, fayda_transaction_id, status, raw_response_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		v.UserID, v.FaydaFIN, v.FaydaTransactionID, v.Status, v.RawResponseHash,
	).Scan(&v.ID, &v.CreatedAt)
}

func (r *pgKYCRepo) GetByUserID(ctx context.Context, userID string) ([]domain.KYCVerification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, fayda_fin, fayda_transaction_id, status, verified_at, fayda_expiry_date, raw_response_hash, created_at
		 FROM kyc_verifications WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing kyc verifications: %w", err)
	}
	defer rows.Close()

	var result []domain.KYCVerification
	for rows.Next() {
		var v domain.KYCVerification
		if err := rows.Scan(&v.ID, &v.UserID, &v.FaydaFIN, &v.FaydaTransactionID, &v.Status, &v.VerifiedAt, &v.FaydaExpiryDate, &v.RawResponseHash, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning kyc verification: %w", err)
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *pgKYCRepo) UpdateStatus(ctx context.Context, id string, status domain.KYCVerificationStatus) error {
	if status == domain.KYCStatusVerified {
		_, err := r.db.Exec(ctx, `UPDATE kyc_verifications SET status = $2, verified_at = NOW() WHERE id = $1`, id, status)
		if err != nil {
			return fmt.Errorf("updating kyc status: %w", err)
		}
		return nil
	}
	_, err := r.db.Exec(ctx, `UPDATE kyc_verifications SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("updating kyc status: %w", err)
	}
	return nil
}

var _ KYCRepository = (*pgKYCRepo)(nil)
