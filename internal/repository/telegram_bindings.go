package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

// TelegramLinkTokenRepository handles the lifecycle of deep-link tokens.
type TelegramLinkTokenRepository interface {
	// Create generates a new cryptographic token for the user, valid for the
	// given TTL. Returns the token string.
	Create(ctx context.Context, userID string, ttl time.Duration) (string, error)

	// Consume validates and atomically consumes a token. Returns the associated
	// user ID. Returns ErrNotFound if the token is invalid, consumed, or expired.
	Consume(ctx context.Context, token string) (userID string, err error)

	// PurgeExpired deletes all expired or consumed tokens older than the cutoff.
	PurgeExpired(ctx context.Context, olderThan time.Duration) (int64, error)
}

type pgTelegramTokenRepo struct{ db DBTX }

func NewTelegramLinkTokenRepository(db DBTX) TelegramLinkTokenRepository {
	return &pgTelegramTokenRepo{db: db}
}

func (r *pgTelegramTokenRepo) Create(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(ttl)
	_, err := r.db.Exec(ctx,
		`INSERT INTO telegram_link_tokens (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("inserting telegram link token: %w", err)
	}
	return token, nil
}

func (r *pgTelegramTokenRepo) Consume(ctx context.Context, token string) (string, error) {
	// Atomically mark consumed and return user_id in a single statement.
	var userID string
	err := r.db.QueryRow(ctx, `
		UPDATE telegram_link_tokens
		SET consumed = TRUE
		WHERE token = $1
			AND consumed = FALSE
			AND expires_at > NOW()
		RETURNING user_id`, token).Scan(&userID)
	if err == pgx.ErrNoRows {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("consuming telegram link token: %w", err)
	}
	return userID, nil
}

func (r *pgTelegramTokenRepo) PurgeExpired(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM telegram_link_tokens WHERE expires_at < NOW() - $1::interval OR (consumed = TRUE AND created_at < NOW() - $1::interval)`,
		olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("purging expired tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

var _ TelegramLinkTokenRepository = (*pgTelegramTokenRepo)(nil)
