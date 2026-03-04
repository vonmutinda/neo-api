package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type BusinessMemberRepository interface {
	Create(ctx context.Context, member *domain.BusinessMember) error
	GetByID(ctx context.Context, id string) (*domain.BusinessMember, error)
	GetByBusinessAndUser(ctx context.Context, businessID, userID string) (*domain.BusinessMember, error)
	UpdateRole(ctx context.Context, id, roleID string) error
	Remove(ctx context.Context, id, removedBy string) error
	ListByBusiness(ctx context.Context, businessID string) ([]domain.BusinessMember, error)
	ListByUser(ctx context.Context, userID string) ([]domain.BusinessMember, error)
	SumTransfersTodayByMember(ctx context.Context, memberUserID, businessID string) (int64, error)
}

type pgBusinessMemberRepo struct{ db DBTX }

func NewBusinessMemberRepository(db DBTX) BusinessMemberRepository {
	return &pgBusinessMemberRepo{db: db}
}

func (r *pgBusinessMemberRepo) Create(ctx context.Context, m *domain.BusinessMember) error {
	query := `
		INSERT INTO business_members (business_id, user_id, role_id, title, invited_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, joined_at, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		m.BusinessID, m.UserID, m.RoleID, m.Title, m.InvitedBy,
	).Scan(&m.ID, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
}

func (r *pgBusinessMemberRepo) GetByID(ctx context.Context, id string) (*domain.BusinessMember, error) {
	return r.scanMember(ctx,
		`SELECT id, business_id, user_id, role_id, title, invited_by,
			joined_at, is_active, removed_at, removed_by, created_at, updated_at
		FROM business_members WHERE id = $1`, id)
}

func (r *pgBusinessMemberRepo) GetByBusinessAndUser(ctx context.Context, businessID, userID string) (*domain.BusinessMember, error) {
	return r.scanMember(ctx,
		`SELECT id, business_id, user_id, role_id, title, invited_by,
			joined_at, is_active, removed_at, removed_by, created_at, updated_at
		FROM business_members WHERE business_id = $1 AND user_id = $2 AND is_active`, businessID, userID)
}

func (r *pgBusinessMemberRepo) UpdateRole(ctx context.Context, id, roleID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_members SET role_id = $2, updated_at = NOW() WHERE id = $1 AND is_active`, id, roleID)
	if err != nil {
		return fmt.Errorf("updating member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMemberNotFound
	}
	return nil
}

func (r *pgBusinessMemberRepo) Remove(ctx context.Context, id, removedBy string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_members SET is_active = FALSE, removed_at = NOW(), removed_by = $2, updated_at = NOW()
		WHERE id = $1 AND is_active`, id, removedBy)
	if err != nil {
		return fmt.Errorf("removing member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMemberNotFound
	}
	return nil
}

func (r *pgBusinessMemberRepo) ListByBusiness(ctx context.Context, businessID string) ([]domain.BusinessMember, error) {
	query := `
		SELECT id, business_id, user_id, role_id, title, invited_by,
			joined_at, is_active, removed_at, removed_by, created_at, updated_at
		FROM business_members WHERE business_id = $1 AND is_active
		ORDER BY joined_at ASC`
	return r.scanMembers(ctx, query, businessID)
}

func (r *pgBusinessMemberRepo) ListByUser(ctx context.Context, userID string) ([]domain.BusinessMember, error) {
	query := `
		SELECT id, business_id, user_id, role_id, title, invited_by,
			joined_at, is_active, removed_at, removed_by, created_at, updated_at
		FROM business_members WHERE user_id = $1 AND is_active
		ORDER BY joined_at ASC`
	return r.scanMembers(ctx, query, userID)
}

// SumTransfersTodayByMember totals all completed pending_transfers initiated by this member today.
func (r *pgBusinessMemberRepo) SumTransfersTodayByMember(ctx context.Context, memberUserID, businessID string) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM pending_transfers
		WHERE initiated_by = $1 AND business_id = $2
			AND status IN ('approved', 'pending')
			AND created_at >= CURRENT_DATE`, memberUserID, businessID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("summing daily transfers: %w", err)
	}
	return total, nil
}

func (r *pgBusinessMemberRepo) scanMember(ctx context.Context, query string, args ...any) (*domain.BusinessMember, error) {
	var m domain.BusinessMember
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&m.ID, &m.BusinessID, &m.UserID, &m.RoleID, &m.Title, &m.InvitedBy,
		&m.JoinedAt, &m.IsActive, &m.RemovedAt, &m.RemovedBy, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning member: %w", err)
	}
	return &m, nil
}

func (r *pgBusinessMemberRepo) scanMembers(ctx context.Context, query string, args ...any) ([]domain.BusinessMember, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}
	defer rows.Close()
	var result []domain.BusinessMember
	for rows.Next() {
		var m domain.BusinessMember
		if err := rows.Scan(
			&m.ID, &m.BusinessID, &m.UserID, &m.RoleID, &m.Title, &m.InvitedBy,
			&m.JoinedAt, &m.IsActive, &m.RemovedAt, &m.RemovedBy, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning member row: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

var _ BusinessMemberRepository = (*pgBusinessMemberRepo)(nil)
