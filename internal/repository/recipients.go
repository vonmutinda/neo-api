package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

// RecipientRepository defines persistence operations for recipients.
type RecipientRepository interface {
	Upsert(ctx context.Context, r *domain.Recipient) error
	GetByID(ctx context.Context, id, ownerUserID string) (*domain.Recipient, error)
	ListByOwner(ctx context.Context, ownerUserID string, filter RecipientFilter) ([]domain.Recipient, int, error)
	FindNeoUserRecipient(ctx context.Context, ownerUserID, neoUserID string) (*domain.Recipient, error)
	SearchByBankAccount(ctx context.Context, ownerUserID, institutionCode, accountPrefix string, limit int) ([]domain.Recipient, error)
	SearchByName(ctx context.Context, ownerUserID, query string, limit int) ([]domain.Recipient, error)
	UpdateFavorite(ctx context.Context, id, ownerUserID string, isFavorite bool) error
	Archive(ctx context.Context, id, ownerUserID string) error
}

// RecipientFilter controls list/search queries.
type RecipientFilter struct {
	Query    string
	Type     *string
	Favorite *bool
	Status   *string
	Limit    int
	Offset   int
}

type pgRecipientRepo struct{ db DBTX }

func NewRecipientRepository(db DBTX) RecipientRepository {
	return &pgRecipientRepo{db: db}
}

var recipientCols = `id, owner_user_id, type, display_name,
	neo_user_id, country_code, number, username,
	institution_code, bank_name, swift_bic, account_number, account_number_masked, bank_country_code,
	beneficiary_id, is_beneficiary,
	is_favorite, last_used_at, last_used_currency, transfer_count,
	status, created_at, updated_at`

func scanRecipient(row pgx.Row) (*domain.Recipient, error) {
	var r domain.Recipient
	err := row.Scan(
		&r.ID, &r.OwnerUserID, &r.Type, &r.DisplayName,
		&r.NeoUserID, &r.CountryCode, &r.Number, &r.Username,
		&r.InstitutionCode, &r.BankName, &r.SwiftBIC, &r.AccountNumber, &r.AccountNumberMasked, &r.BankCountryCode,
		&r.BeneficiaryID, &r.IsBeneficiary,
		&r.IsFavorite, &r.LastUsedAt, &r.LastUsedCurrency, &r.TransferCount,
		&r.Status, &r.CreatedAt, &r.UpdatedAt,
	)
	return &r, err
}

func scanRecipients(rows pgx.Rows) ([]domain.Recipient, error) {
	var result []domain.Recipient
	for rows.Next() {
		var r domain.Recipient
		if err := rows.Scan(
			&r.ID, &r.OwnerUserID, &r.Type, &r.DisplayName,
			&r.NeoUserID, &r.CountryCode, &r.Number, &r.Username,
			&r.InstitutionCode, &r.BankName, &r.SwiftBIC, &r.AccountNumber, &r.AccountNumberMasked, &r.BankCountryCode,
			&r.BeneficiaryID, &r.IsBeneficiary,
			&r.IsFavorite, &r.LastUsedAt, &r.LastUsedCurrency, &r.TransferCount,
			&r.Status, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning recipient row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (repo *pgRecipientRepo) Upsert(ctx context.Context, r *domain.Recipient) error {
	var conflictClause string
	switch r.Type {
	case domain.RecipientNeoUser:
		conflictClause = "(owner_user_id, neo_user_id, status) WHERE neo_user_id IS NOT NULL"
	case domain.RecipientBankAccount:
		conflictClause = "(owner_user_id, institution_code, account_number, status) WHERE institution_code IS NOT NULL AND account_number IS NOT NULL"
	default:
		return fmt.Errorf("unknown recipient type: %s", r.Type)
	}

	query := fmt.Sprintf(`
		INSERT INTO recipients (
			owner_user_id, type, display_name,
			neo_user_id, country_code, number, username,
			institution_code, bank_name, swift_bic, account_number, account_number_masked, bank_country_code,
			beneficiary_id, is_beneficiary,
			last_used_at, last_used_currency, transfer_count, status
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15,
			$16, $17, 1, 'active'
		)
		ON CONFLICT %s DO UPDATE SET
			display_name       = EXCLUDED.display_name,
			country_code       = COALESCE(EXCLUDED.country_code, recipients.country_code),
			number             = COALESCE(EXCLUDED.number, recipients.number),
			username           = COALESCE(EXCLUDED.username, recipients.username),
			last_used_at       = EXCLUDED.last_used_at,
			last_used_currency = EXCLUDED.last_used_currency,
			transfer_count     = recipients.transfer_count + 1,
			updated_at         = NOW()
		RETURNING %s`, conflictClause, recipientCols)

	row := repo.db.QueryRow(ctx, query,
		r.OwnerUserID, r.Type, r.DisplayName,
		r.NeoUserID, r.CountryCode, r.Number, r.Username,
		r.InstitutionCode, r.BankName, r.SwiftBIC, r.AccountNumber, r.AccountNumberMasked, r.BankCountryCode,
		r.BeneficiaryID, r.IsBeneficiary,
		r.LastUsedAt, r.LastUsedCurrency,
	)

	scanned, err := scanRecipient(row)
	if err != nil {
		return fmt.Errorf("upserting recipient: %w", err)
	}
	*r = *scanned
	return nil
}

func (repo *pgRecipientRepo) GetByID(ctx context.Context, id, ownerUserID string) (*domain.Recipient, error) {
	row := repo.db.QueryRow(ctx,
		`SELECT `+recipientCols+` FROM recipients WHERE id = $1 AND owner_user_id = $2`,
		id, ownerUserID,
	)
	r, err := scanRecipient(row)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRecipientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting recipient: %w", err)
	}
	return r, nil
}

func (repo *pgRecipientRepo) ListByOwner(ctx context.Context, ownerUserID string, f RecipientFilter) ([]domain.Recipient, int, error) {
	var (
		where []string
		args  []any
		idx   = 1
	)

	where = append(where, fmt.Sprintf("owner_user_id = $%d", idx))
	args = append(args, ownerUserID)
	idx++

	status := "active"
	if f.Status != nil {
		status = *f.Status
	}
	where = append(where, fmt.Sprintf("status = $%d", idx))
	args = append(args, status)
	idx++

	if f.Type != nil {
		where = append(where, fmt.Sprintf("type = $%d", idx))
		args = append(args, *f.Type)
		idx++
	}
	if f.Favorite != nil {
		where = append(where, fmt.Sprintf("is_favorite = $%d", idx))
		args = append(args, *f.Favorite)
		idx++
	}
	if f.Query != "" {
		where = append(where, fmt.Sprintf("display_name ILIKE $%d", idx))
		args = append(args, f.Query+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf("SELECT count(*) FROM recipients WHERE %s", whereClause)
	var total int
	if err := repo.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting recipients: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(
		"SELECT %s FROM recipients WHERE %s ORDER BY is_favorite DESC, last_used_at DESC NULLS LAST LIMIT $%d OFFSET $%d",
		recipientCols, whereClause, idx, idx+1,
	)
	args = append(args, limit, offset)

	rows, err := repo.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing recipients: %w", err)
	}
	defer rows.Close()

	result, err := scanRecipients(rows)
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func (repo *pgRecipientRepo) FindNeoUserRecipient(ctx context.Context, ownerUserID, neoUserID string) (*domain.Recipient, error) {
	row := repo.db.QueryRow(ctx,
		`SELECT `+recipientCols+` FROM recipients WHERE owner_user_id = $1 AND neo_user_id = $2 AND status = 'active'`,
		ownerUserID, neoUserID,
	)
	r, err := scanRecipient(row)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRecipientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding neo user recipient: %w", err)
	}
	return r, nil
}

func (repo *pgRecipientRepo) SearchByBankAccount(ctx context.Context, ownerUserID, institutionCode, accountPrefix string, limit int) ([]domain.Recipient, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := repo.db.Query(ctx,
		`SELECT `+recipientCols+` FROM recipients
		 WHERE owner_user_id = $1 AND status = 'active'
		   AND institution_code = $2 AND account_number LIKE $3
		 ORDER BY transfer_count DESC, last_used_at DESC NULLS LAST
		 LIMIT $4`,
		ownerUserID, institutionCode, accountPrefix+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searching recipients by bank account: %w", err)
	}
	defer rows.Close()
	return scanRecipients(rows)
}

func (repo *pgRecipientRepo) SearchByName(ctx context.Context, ownerUserID, query string, limit int) ([]domain.Recipient, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := repo.db.Query(ctx,
		`SELECT `+recipientCols+` FROM recipients
		 WHERE owner_user_id = $1 AND status = 'active'
		   AND display_name ILIKE $2
		 ORDER BY transfer_count DESC, last_used_at DESC NULLS LAST
		 LIMIT $3`,
		ownerUserID, query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searching recipients by name: %w", err)
	}
	defer rows.Close()
	return scanRecipients(rows)
}

func (repo *pgRecipientRepo) UpdateFavorite(ctx context.Context, id, ownerUserID string, isFavorite bool) error {
	tag, err := repo.db.Exec(ctx,
		`UPDATE recipients SET is_favorite = $3, updated_at = NOW()
		 WHERE id = $1 AND owner_user_id = $2 AND status = 'active'`,
		id, ownerUserID, isFavorite,
	)
	if err != nil {
		return fmt.Errorf("updating recipient favorite: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRecipientNotFound
	}
	return nil
}

func (repo *pgRecipientRepo) Archive(ctx context.Context, id, ownerUserID string) error {
	tag, err := repo.db.Exec(ctx,
		`UPDATE recipients SET status = 'archived', updated_at = NOW()
		 WHERE id = $1 AND owner_user_id = $2 AND status = 'active'`,
		id, ownerUserID,
	)
	if err != nil {
		return fmt.Errorf("archiving recipient: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRecipientNotFound
	}
	return nil
}

var _ RecipientRepository = (*pgRecipientRepo)(nil)
