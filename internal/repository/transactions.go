package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/jackc/pgx/v5"
)

type TransactionReceiptRepository interface {
	Create(ctx context.Context, receipt *domain.TransactionReceipt) error
	GetByID(ctx context.Context, id string) (*domain.TransactionReceipt, error)
	GetByEthSwitchReference(ctx context.Context, ref string) (*domain.TransactionReceipt, error)
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]domain.TransactionReceipt, error)
	ListByUserIDFiltered(ctx context.Context, userID string, currency *string, limit, offset int) ([]domain.TransactionReceipt, error)
	UpdateStatus(ctx context.Context, id string, status domain.ReceiptStatus) error
}

type pgReceiptRepo struct{ db DBTX }

func NewTransactionReceiptRepository(db DBTX) TransactionReceiptRepository {
	return &pgReceiptRepo{db: db}
}

func (r *pgReceiptRepo) Create(ctx context.Context, rec *domain.TransactionReceipt) error {
	var cpPhone *string
	var cpCC *string
	if rec.CounterpartyPhone != nil {
		e := rec.CounterpartyPhone.E164()
		cpPhone = &e
		cc := rec.CounterpartyPhone.CountryCode
		cpCC = &cc
	}
	query := `
		INSERT INTO transaction_receipts (
			user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
			type, status, amount_cents, currency,
			counterparty_name, counterparty_phone, counterparty_country_code, counterparty_institution, narration,
			metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		rec.UserID, rec.LedgerTransactionID, rec.EthSwitchReference, rec.IdempotencyKey,
		rec.Type, rec.Status, rec.AmountCents, rec.Currency,
		rec.CounterpartyName, cpPhone, cpCC, rec.CounterpartyInstitution, rec.Narration,
		rec.Metadata,
	).Scan(&rec.ID, &rec.CreatedAt, &rec.UpdatedAt)
}

func (r *pgReceiptRepo) GetByID(ctx context.Context, id string) (*domain.TransactionReceipt, error) {
	var rec domain.TransactionReceipt
	var cpPhone *string
	var cpCC *string
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
			type, status, amount_cents, currency,
			counterparty_name, counterparty_phone, counterparty_country_code, counterparty_institution, narration,
			fee_cents, fee_breakdown, metadata,
			created_at, updated_at
		FROM transaction_receipts WHERE id = $1`, id).Scan(
		&rec.ID, &rec.UserID, &rec.LedgerTransactionID, &rec.EthSwitchReference, &rec.IdempotencyKey,
		&rec.Type, &rec.Status, &rec.AmountCents, &rec.Currency,
		&rec.CounterpartyName, &cpPhone, &cpCC, &rec.CounterpartyInstitution, &rec.Narration,
		&rec.FeeCents, &rec.FeeBreakdown, &rec.Metadata,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting receipt: %w", err)
	}
	if cpPhone != nil {
		p, _ := phone.Parse(*cpPhone)
		rec.CounterpartyPhone = &p
	}
	return &rec, nil
}

func (r *pgReceiptRepo) GetByEthSwitchReference(ctx context.Context, ref string) (*domain.TransactionReceipt, error) {
	var rec domain.TransactionReceipt
	var cpPhone *string
	var cpCC *string
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
			type, status, amount_cents, currency,
			counterparty_name, counterparty_phone, counterparty_country_code, counterparty_institution, narration,
			fee_cents, fee_breakdown, metadata,
			created_at, updated_at
		FROM transaction_receipts WHERE ethswitch_reference = $1`, ref).Scan(
		&rec.ID, &rec.UserID, &rec.LedgerTransactionID, &rec.EthSwitchReference, &rec.IdempotencyKey,
		&rec.Type, &rec.Status, &rec.AmountCents, &rec.Currency,
		&rec.CounterpartyName, &cpPhone, &cpCC, &rec.CounterpartyInstitution, &rec.Narration,
		&rec.FeeCents, &rec.FeeBreakdown, &rec.Metadata,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting receipt by ethswitch ref: %w", err)
	}
	if cpPhone != nil {
		p, _ := phone.Parse(*cpPhone)
		rec.CounterpartyPhone = &p
	}
	return &rec, nil
}

func (r *pgReceiptRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]domain.TransactionReceipt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
			type, status, amount_cents, currency,
			counterparty_name, counterparty_phone, counterparty_country_code, counterparty_institution, narration,
			fee_cents, fee_breakdown, metadata,
			created_at, updated_at
		FROM transaction_receipts WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing receipts: %w", err)
	}
	defer rows.Close()
	var result []domain.TransactionReceipt
	for rows.Next() {
		var rec domain.TransactionReceipt
		var cpPhone *string
		var cpCC *string
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.LedgerTransactionID, &rec.EthSwitchReference, &rec.IdempotencyKey,
			&rec.Type, &rec.Status, &rec.AmountCents, &rec.Currency,
			&rec.CounterpartyName, &cpPhone, &cpCC, &rec.CounterpartyInstitution, &rec.Narration,
			&rec.FeeCents, &rec.FeeBreakdown, &rec.Metadata,
			&rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning receipt: %w", err)
		}
		if cpPhone != nil {
			p, _ := phone.Parse(*cpPhone)
			rec.CounterpartyPhone = &p
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

func (r *pgReceiptRepo) ListByUserIDFiltered(ctx context.Context, userID string, currency *string, limit, offset int) ([]domain.TransactionReceipt, error) {
	const cols = `id, user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
		type, status, amount_cents, currency,
		counterparty_name, counterparty_phone, counterparty_country_code, counterparty_institution, narration,
		fee_cents, fee_breakdown, metadata,
		created_at, updated_at`

	var rows pgx.Rows
	var err error
	if currency != nil && *currency != "" {
		rows, err = r.db.Query(ctx,
			`SELECT `+cols+` FROM transaction_receipts WHERE user_id = $1 AND currency = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
			userID, *currency, limit, offset)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+cols+` FROM transaction_receipts WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			userID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("listing receipts: %w", err)
	}
	defer rows.Close()

	var result []domain.TransactionReceipt
	for rows.Next() {
		var rec domain.TransactionReceipt
		var cpPhone *string
		var cpCC *string
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.LedgerTransactionID, &rec.EthSwitchReference, &rec.IdempotencyKey,
			&rec.Type, &rec.Status, &rec.AmountCents, &rec.Currency,
			&rec.CounterpartyName, &cpPhone, &cpCC, &rec.CounterpartyInstitution, &rec.Narration,
			&rec.FeeCents, &rec.FeeBreakdown, &rec.Metadata,
			&rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning receipt: %w", err)
		}
		if cpPhone != nil {
			p, _ := phone.Parse(*cpPhone)
			rec.CounterpartyPhone = &p
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

func (r *pgReceiptRepo) UpdateStatus(ctx context.Context, id string, status domain.ReceiptStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE transaction_receipts SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

var _ TransactionReceiptRepository = (*pgReceiptRepo)(nil)
