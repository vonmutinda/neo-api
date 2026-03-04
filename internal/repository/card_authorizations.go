package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type CardAuthorizationRepository interface {
	Create(ctx context.Context, auth *domain.CardAuthorization) error
	GetByID(ctx context.Context, id string) (*domain.CardAuthorization, error)
	GetByRRN(ctx context.Context, rrn string) (*domain.CardAuthorization, error)
	UpdateStatus(ctx context.Context, id string, status domain.AuthStatus) error
	Settle(ctx context.Context, id string, settlementAmountCents int64) error
	Reverse(ctx context.Context, id string) error
	ListPendingExpired(ctx context.Context) ([]domain.CardAuthorization, error)
	SumApprovedToday(ctx context.Context, cardID string) (int64, error)
	SumApprovedThisMonth(ctx context.Context, cardID string) (int64, error)

	// SumInternationalThisMonth returns total auth_amount_cents for a user's cards
	// where merchant_currency != 'ETB' this calendar month (for regulatory cap enforcement).
	SumInternationalThisMonth(ctx context.Context, userID string) (int64, error)
}

type pgCardAuthRepo struct{ db DBTX }

func NewCardAuthorizationRepository(db DBTX) CardAuthorizationRepository {
	return &pgCardAuthRepo{db: db}
}

func (r *pgCardAuthRepo) Create(ctx context.Context, a *domain.CardAuthorization) error {
	query := `
		INSERT INTO card_authorizations (
			card_id, retrieval_reference_number, stan, auth_code,
			merchant_name, merchant_id, merchant_category_code, terminal_id, acquiring_institution,
			auth_amount_cents, currency, status, decline_reason, response_code, ledger_hold_id,
			merchant_currency, fx_rate_applied, fx_from_currency, fx_from_amount_cents)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id, authorized_at, expires_at, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		a.CardID, a.RetrievalReferenceNumber, a.STAN, a.AuthCode,
		a.MerchantName, a.MerchantID, a.MerchantCategoryCode, a.TerminalID, a.AcquiringInstitution,
		a.AuthAmountCents, a.Currency, a.Status, a.DeclineReason, a.ResponseCode, a.LedgerHoldID,
		a.MerchantCurrency, a.FXRateApplied, a.FXFromCurrency, a.FXFromAmountCents,
	).Scan(&a.ID, &a.AuthorizedAt, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt)
}

func (r *pgCardAuthRepo) GetByID(ctx context.Context, id string) (*domain.CardAuthorization, error) {
	var a domain.CardAuthorization
	err := r.db.QueryRow(ctx, `
		SELECT id, card_id, retrieval_reference_number, stan, auth_code,
			merchant_name, merchant_id, merchant_category_code, terminal_id, acquiring_institution,
			auth_amount_cents, settlement_amount_cents, currency, status,
			decline_reason, response_code, ledger_hold_id,
			authorized_at, settled_at, reversed_at, expires_at, created_at, updated_at
		FROM card_authorizations WHERE id = $1`, id).Scan(
		&a.ID, &a.CardID, &a.RetrievalReferenceNumber, &a.STAN, &a.AuthCode,
		&a.MerchantName, &a.MerchantID, &a.MerchantCategoryCode, &a.TerminalID, &a.AcquiringInstitution,
		&a.AuthAmountCents, &a.SettlementAmountCents, &a.Currency, &a.Status,
		&a.DeclineReason, &a.ResponseCode, &a.LedgerHoldID,
		&a.AuthorizedAt, &a.SettledAt, &a.ReversedAt, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting card auth: %w", err)
	}
	return &a, nil
}

func (r *pgCardAuthRepo) GetByRRN(ctx context.Context, rrn string) (*domain.CardAuthorization, error) {
	var a domain.CardAuthorization
	err := r.db.QueryRow(ctx, `
		SELECT id, card_id, retrieval_reference_number, stan, auth_code,
			merchant_name, merchant_id, merchant_category_code, terminal_id, acquiring_institution,
			auth_amount_cents, settlement_amount_cents, currency, status,
			decline_reason, response_code, ledger_hold_id,
			authorized_at, settled_at, reversed_at, expires_at, created_at, updated_at
		FROM card_authorizations WHERE retrieval_reference_number = $1`, rrn).Scan(
		&a.ID, &a.CardID, &a.RetrievalReferenceNumber, &a.STAN, &a.AuthCode,
		&a.MerchantName, &a.MerchantID, &a.MerchantCategoryCode, &a.TerminalID, &a.AcquiringInstitution,
		&a.AuthAmountCents, &a.SettlementAmountCents, &a.Currency, &a.Status,
		&a.DeclineReason, &a.ResponseCode, &a.LedgerHoldID,
		&a.AuthorizedAt, &a.SettledAt, &a.ReversedAt, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting card auth by rrn: %w", err)
	}
	return &a, nil
}

func (r *pgCardAuthRepo) UpdateStatus(ctx context.Context, id string, status domain.AuthStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE card_authorizations SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (r *pgCardAuthRepo) Settle(ctx context.Context, id string, settlementAmountCents int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE card_authorizations SET status='cleared', settlement_amount_cents=$2, settled_at=NOW(), updated_at=NOW() WHERE id=$1`,
		id, settlementAmountCents)
	return err
}

func (r *pgCardAuthRepo) Reverse(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE card_authorizations SET status='reversed', reversed_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *pgCardAuthRepo) ListPendingExpired(ctx context.Context) ([]domain.CardAuthorization, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, card_id, retrieval_reference_number, stan, auth_code,
			merchant_name, merchant_id, merchant_category_code, terminal_id, acquiring_institution,
			auth_amount_cents, settlement_amount_cents, currency, status,
			decline_reason, response_code, ledger_hold_id,
			authorized_at, settled_at, reversed_at, expires_at, created_at, updated_at
		 FROM card_authorizations WHERE status = 'approved' AND expires_at < NOW()`)
	if err != nil {
		return nil, fmt.Errorf("listing expired auths: %w", err)
	}
	defer rows.Close()

	var result []domain.CardAuthorization
	for rows.Next() {
		var a domain.CardAuthorization
		if err := rows.Scan(
			&a.ID, &a.CardID, &a.RetrievalReferenceNumber, &a.STAN, &a.AuthCode,
			&a.MerchantName, &a.MerchantID, &a.MerchantCategoryCode, &a.TerminalID, &a.AcquiringInstitution,
			&a.AuthAmountCents, &a.SettlementAmountCents, &a.Currency, &a.Status,
			&a.DeclineReason, &a.ResponseCode, &a.LedgerHoldID,
			&a.AuthorizedAt, &a.SettledAt, &a.ReversedAt, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning expired auth: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r *pgCardAuthRepo) SumApprovedToday(ctx context.Context, cardID string) (int64, error) {
	var total *int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(auth_amount_cents), 0)
		FROM card_authorizations
		WHERE card_id = $1
			AND status IN ('approved', 'cleared', 'partially_cleared')
			AND authorized_at >= CURRENT_DATE
			AND authorized_at < CURRENT_DATE + INTERVAL '1 day'`, cardID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("summing daily auths: %w", err)
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

func (r *pgCardAuthRepo) SumApprovedThisMonth(ctx context.Context, cardID string) (int64, error) {
	var total *int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(auth_amount_cents), 0)
		FROM card_authorizations
		WHERE card_id = $1
			AND status IN ('approved', 'cleared', 'partially_cleared')
			AND authorized_at >= DATE_TRUNC('month', CURRENT_DATE)
			AND authorized_at < DATE_TRUNC('month', CURRENT_DATE) + INTERVAL '1 month'`, cardID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("summing monthly auths: %w", err)
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

func (r *pgCardAuthRepo) SumInternationalThisMonth(ctx context.Context, userID string) (int64, error) {
	var total *int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(ca.auth_amount_cents), 0)
		FROM card_authorizations ca
		JOIN cards c ON c.id = ca.card_id
		WHERE c.user_id = $1
			AND ca.status IN ('approved', 'cleared', 'partially_cleared')
			AND ca.merchant_currency IS NOT NULL AND ca.merchant_currency != 'ETB'
			AND ca.authorized_at >= DATE_TRUNC('month', CURRENT_DATE)
			AND ca.authorized_at < DATE_TRUNC('month', CURRENT_DATE) + INTERVAL '1 month'`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("summing international card auths: %w", err)
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

var _ CardAuthorizationRepository = (*pgCardAuthRepo)(nil)
