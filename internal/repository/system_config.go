package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type SystemConfigRepository interface {
	Get(ctx context.Context, key string) (*domain.SystemConfig, error)
	Set(ctx context.Context, key string, value json.RawMessage, updatedBy *string) error
	ListAll(ctx context.Context) ([]domain.SystemConfig, error)
}

type pgSystemConfigRepo struct{ db DBTX }

func NewSystemConfigRepository(db DBTX) SystemConfigRepository {
	return &pgSystemConfigRepo{db: db}
}

func (r *pgSystemConfigRepo) Get(ctx context.Context, key string) (*domain.SystemConfig, error) {
	var c domain.SystemConfig
	err := r.db.QueryRow(ctx,
		`SELECT key, value, description, updated_by, updated_at, created_at FROM system_config WHERE key = $1`,
		key,
	).Scan(&c.Key, &c.Value, &c.Description, &c.UpdatedBy, &c.UpdatedAt, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting config %s: %w", key, err)
	}
	return &c, nil
}

func (r *pgSystemConfigRepo) Set(ctx context.Context, key string, value json.RawMessage, updatedBy *string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE system_config SET value = $2, updated_by = $3, updated_at = NOW()
		WHERE key = $1`,
		key, value, updatedBy)
	if err != nil {
		return fmt.Errorf("setting config %s: %w", key, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConfigNotFound
	}
	return nil
}

func (r *pgSystemConfigRepo) ListAll(ctx context.Context) ([]domain.SystemConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT key, value, description, updated_by, updated_at, created_at FROM system_config ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("listing config: %w", err)
	}
	defer rows.Close()

	var configs []domain.SystemConfig
	for rows.Next() {
		var c domain.SystemConfig
		if err := rows.Scan(&c.Key, &c.Value, &c.Description, &c.UpdatedBy, &c.UpdatedAt, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

var _ SystemConfigRepository = (*pgSystemConfigRepo)(nil)
