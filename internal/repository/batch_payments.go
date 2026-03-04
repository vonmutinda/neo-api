package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/jackc/pgx/v5"
)

type BatchPaymentRepository interface {
	CreateBatch(ctx context.Context, batch *domain.BatchPayment) error
	GetBatch(ctx context.Context, id string) (*domain.BatchPayment, error)
	UpdateBatchStatus(ctx context.Context, id string, status domain.BatchStatus) error
	ApproveBatch(ctx context.Context, id, approverID string) error
	ListByBusiness(ctx context.Context, businessID string, limit, offset int) ([]domain.BatchPayment, error)

	CreateItem(ctx context.Context, item *domain.BatchPaymentItem) error
	ListItems(ctx context.Context, batchID string) ([]domain.BatchPaymentItem, error)
	UpdateItemStatus(ctx context.Context, id string, status domain.BatchItemStatus, txID, errMsg *string) error
	CreateItems(ctx context.Context, items []domain.BatchPaymentItem) error
}

type pgBatchPaymentRepo struct{ db DBTX }

func NewBatchPaymentRepository(db DBTX) BatchPaymentRepository {
	return &pgBatchPaymentRepo{db: db}
}

func (r *pgBatchPaymentRepo) CreateBatch(ctx context.Context, b *domain.BatchPayment) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO batch_payments (business_id, name, total_cents, currency_code, item_count, status, initiated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at, updated_at`,
		b.BusinessID, b.Name, b.TotalCents, b.CurrencyCode, b.ItemCount, b.Status, b.InitiatedBy,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
}

func (r *pgBatchPaymentRepo) GetBatch(ctx context.Context, id string) (*domain.BatchPayment, error) {
	var b domain.BatchPayment
	err := r.db.QueryRow(ctx,
		`SELECT id, business_id, name, total_cents, currency_code, item_count, status,
			initiated_by, approved_by, approved_at, processed_at, completed_at, created_at, updated_at
		FROM batch_payments WHERE id = $1`, id).Scan(
		&b.ID, &b.BusinessID, &b.Name, &b.TotalCents, &b.CurrencyCode, &b.ItemCount, &b.Status,
		&b.InitiatedBy, &b.ApprovedBy, &b.ApprovedAt, &b.ProcessedAt, &b.CompletedAt,
		&b.CreatedAt, &b.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting batch: %w", err)
	}
	return &b, nil
}

func (r *pgBatchPaymentRepo) UpdateBatchStatus(ctx context.Context, id string, status domain.BatchStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE batch_payments SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return fmt.Errorf("updating batch status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBatchNotFound
	}
	return nil
}

func (r *pgBatchPaymentRepo) ApproveBatch(ctx context.Context, id, approverID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE batch_payments SET status='approved', approved_by=$2, approved_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND status='draft'`, id, approverID)
	if err != nil {
		return fmt.Errorf("approving batch: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBatchNotDraft
	}
	return nil
}

func (r *pgBatchPaymentRepo) ListByBusiness(ctx context.Context, businessID string, limit, offset int) ([]domain.BatchPayment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, business_id, name, total_cents, currency_code, item_count, status,
			initiated_by, approved_by, approved_at, processed_at, completed_at, created_at, updated_at
		FROM batch_payments WHERE business_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, businessID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing batches: %w", err)
	}
	defer rows.Close()
	var result []domain.BatchPayment
	for rows.Next() {
		var b domain.BatchPayment
		if err := rows.Scan(
			&b.ID, &b.BusinessID, &b.Name, &b.TotalCents, &b.CurrencyCode, &b.ItemCount, &b.Status,
			&b.InitiatedBy, &b.ApprovedBy, &b.ApprovedAt, &b.ProcessedAt, &b.CompletedAt,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning batch: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (r *pgBatchPaymentRepo) CreateItem(ctx context.Context, item *domain.BatchPaymentItem) error {
	var rpPhone *string
	var rpCC *string
	if item.RecipientPhone != nil {
		e := item.RecipientPhone.E164()
		rpPhone = &e
		cc := item.RecipientPhone.CountryCode
		rpCC = &cc
	}
	return r.db.QueryRow(ctx,
		`INSERT INTO batch_payment_items (batch_id, recipient_name, recipient_phone, recipient_country_code, recipient_bank,
			recipient_account, amount_cents, narration, category_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, created_at, updated_at`,
		item.BatchID, item.RecipientName, rpPhone, rpCC, item.RecipientBank,
		item.RecipientAccount, item.AmountCents, item.Narration, item.CategoryID,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *pgBatchPaymentRepo) CreateItems(ctx context.Context, items []domain.BatchPaymentItem) error {
	for i := range items {
		if err := r.CreateItem(ctx, &items[i]); err != nil {
			return fmt.Errorf("creating item %d: %w", i, err)
		}
	}
	return nil
}

func (r *pgBatchPaymentRepo) ListItems(ctx context.Context, batchID string) ([]domain.BatchPaymentItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, batch_id, recipient_name, recipient_phone, recipient_country_code, recipient_bank, recipient_account,
			amount_cents, narration, category_id, status, transaction_id, error_message, created_at, updated_at
		FROM batch_payment_items WHERE batch_id = $1 ORDER BY created_at`, batchID)
	if err != nil {
		return nil, fmt.Errorf("listing batch items: %w", err)
	}
	defer rows.Close()
	var result []domain.BatchPaymentItem
	for rows.Next() {
		var item domain.BatchPaymentItem
		var rpPhone *string
		var rpCC *string
		if err := rows.Scan(
			&item.ID, &item.BatchID, &item.RecipientName, &rpPhone, &rpCC,
			&item.RecipientBank, &item.RecipientAccount, &item.AmountCents, &item.Narration,
			&item.CategoryID, &item.Status, &item.TransactionID, &item.ErrorMessage,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning batch item: %w", err)
		}
		if rpPhone != nil {
			p, _ := phone.Parse(*rpPhone)
			item.RecipientPhone = &p
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *pgBatchPaymentRepo) UpdateItemStatus(ctx context.Context, id string, status domain.BatchItemStatus, txID, errMsg *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE batch_payment_items SET status=$2, transaction_id=$3, error_message=$4, updated_at=NOW()
		WHERE id=$1`, id, status, txID, errMsg)
	if err != nil {
		return fmt.Errorf("updating batch item status: %w", err)
	}
	return nil
}

var _ BatchPaymentRepository = (*pgBatchPaymentRepo)(nil)
