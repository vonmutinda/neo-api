package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type PendingTransferRepository interface {
	Create(ctx context.Context, pt *domain.PendingTransfer) error
	GetByID(ctx context.Context, id string) (*domain.PendingTransfer, error)
	Approve(ctx context.Context, id, approverID string) error
	Reject(ctx context.Context, id, rejectorID, reason string) error
	ListPendingByBusiness(ctx context.Context, businessID string, limit, offset int) ([]domain.PendingTransfer, error)
	CountPendingByBusiness(ctx context.Context, businessID string) (int, error)
	ExpireOlderThan(ctx context.Context) (int64, error)
}

type pgPendingTransferRepo struct{ db DBTX }

func NewPendingTransferRepository(db DBTX) PendingTransferRepository {
	return &pgPendingTransferRepo{db: db}
}

func (r *pgPendingTransferRepo) Create(ctx context.Context, pt *domain.PendingTransfer) error {
	query := `
		INSERT INTO pending_transfers (business_id, initiated_by, transfer_type,
			amount_cents, currency_code, recipient_info, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, status, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		pt.BusinessID, pt.InitiatedBy, pt.TransferType,
		pt.AmountCents, pt.CurrencyCode, pt.RecipientInfo, pt.ExpiresAt,
	).Scan(&pt.ID, &pt.Status, &pt.CreatedAt, &pt.UpdatedAt)
}

func (r *pgPendingTransferRepo) GetByID(ctx context.Context, id string) (*domain.PendingTransfer, error) {
	return r.scan(ctx, `
		SELECT id, business_id, initiated_by, transfer_type, amount_cents, currency_code,
			recipient_info, status, reason, approved_by, approved_at, rejected_by, rejected_at,
			expires_at, created_at, updated_at
		FROM pending_transfers WHERE id = $1`, id)
}

func (r *pgPendingTransferRepo) Approve(ctx context.Context, id, approverID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE pending_transfers
		SET status = 'approved', approved_by = $2, approved_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending'`, id, approverID)
	if err != nil {
		return fmt.Errorf("approving transfer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPendingTransferNotFound
	}
	return nil
}

func (r *pgPendingTransferRepo) Reject(ctx context.Context, id, rejectorID, reason string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE pending_transfers
		SET status = 'rejected', rejected_by = $2, rejected_at = NOW(), reason = $3, updated_at = NOW()
		WHERE id = $1 AND status = 'pending'`, id, rejectorID, reason)
	if err != nil {
		return fmt.Errorf("rejecting transfer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPendingTransferNotFound
	}
	return nil
}

func (r *pgPendingTransferRepo) ListPendingByBusiness(ctx context.Context, businessID string, limit, offset int) ([]domain.PendingTransfer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, business_id, initiated_by, transfer_type, amount_cents, currency_code,
			recipient_info, status, reason, approved_by, approved_at, rejected_by, rejected_at,
			expires_at, created_at, updated_at
		FROM pending_transfers
		WHERE business_id = $1 AND status = 'pending'
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, businessID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing pending transfers: %w", err)
	}
	defer rows.Close()
	var result []domain.PendingTransfer
	for rows.Next() {
		var pt domain.PendingTransfer
		if err := rows.Scan(
			&pt.ID, &pt.BusinessID, &pt.InitiatedBy, &pt.TransferType,
			&pt.AmountCents, &pt.CurrencyCode, &pt.RecipientInfo,
			&pt.Status, &pt.Reason, &pt.ApprovedBy, &pt.ApprovedAt,
			&pt.RejectedBy, &pt.RejectedAt, &pt.ExpiresAt,
			&pt.CreatedAt, &pt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning pending transfer row: %w", err)
		}
		result = append(result, pt)
	}
	return result, rows.Err()
}

func (r *pgPendingTransferRepo) CountPendingByBusiness(ctx context.Context, businessID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM pending_transfers WHERE business_id = $1 AND status = 'pending'`,
		businessID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending transfers: %w", err)
	}
	return count, nil
}

func (r *pgPendingTransferRepo) ExpireOlderThan(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE pending_transfers SET status = 'expired', updated_at = NOW()
		WHERE status = 'pending' AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("expiring pending transfers: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *pgPendingTransferRepo) scan(ctx context.Context, query string, args ...any) (*domain.PendingTransfer, error) {
	var pt domain.PendingTransfer
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&pt.ID, &pt.BusinessID, &pt.InitiatedBy, &pt.TransferType,
		&pt.AmountCents, &pt.CurrencyCode, &pt.RecipientInfo,
		&pt.Status, &pt.Reason, &pt.ApprovedBy, &pt.ApprovedAt,
		&pt.RejectedBy, &pt.RejectedAt, &pt.ExpiresAt,
		&pt.CreatedAt, &pt.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrPendingTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning pending transfer: %w", err)
	}
	return &pt, nil
}

var _ PendingTransferRepository = (*pgPendingTransferRepo)(nil)
