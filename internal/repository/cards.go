package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type CardRepository interface {
	Create(ctx context.Context, card *domain.Card) error
	GetByID(ctx context.Context, id string) (*domain.Card, error)
	GetByToken(ctx context.Context, tokenizedPAN string) (*domain.Card, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Card, error)
	UpdateStatus(ctx context.Context, id string, status domain.CardStatus) error
	UpdateLimits(ctx context.Context, id string, daily, monthly, perTxn int64) error
	UpdateToggles(ctx context.Context, id string, online, contactless, atm, international bool) error
}

type pgCardRepo struct{ db DBTX }

func NewCardRepository(db DBTX) CardRepository { return &pgCardRepo{db: db} }

func (r *pgCardRepo) Create(ctx context.Context, c *domain.Card) error {
	query := `
		INSERT INTO cards (user_id, tokenized_pan, last_four, expiry_month, expiry_year, type, status,
			allow_online, allow_contactless, allow_atm, allow_international,
			daily_limit_cents, monthly_limit_cents, per_txn_limit_cents, ledger_card_account)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		c.UserID, c.TokenizedPAN, c.LastFour, c.ExpiryMonth, c.ExpiryYear, c.Type, c.Status,
		c.AllowOnline, c.AllowContactless, c.AllowATM, c.AllowInternational,
		c.DailyLimitCents, c.MonthlyLimitCents, c.PerTxnLimitCents, c.LedgerCardAccount,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *pgCardRepo) GetByID(ctx context.Context, id string) (*domain.Card, error) {
	return r.scanCard(ctx, `SELECT * FROM cards WHERE id = $1`, id)
}

func (r *pgCardRepo) GetByToken(ctx context.Context, tokenizedPAN string) (*domain.Card, error) {
	return r.scanCard(ctx, `SELECT * FROM cards WHERE tokenized_pan = $1`, tokenizedPAN)
}

func (r *pgCardRepo) ListByUserID(ctx context.Context, userID string) ([]domain.Card, error) {
	rows, err := r.db.Query(ctx, `SELECT * FROM cards WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing cards: %w", err)
	}
	defer rows.Close()
	var cards []domain.Card
	for rows.Next() {
		c, err := scanCardRow(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, *c)
	}
	return cards, rows.Err()
}

func (r *pgCardRepo) UpdateStatus(ctx context.Context, id string, status domain.CardStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE cards SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("updating card status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCardNotFound
	}
	return nil
}

func (r *pgCardRepo) UpdateLimits(ctx context.Context, id string, daily, monthly, perTxn int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE cards SET daily_limit_cents=$2, monthly_limit_cents=$3, per_txn_limit_cents=$4, updated_at=NOW() WHERE id=$1`,
		id, daily, monthly, perTxn)
	if err != nil {
		return fmt.Errorf("updating card limits: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCardNotFound
	}
	return nil
}

func (r *pgCardRepo) UpdateToggles(ctx context.Context, id string, online, contactless, atm, international bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE cards SET allow_online=$2, allow_contactless=$3, allow_atm=$4, allow_international=$5, updated_at=NOW() WHERE id=$1`,
		id, online, contactless, atm, international)
	if err != nil {
		return fmt.Errorf("updating card toggles: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCardNotFound
	}
	return nil
}

func (r *pgCardRepo) scanCard(ctx context.Context, query string, args ...any) (*domain.Card, error) {
	row := r.db.QueryRow(ctx, query, args...)
	c, err := scanCardFromRow(row)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCardNotFound
	}
	return c, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanCardFromRow(row scannable) (*domain.Card, error) {
	var c domain.Card
	err := row.Scan(
		&c.ID, &c.UserID, &c.TokenizedPAN, &c.LastFour, &c.ExpiryMonth, &c.ExpiryYear,
		&c.Type, &c.Status,
		&c.AllowOnline, &c.AllowContactless, &c.AllowATM, &c.AllowInternational,
		&c.DailyLimitCents, &c.MonthlyLimitCents, &c.PerTxnLimitCents,
		&c.LedgerCardAccount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning card: %w", err)
	}
	return &c, nil
}

func scanCardRow(rows pgx.Rows) (*domain.Card, error) { return scanCardFromRow(rows) }

var _ CardRepository = (*pgCardRepo)(nil)
