package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type AccountDetailsRepository interface {
	Create(ctx context.Context, details *domain.AccountDetails) error
	GetByCurrencyBalanceID(ctx context.Context, currencyBalanceID string) (*domain.AccountDetails, error)
	ListByUser(ctx context.Context, userID string) ([]domain.AccountDetails, error)
	NextAccountNumber(ctx context.Context) (int64, error)
}

type pgAccountDetailsRepo struct{ db DBTX }

func NewAccountDetailsRepository(db DBTX) AccountDetailsRepository {
	return &pgAccountDetailsRepo{db: db}
}

func (r *pgAccountDetailsRepo) Create(ctx context.Context, d *domain.AccountDetails) error {
	query := `
		INSERT INTO account_details (currency_balance_id, iban, account_number, bank_name, swift_code, routing_number, sort_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		d.CurrencyBalanceID, d.IBAN, d.AccountNumber, d.BankName, d.SwiftCode, d.RoutingNumber, d.SortCode,
	).Scan(&d.ID, &d.CreatedAt)
}

func (r *pgAccountDetailsRepo) GetByCurrencyBalanceID(ctx context.Context, currencyBalanceID string) (*domain.AccountDetails, error) {
	var d domain.AccountDetails
	err := r.db.QueryRow(ctx,
		`SELECT id, currency_balance_id, iban, account_number, bank_name, swift_code, routing_number, sort_code, created_at
		 FROM account_details WHERE currency_balance_id = $1`,
		currencyBalanceID).
		Scan(&d.ID, &d.CurrencyBalanceID, &d.IBAN, &d.AccountNumber, &d.BankName, &d.SwiftCode, &d.RoutingNumber, &d.SortCode, &d.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting account details: %w", err)
	}
	return &d, nil
}

func (r *pgAccountDetailsRepo) ListByUser(ctx context.Context, userID string) ([]domain.AccountDetails, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ad.id, ad.currency_balance_id, ad.iban, ad.account_number, ad.bank_name, ad.swift_code, ad.routing_number, ad.sort_code, ad.created_at
		 FROM account_details ad
		 JOIN currency_balances cb ON cb.id = ad.currency_balance_id
		 WHERE cb.user_id = $1 AND cb.deleted_at IS NULL
		 ORDER BY ad.created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing account details: %w", err)
	}
	defer rows.Close()

	var result []domain.AccountDetails
	for rows.Next() {
		var d domain.AccountDetails
		if err := rows.Scan(&d.ID, &d.CurrencyBalanceID, &d.IBAN, &d.AccountNumber, &d.BankName, &d.SwiftCode, &d.RoutingNumber, &d.SortCode, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning account details: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *pgAccountDetailsRepo) NextAccountNumber(ctx context.Context) (int64, error) {
	var seq int64
	err := r.db.QueryRow(ctx, "SELECT nextval('account_number_seq')").Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("getting next account number: %w", err)
	}
	return seq, nil
}

var _ AccountDetailsRepository = (*pgAccountDetailsRepo)(nil)
