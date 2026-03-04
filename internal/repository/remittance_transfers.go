package repository

import (
	"context"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type RemittanceTransferRepository interface {
	Create(ctx context.Context, rt *domain.RemittanceTransfer) error
	GetByID(ctx context.Context, id string) (*domain.RemittanceTransfer, error)
	UpdateStatus(ctx context.Context, id string, status domain.RemittanceTransferStatus, providerTransferID *string, failureReason *string) error
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]domain.RemittanceTransfer, error)
	ListPending(ctx context.Context) ([]domain.RemittanceTransfer, error)
}

type pgRemittanceTransferRepo struct {
	db DBTX
}

func NewRemittanceTransferRepository(db DBTX) RemittanceTransferRepository {
	return &pgRemittanceTransferRepo{db: db}
}

func (r *pgRemittanceTransferRepo) Create(ctx context.Context, rt *domain.RemittanceTransfer) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO remittance_transfers (user_id, provider_id, provider_transfer_id, quote_id,
			source_currency, target_currency, source_amount_cents, target_amount_cents,
			exchange_rate, our_fee_cents, provider_fee_cents, total_fee_cents,
			status, recipient_name, recipient_country, hold_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, created_at, updated_at`,
		rt.UserID, rt.ProviderID, rt.ProviderTransferID, rt.QuoteID,
		rt.SourceCurrency, rt.TargetCurrency, rt.SourceAmountCents, rt.TargetAmountCents,
		rt.ExchangeRate, rt.OurFeeCents, rt.ProviderFeeCents, rt.TotalFeeCents,
		rt.Status, rt.RecipientName, rt.RecipientCountry, rt.HoldID,
	).Scan(&rt.ID, &rt.CreatedAt, &rt.UpdatedAt)
}

func (r *pgRemittanceTransferRepo) GetByID(ctx context.Context, id string) (*domain.RemittanceTransfer, error) {
	var rt domain.RemittanceTransfer
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, provider_id, provider_transfer_id, quote_id,
			source_currency, target_currency, source_amount_cents, target_amount_cents,
			exchange_rate, our_fee_cents, provider_fee_cents, total_fee_cents,
			status, recipient_name, recipient_country, hold_id, failure_reason,
			created_at, updated_at
		FROM remittance_transfers WHERE id = $1`, id,
	).Scan(&rt.ID, &rt.UserID, &rt.ProviderID, &rt.ProviderTransferID, &rt.QuoteID,
		&rt.SourceCurrency, &rt.TargetCurrency, &rt.SourceAmountCents, &rt.TargetAmountCents,
		&rt.ExchangeRate, &rt.OurFeeCents, &rt.ProviderFeeCents, &rt.TotalFeeCents,
		&rt.Status, &rt.RecipientName, &rt.RecipientCountry, &rt.HoldID, &rt.FailureReason,
		&rt.CreatedAt, &rt.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRemittanceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *pgRemittanceTransferRepo) UpdateStatus(ctx context.Context, id string, status domain.RemittanceTransferStatus, providerTransferID *string, failureReason *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE remittance_transfers SET status=$2, provider_transfer_id=COALESCE($3, provider_transfer_id),
			failure_reason=COALESCE($4, failure_reason), updated_at=NOW()
		WHERE id=$1`, id, status, providerTransferID, failureReason)
	return err
}

func (r *pgRemittanceTransferRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]domain.RemittanceTransfer, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, provider_id, provider_transfer_id, quote_id,
			source_currency, target_currency, source_amount_cents, target_amount_cents,
			exchange_rate, our_fee_cents, provider_fee_cents, total_fee_cents,
			status, recipient_name, recipient_country, hold_id, failure_reason,
			created_at, updated_at
		FROM remittance_transfers WHERE user_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRemittanceTransfers(rows)
}

func (r *pgRemittanceTransferRepo) ListPending(ctx context.Context) ([]domain.RemittanceTransfer, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, provider_id, provider_transfer_id, quote_id,
			source_currency, target_currency, source_amount_cents, target_amount_cents,
			exchange_rate, our_fee_cents, provider_fee_cents, total_fee_cents,
			status, recipient_name, recipient_country, hold_id, failure_reason,
			created_at, updated_at
		FROM remittance_transfers
		WHERE status NOT IN ('delivered','failed','cancelled','refunded')
		ORDER BY updated_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRemittanceTransfers(rows)
}

func scanRemittanceTransfers(rows pgx.Rows) ([]domain.RemittanceTransfer, error) {
	var result []domain.RemittanceTransfer
	for rows.Next() {
		var rt domain.RemittanceTransfer
		if err := rows.Scan(&rt.ID, &rt.UserID, &rt.ProviderID, &rt.ProviderTransferID, &rt.QuoteID,
			&rt.SourceCurrency, &rt.TargetCurrency, &rt.SourceAmountCents, &rt.TargetAmountCents,
			&rt.ExchangeRate, &rt.OurFeeCents, &rt.ProviderFeeCents, &rt.TotalFeeCents,
			&rt.Status, &rt.RecipientName, &rt.RecipientCountry, &rt.HoldID, &rt.FailureReason,
			&rt.CreatedAt, &rt.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, rt)
	}
	return result, rows.Err()
}
