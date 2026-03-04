package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type PotRepository interface {
	Create(ctx context.Context, pot *domain.Pot) error
	Update(ctx context.Context, pot *domain.Pot) error
	Archive(ctx context.Context, potID, userID string) error
	GetByID(ctx context.Context, potID string) (*domain.Pot, error)
	ListActiveByUser(ctx context.Context, userID string) ([]domain.Pot, error)
}

type pgPotRepo struct{ db DBTX }

func NewPotRepository(db DBTX) PotRepository {
	return &pgPotRepo{db: db}
}

func (r *pgPotRepo) Create(ctx context.Context, pot *domain.Pot) error {
	query := `
		INSERT INTO pots (user_id, name, currency_code, target_cents, emoji)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		pot.UserID, pot.Name, pot.CurrencyCode, pot.TargetCents, pot.Emoji,
	).Scan(&pot.ID, &pot.CreatedAt, &pot.UpdatedAt)
}

func (r *pgPotRepo) Update(ctx context.Context, pot *domain.Pot) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE pots SET name = $3, target_cents = $4, emoji = $5, updated_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND NOT is_archived`,
		pot.ID, pot.UserID, pot.Name, pot.TargetCents, pot.Emoji)
	if err != nil {
		return fmt.Errorf("updating pot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPotNotFound
	}
	return nil
}

func (r *pgPotRepo) Archive(ctx context.Context, potID, userID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE pots SET is_archived = TRUE, archived_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND NOT is_archived`,
		potID, userID)
	if err != nil {
		return fmt.Errorf("archiving pot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPotNotFound
	}
	return nil
}

func (r *pgPotRepo) GetByID(ctx context.Context, potID string) (*domain.Pot, error) {
	var p domain.Pot
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, name, currency_code, target_cents, emoji, is_archived, created_at, updated_at, archived_at
		 FROM pots WHERE id = $1`, potID).
		Scan(&p.ID, &p.UserID, &p.Name, &p.CurrencyCode, &p.TargetCents, &p.Emoji,
			&p.IsArchived, &p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrPotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting pot: %w", err)
	}
	return &p, nil
}

func (r *pgPotRepo) ListActiveByUser(ctx context.Context, userID string) ([]domain.Pot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, currency_code, target_cents, emoji, is_archived, created_at, updated_at, archived_at
		 FROM pots WHERE user_id = $1 AND NOT is_archived
		 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing pots: %w", err)
	}
	defer rows.Close()

	var result []domain.Pot
	for rows.Next() {
		var p domain.Pot
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.CurrencyCode, &p.TargetCents, &p.Emoji,
			&p.IsArchived, &p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt); err != nil {
			return nil, fmt.Errorf("scanning pot: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

var _ PotRepository = (*pgPotRepo)(nil)
