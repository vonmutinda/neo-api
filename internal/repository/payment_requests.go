package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/jackc/pgx/v5"
)

type PaymentRequestRepository interface {
	Create(ctx context.Context, pr *domain.PaymentRequest) error
	GetByID(ctx context.Context, id string) (*domain.PaymentRequest, error)
	Pay(ctx context.Context, id, transactionID string) error
	Decline(ctx context.Context, id, reason string) error
	Cancel(ctx context.Context, id string) error
	IncrementReminder(ctx context.Context, id string) error
	ListByRequester(ctx context.Context, requesterID string, limit, offset int) ([]domain.PaymentRequest, error)
	ListByPayer(ctx context.Context, payerID string, limit, offset int) ([]domain.PaymentRequest, error)
	CountPendingByPayer(ctx context.Context, payerID string) (int, error)
	ExpirePending(ctx context.Context) (int64, error)
	ResolvePayer(ctx context.Context, p phone.PhoneNumber, userID string) (int64, error)
}

type pgPaymentRequestRepo struct{ db DBTX }

func NewPaymentRequestRepository(db DBTX) PaymentRequestRepository {
	return &pgPaymentRequestRepo{db: db}
}

func (r *pgPaymentRequestRepo) Create(ctx context.Context, pr *domain.PaymentRequest) error {
	query := `INSERT INTO payment_requests
		(requester_id, payer_id, payer_country_code, payer_number,
		 amount_cents, currency_code, narration, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, status, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		pr.RequesterID, pr.PayerID, pr.PayerPhone.CountryCode, pr.PayerPhone.Number,
		pr.AmountCents, pr.CurrencyCode, pr.Narration, pr.ExpiresAt,
	).Scan(&pr.ID, &pr.Status, &pr.CreatedAt, &pr.UpdatedAt)
}

func (r *pgPaymentRequestRepo) GetByID(ctx context.Context, id string) (*domain.PaymentRequest, error) {
	var pr domain.PaymentRequest
	err := r.db.QueryRow(ctx, `
		SELECT pr.id, pr.requester_id, pr.payer_id, pr.payer_country_code, pr.payer_number,
			pr.amount_cents, pr.currency_code, pr.narration, pr.status,
			pr.transaction_id, pr.decline_reason, pr.reminder_count, pr.last_reminded_at,
			pr.paid_at, pr.declined_at, pr.cancelled_at, pr.expires_at,
			pr.created_at, pr.updated_at,
			COALESCE(req.first_name || ' ' || COALESCE(req.last_name, ''), ''),
			COALESCE(pay.first_name || ' ' || COALESCE(pay.last_name, ''), '')
		FROM payment_requests pr
		LEFT JOIN users req ON req.id = pr.requester_id
		LEFT JOIN users pay ON pay.id = pr.payer_id
		WHERE pr.id = $1`, id,
	).Scan(
		&pr.ID, &pr.RequesterID, &pr.PayerID, &pr.PayerPhone.CountryCode, &pr.PayerPhone.Number,
		&pr.AmountCents, &pr.CurrencyCode, &pr.Narration, &pr.Status,
		&pr.TransactionID, &pr.DeclineReason, &pr.ReminderCount, &pr.LastRemindedAt,
		&pr.PaidAt, &pr.DeclinedAt, &pr.CancelledAt, &pr.ExpiresAt,
		&pr.CreatedAt, &pr.UpdatedAt,
		&pr.RequesterName, &pr.PayerName,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrPaymentRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting payment request: %w", err)
	}
	return &pr, nil
}

func (r *pgPaymentRequestRepo) Pay(ctx context.Context, id, transactionID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE payment_requests
		SET status = 'paid', transaction_id = $2, paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending'`, id, transactionID)
	if err != nil {
		return fmt.Errorf("paying payment request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPaymentRequestNotFound
	}
	return nil
}

func (r *pgPaymentRequestRepo) Decline(ctx context.Context, id, reason string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE payment_requests
		SET status = 'declined', decline_reason = $2, declined_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending'`, id, reason)
	if err != nil {
		return fmt.Errorf("declining payment request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPaymentRequestNotFound
	}
	return nil
}

func (r *pgPaymentRequestRepo) Cancel(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE payment_requests
		SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return fmt.Errorf("cancelling payment request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPaymentRequestNotFound
	}
	return nil
}

func (r *pgPaymentRequestRepo) IncrementReminder(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE payment_requests
		SET reminder_count = reminder_count + 1, last_reminded_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending' AND reminder_count < 3`, id)
	if err != nil {
		return fmt.Errorf("incrementing reminder: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrReminderLimitReached
	}
	return nil
}

func (r *pgPaymentRequestRepo) ListByRequester(ctx context.Context, requesterID string, limit, offset int) ([]domain.PaymentRequest, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT pr.id, pr.requester_id, pr.payer_id, pr.payer_country_code, pr.payer_number,
			pr.amount_cents, pr.currency_code, pr.narration, pr.status,
			pr.transaction_id, pr.decline_reason, pr.reminder_count, pr.last_reminded_at,
			pr.paid_at, pr.declined_at, pr.cancelled_at, pr.expires_at,
			pr.created_at, pr.updated_at,
			COALESCE(pay.first_name || ' ' || COALESCE(pay.last_name, ''), '')
		FROM payment_requests pr
		LEFT JOIN users pay ON pay.id = pr.payer_id
		WHERE pr.requester_id = $1
		ORDER BY pr.created_at DESC LIMIT $2 OFFSET $3`, requesterID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing payment requests by requester: %w", err)
	}
	defer rows.Close()
	return scanPaymentRequestListWithPayerName(rows)
}

func (r *pgPaymentRequestRepo) ListByPayer(ctx context.Context, payerID string, limit, offset int) ([]domain.PaymentRequest, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT pr.id, pr.requester_id, pr.payer_id, pr.payer_country_code, pr.payer_number,
			pr.amount_cents, pr.currency_code, pr.narration, pr.status,
			pr.transaction_id, pr.decline_reason, pr.reminder_count, pr.last_reminded_at,
			pr.paid_at, pr.declined_at, pr.cancelled_at, pr.expires_at,
			pr.created_at, pr.updated_at,
			COALESCE(req.first_name || ' ' || COALESCE(req.last_name, ''), '')
		FROM payment_requests pr
		LEFT JOIN users req ON req.id = pr.requester_id
		WHERE pr.payer_id = $1
		ORDER BY pr.created_at DESC LIMIT $2 OFFSET $3`, payerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing payment requests by payer: %w", err)
	}
	defer rows.Close()
	return scanPaymentRequestListWithRequesterName(rows)
}

func (r *pgPaymentRequestRepo) CountPendingByPayer(ctx context.Context, payerID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM payment_requests WHERE payer_id = $1 AND status = 'pending'`,
		payerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending payment requests: %w", err)
	}
	return count, nil
}

func (r *pgPaymentRequestRepo) ExpirePending(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE payment_requests SET status = 'expired', updated_at = NOW()
		WHERE status = 'pending' AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("expiring payment requests: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *pgPaymentRequestRepo) ResolvePayer(ctx context.Context, p phone.PhoneNumber, userID string) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE payment_requests SET payer_id = $3, updated_at = NOW()
		WHERE payer_country_code = $1 AND payer_number = $2
		  AND payer_id IS NULL AND status = 'pending'`,
		p.CountryCode, p.Number, userID)
	if err != nil {
		return 0, fmt.Errorf("resolving payer: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanPaymentRequestListWithPayerName(rows pgx.Rows) ([]domain.PaymentRequest, error) {
	var result []domain.PaymentRequest
	for rows.Next() {
		var pr domain.PaymentRequest
		if err := rows.Scan(
			&pr.ID, &pr.RequesterID, &pr.PayerID, &pr.PayerPhone.CountryCode, &pr.PayerPhone.Number,
			&pr.AmountCents, &pr.CurrencyCode, &pr.Narration, &pr.Status,
			&pr.TransactionID, &pr.DeclineReason, &pr.ReminderCount, &pr.LastRemindedAt,
			&pr.PaidAt, &pr.DeclinedAt, &pr.CancelledAt, &pr.ExpiresAt,
			&pr.CreatedAt, &pr.UpdatedAt,
			&pr.PayerName,
		); err != nil {
			return nil, fmt.Errorf("scanning payment request: %w", err)
		}
		result = append(result, pr)
	}
	return result, rows.Err()
}

func scanPaymentRequestListWithRequesterName(rows pgx.Rows) ([]domain.PaymentRequest, error) {
	var result []domain.PaymentRequest
	for rows.Next() {
		var pr domain.PaymentRequest
		if err := rows.Scan(
			&pr.ID, &pr.RequesterID, &pr.PayerID, &pr.PayerPhone.CountryCode, &pr.PayerPhone.Number,
			&pr.AmountCents, &pr.CurrencyCode, &pr.Narration, &pr.Status,
			&pr.TransactionID, &pr.DeclineReason, &pr.ReminderCount, &pr.LastRemindedAt,
			&pr.PaidAt, &pr.DeclinedAt, &pr.CancelledAt, &pr.ExpiresAt,
			&pr.CreatedAt, &pr.UpdatedAt,
			&pr.RequesterName,
		); err != nil {
			return nil, fmt.Errorf("scanning payment request: %w", err)
		}
		result = append(result, pr)
	}
	return result, rows.Err()
}

var _ PaymentRequestRepository = (*pgPaymentRequestRepo)(nil)
