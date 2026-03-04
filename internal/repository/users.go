package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByPhone(ctx context.Context, p phone.PhoneNumber) (*domain.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	UpdateKYCLevel(ctx context.Context, id string, level domain.KYCLevel) error
	UpdateDemographics(ctx context.Context, id string, first, middle, last *string, dob *string, gender *string, photoURL *string) error
	Freeze(ctx context.Context, id, reason string) error
	Unfreeze(ctx context.Context, id string) error
	BindTelegram(ctx context.Context, id string, telegramID int64, username string) error
	UnbindTelegram(ctx context.Context, id string) error
	UpdateSpendWaterfall(ctx context.Context, id string, waterfall domain.SpendWaterfall) error
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	UpdatePassword(ctx context.Context, id, hash string) error
}

type pgUserRepo struct{ db DBTX }

func NewUserRepository(db DBTX) UserRepository { return &pgUserRepo{db: db} }

func (r *pgUserRepo) Create(ctx context.Context, user *domain.User) error {
	if user.AccountType == "" {
		user.AccountType = domain.AccountTypePersonal
	}
	var pwHash *string
	if user.PasswordHash != "" {
		pwHash = &user.PasswordHash
	}
	query := `
		INSERT INTO users (id, country_code, number, username, password_hash, ledger_wallet_id, kyc_level, account_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(ctx, query, user.ID, user.PhoneNumber.CountryCode, user.PhoneNumber.Number, user.Username, pwHash, user.LedgerWalletID, user.KYCLevel, user.AccountType).
		Scan(&user.CreatedAt, &user.UpdatedAt)
}

func (r *pgUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return r.scanUser(ctx, `SELECT * FROM users WHERE id = $1`, id)
}

func (r *pgUserRepo) GetByPhone(ctx context.Context, p phone.PhoneNumber) (*domain.User, error) {
	return r.scanUser(ctx, `SELECT * FROM users WHERE country_code = $1 AND number = $2`, p.CountryCode, p.Number)
}

func (r *pgUserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	return r.scanUser(ctx, `SELECT * FROM users WHERE telegram_id = $1`, telegramID)
}

func (r *pgUserRepo) UpdateKYCLevel(ctx context.Context, id string, level domain.KYCLevel) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET kyc_level = $2, updated_at = NOW() WHERE id = $1`, id, level)
	if err != nil {
		return fmt.Errorf("updating kyc level: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) UpdateDemographics(ctx context.Context, id string, first, middle, last *string, dob *string, gender *string, photoURL *string) error {
	query := `
		UPDATE users SET
			first_name = COALESCE($2, first_name),
			middle_name = COALESCE($3, middle_name),
			last_name = COALESCE($4, last_name),
			date_of_birth = COALESCE($5::date, date_of_birth),
			gender = COALESCE($6, gender),
			fayda_photo_url = COALESCE($7, fayda_photo_url),
			updated_at = NOW()
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id, first, middle, last, dob, gender, photoURL)
	if err != nil {
		return fmt.Errorf("updating demographics: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) Freeze(ctx context.Context, id, reason string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET is_frozen = TRUE, frozen_reason = $2, frozen_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id, reason)
	if err != nil {
		return fmt.Errorf("freezing user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) Unfreeze(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET is_frozen = FALSE, frozen_reason = NULL, frozen_at = NULL, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("unfreezing user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) BindTelegram(ctx context.Context, id string, telegramID int64, username string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET telegram_id = $2, telegram_username = $3, updated_at = NOW() WHERE id = $1`,
		id, telegramID, username)
	if err != nil {
		return fmt.Errorf("binding telegram: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) UnbindTelegram(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET telegram_id = NULL, telegram_username = NULL, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("unbinding telegram: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) UpdateSpendWaterfall(ctx context.Context, id string, waterfall domain.SpendWaterfall) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET spend_waterfall_order = $2, updated_at = NOW() WHERE id = $1`,
		id, waterfall)
	if err != nil {
		return fmt.Errorf("updating spend waterfall: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.scanUser(ctx, `SELECT * FROM users WHERE username = $1`, username)
}

func (r *pgUserRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`, id, hash)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *pgUserRepo) scanUser(ctx context.Context, query string, args ...any) (*domain.User, error) {
	var u domain.User
	var pwHash *string
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.PhoneNumber.CountryCode, &u.PhoneNumber.Number, &u.Username, &pwHash, &u.FaydaIDNumber,
		&u.FirstName, &u.MiddleName, &u.LastName, &u.DateOfBirth, &u.Gender, &u.FaydaPhotoURL,
		&u.KYCLevel, &u.IsFrozen, &u.FrozenReason, &u.FrozenAt,
		&u.LedgerWalletID, &u.TelegramID, &u.TelegramUsername,
		&u.AccountType, &u.SpendWaterfallOrder,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	if pwHash != nil {
		u.PasswordHash = *pwHash
	}
	return &u, nil
}

var _ UserRepository = (*pgUserRepo)(nil)
