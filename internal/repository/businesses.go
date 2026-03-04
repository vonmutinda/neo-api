package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type BusinessRepository interface {
	Create(ctx context.Context, biz *domain.Business) error
	GetByID(ctx context.Context, id string) (*domain.Business, error)
	Update(ctx context.Context, biz *domain.Business) error
	ListByOwner(ctx context.Context, ownerUserID string) ([]domain.Business, error)
	ListByMember(ctx context.Context, userID string) ([]domain.Business, error)
	UpdateStatus(ctx context.Context, id string, status domain.BusinessStatus) error
	Freeze(ctx context.Context, id, reason string) error
	Unfreeze(ctx context.Context, id string) error
}

type pgBusinessRepo struct{ db DBTX }

func NewBusinessRepository(db DBTX) BusinessRepository {
	return &pgBusinessRepo{db: db}
}

func (r *pgBusinessRepo) Create(ctx context.Context, biz *domain.Business) error {
	query := `
		INSERT INTO businesses (
			id, owner_user_id, name, trade_name, tin_number, trade_license_number,
			industry_category, industry_sub_category, registration_date,
			address, city, sub_city, woreda, country_code, number, email, website,
			status, ledger_wallet_id, kyb_level
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		biz.ID, biz.OwnerUserID, biz.Name, biz.TradeName,
		biz.TINNumber, biz.TradeLicenseNumber,
		biz.IndustryCategory, biz.IndustrySubCategory, biz.RegistrationDate,
		biz.Address, biz.City, biz.SubCity, biz.Woreda,
		biz.PhoneNumber.CountryCode, biz.PhoneNumber.Number, biz.Email, biz.Website,
		biz.Status, biz.LedgerWalletID, biz.KYBLevel,
	).Scan(&biz.CreatedAt, &biz.UpdatedAt)
}

func (r *pgBusinessRepo) GetByID(ctx context.Context, id string) (*domain.Business, error) {
	query := `
		SELECT id, owner_user_id, name, trade_name, tin_number, trade_license_number,
			industry_category, industry_sub_category, registration_date,
			address, city, sub_city, woreda, country_code, number, email, website,
			status, ledger_wallet_id, kyb_level, is_frozen, frozen_reason, frozen_at,
			created_at, updated_at
		FROM businesses WHERE id = $1`
	return r.scanBusiness(ctx, query, id)
}

func (r *pgBusinessRepo) Update(ctx context.Context, biz *domain.Business) error {
	query := `
		UPDATE businesses SET
			name = $2, trade_name = $3, industry_category = $4,
			industry_sub_category = $5, address = $6, city = $7,
			sub_city = $8, woreda = $9, country_code = $10, number = $11,
			email = $12, website = $13, updated_at = NOW()
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, query,
		biz.ID, biz.Name, biz.TradeName, biz.IndustryCategory,
		biz.IndustrySubCategory, biz.Address, biz.City,
		biz.SubCity, biz.Woreda, biz.PhoneNumber.CountryCode, biz.PhoneNumber.Number,
		biz.Email, biz.Website)
	if err != nil {
		return fmt.Errorf("updating business: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBusinessNotFound
	}
	return nil
}

func (r *pgBusinessRepo) ListByOwner(ctx context.Context, ownerUserID string) ([]domain.Business, error) {
	query := `
		SELECT id, owner_user_id, name, trade_name, tin_number, trade_license_number,
			industry_category, industry_sub_category, registration_date,
			address, city, sub_city, woreda, country_code, number, email, website,
			status, ledger_wallet_id, kyb_level, is_frozen, frozen_reason, frozen_at,
			created_at, updated_at
		FROM businesses WHERE owner_user_id = $1 ORDER BY created_at DESC`
	return r.scanBusinesses(ctx, query, ownerUserID)
}

func (r *pgBusinessRepo) ListByMember(ctx context.Context, userID string) ([]domain.Business, error) {
	query := `
		SELECT b.id, b.owner_user_id, b.name, b.trade_name, b.tin_number, b.trade_license_number,
			b.industry_category, b.industry_sub_category, b.registration_date,
			b.address, b.city, b.sub_city, b.woreda, b.country_code, b.number, b.email, b.website,
			b.status, b.ledger_wallet_id, b.kyb_level, b.is_frozen, b.frozen_reason, b.frozen_at,
			b.created_at, b.updated_at
		FROM businesses b
		JOIN business_members bm ON bm.business_id = b.id
		WHERE bm.user_id = $1 AND bm.is_active
		ORDER BY b.created_at DESC`
	return r.scanBusinesses(ctx, query, userID)
}

func (r *pgBusinessRepo) UpdateStatus(ctx context.Context, id string, status domain.BusinessStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE businesses SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("updating business status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBusinessNotFound
	}
	return nil
}

func (r *pgBusinessRepo) Freeze(ctx context.Context, id, reason string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE businesses SET is_frozen = TRUE, frozen_reason = $2, frozen_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id, reason)
	if err != nil {
		return fmt.Errorf("freezing business: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBusinessNotFound
	}
	return nil
}

func (r *pgBusinessRepo) Unfreeze(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE businesses SET is_frozen = FALSE, frozen_reason = NULL, frozen_at = NULL, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("unfreezing business: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBusinessNotFound
	}
	return nil
}

func (r *pgBusinessRepo) scanBusiness(ctx context.Context, query string, args ...any) (*domain.Business, error) {
	var b domain.Business
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&b.ID, &b.OwnerUserID, &b.Name, &b.TradeName, &b.TINNumber, &b.TradeLicenseNumber,
		&b.IndustryCategory, &b.IndustrySubCategory, &b.RegistrationDate,
		&b.Address, &b.City, &b.SubCity, &b.Woreda, &b.PhoneNumber.CountryCode, &b.PhoneNumber.Number, &b.Email, &b.Website,
		&b.Status, &b.LedgerWalletID, &b.KYBLevel, &b.IsFrozen, &b.FrozenReason, &b.FrozenAt,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrBusinessNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning business: %w", err)
	}
	return &b, nil
}

func (r *pgBusinessRepo) scanBusinesses(ctx context.Context, query string, args ...any) ([]domain.Business, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing businesses: %w", err)
	}
	defer rows.Close()
	var result []domain.Business
	for rows.Next() {
		var b domain.Business
		if err := rows.Scan(
			&b.ID, &b.OwnerUserID, &b.Name, &b.TradeName, &b.TINNumber, &b.TradeLicenseNumber,
			&b.IndustryCategory, &b.IndustrySubCategory, &b.RegistrationDate,
			&b.Address, &b.City, &b.SubCity, &b.Woreda, &b.PhoneNumber.CountryCode, &b.PhoneNumber.Number, &b.Email, &b.Website,
			&b.Status, &b.LedgerWalletID, &b.KYBLevel, &b.IsFrozen, &b.FrozenReason, &b.FrozenAt,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning business row: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

var _ BusinessRepository = (*pgBusinessRepo)(nil)
