package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type RemittanceProviderRow struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	IsActive      bool      `json:"isActive"`
	Corridors     []byte    `json:"corridors"`
	APIBaseURL    string    `json:"apiBaseUrl"`
	WebhookSecret *string   `json:"webhookSecret,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type RemittanceProviderRepository interface {
	GetByID(ctx context.Context, id string) (*RemittanceProviderRow, error)
	ListActive(ctx context.Context) ([]RemittanceProviderRow, error)
	UpdateStatus(ctx context.Context, id string, isActive bool) error
}

type pgRemittanceProviderRepo struct {
	db DBTX
}

func NewRemittanceProviderRepository(db DBTX) RemittanceProviderRepository {
	return &pgRemittanceProviderRepo{db: db}
}

func (r *pgRemittanceProviderRepo) GetByID(ctx context.Context, id string) (*RemittanceProviderRow, error) {
	var p RemittanceProviderRow
	err := r.db.QueryRow(ctx,
		`SELECT id, name, is_active, corridors, api_base_url, webhook_secret, created_at, updated_at
		FROM remittance_providers WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.IsActive, &p.Corridors, &p.APIBaseURL, &p.WebhookSecret, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *pgRemittanceProviderRepo) ListActive(ctx context.Context) ([]RemittanceProviderRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, is_active, corridors, api_base_url, webhook_secret, created_at, updated_at
		FROM remittance_providers WHERE is_active = true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RemittanceProviderRow
	for rows.Next() {
		var p RemittanceProviderRow
		if err := rows.Scan(&p.ID, &p.Name, &p.IsActive, &p.Corridors, &p.APIBaseURL, &p.WebhookSecret, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *pgRemittanceProviderRepo) UpdateStatus(ctx context.Context, id string, isActive bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE remittance_providers SET is_active=$2, updated_at=NOW() WHERE id=$1`, id, isActive)
	return err
}
