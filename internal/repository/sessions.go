package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	GetByRefreshToken(ctx context.Context, token string) (*domain.Session, error)
	Revoke(ctx context.Context, id string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type pgSessionRepo struct{ db DBTX }

func NewSessionRepository(db DBTX) SessionRepository { return &pgSessionRepo{db: db} }

func (r *pgSessionRepo) Create(ctx context.Context, s *domain.Session) error {
	query := `
		INSERT INTO sessions (user_id, refresh_token, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		s.UserID, s.RefreshToken, s.UserAgent, s.IPAddress, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
}

func (r *pgSessionRepo) GetByRefreshToken(ctx context.Context, token string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, refresh_token, user_agent, ip_address, expires_at, revoked_at, created_at
		FROM sessions WHERE refresh_token = $1`, token).Scan(
		&s.ID, &s.UserID, &s.RefreshToken, &s.UserAgent, &s.IPAddress,
		&s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	return &s, nil
}

func (r *pgSessionRepo) Revoke(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSessionNotFound
	}
	return nil
}

func (r *pgSessionRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("revoking all sessions: %w", err)
	}
	return nil
}

func (r *pgSessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM sessions WHERE expires_at < NOW() OR revoked_at IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

var _ SessionRepository = (*pgSessionRepo)(nil)
