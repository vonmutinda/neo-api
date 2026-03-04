package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type BusinessRoleRepository interface {
	Create(ctx context.Context, role *domain.BusinessRole) error
	GetByID(ctx context.Context, id string) (*domain.BusinessRole, error)
	Update(ctx context.Context, role *domain.BusinessRole) error
	Delete(ctx context.Context, id string) error
	ListByBusiness(ctx context.Context, businessID string) ([]domain.BusinessRole, error)
	GetSystemRoleByName(ctx context.Context, name string) (*domain.BusinessRole, error)
	SetPermissions(ctx context.Context, roleID string, perms []domain.BusinessPermission) error
	GetPermissions(ctx context.Context, roleID string) ([]domain.BusinessPermission, error)
	CountMembersByRole(ctx context.Context, roleID string) (int, error)
}

type pgBusinessRoleRepo struct{ db DBTX }

func NewBusinessRoleRepository(db DBTX) BusinessRoleRepository {
	return &pgBusinessRoleRepo{db: db}
}

func (r *pgBusinessRoleRepo) Create(ctx context.Context, role *domain.BusinessRole) error {
	query := `
		INSERT INTO business_roles (business_id, name, description, is_system, is_default,
			max_transfer_cents, daily_transfer_limit_cents, requires_approval_above_cents)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		role.BusinessID, role.Name, role.Description, role.IsSystem, role.IsDefault,
		role.MaxTransferCents, role.DailyTransferLimitCents, role.RequiresApprovalAboveCents,
	).Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)
}

func (r *pgBusinessRoleRepo) GetByID(ctx context.Context, id string) (*domain.BusinessRole, error) {
	role, err := r.scanRole(ctx,
		`SELECT id, business_id, name, description, is_system, is_default,
			max_transfer_cents, daily_transfer_limit_cents, requires_approval_above_cents,
			created_at, updated_at
		FROM business_roles WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	perms, err := r.GetPermissions(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("loading role permissions: %w", err)
	}
	role.Permissions = perms
	return role, nil
}

func (r *pgBusinessRoleRepo) Update(ctx context.Context, role *domain.BusinessRole) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_roles SET
			name = $2, description = $3,
			max_transfer_cents = $4, daily_transfer_limit_cents = $5,
			requires_approval_above_cents = $6, updated_at = NOW()
		WHERE id = $1 AND NOT is_system`,
		role.ID, role.Name, role.Description,
		role.MaxTransferCents, role.DailyTransferLimitCents, role.RequiresApprovalAboveCents)
	if err != nil {
		return fmt.Errorf("updating business role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSystemRoleImmutable
	}
	return nil
}

func (r *pgBusinessRoleRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM business_roles WHERE id = $1 AND NOT is_system`, id)
	if err != nil {
		return fmt.Errorf("deleting business role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSystemRoleImmutable
	}
	return nil
}

func (r *pgBusinessRoleRepo) ListByBusiness(ctx context.Context, businessID string) ([]domain.BusinessRole, error) {
	query := `
		SELECT id, business_id, name, description, is_system, is_default,
			max_transfer_cents, daily_transfer_limit_cents, requires_approval_above_cents,
			created_at, updated_at
		FROM business_roles
		WHERE business_id = $1 OR business_id IS NULL
		ORDER BY is_system DESC, name ASC`
	rows, err := r.db.Query(ctx, query, businessID)
	if err != nil {
		return nil, fmt.Errorf("listing business roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.BusinessRole
	for rows.Next() {
		var role domain.BusinessRole
		if err := rows.Scan(
			&role.ID, &role.BusinessID, &role.Name, &role.Description,
			&role.IsSystem, &role.IsDefault,
			&role.MaxTransferCents, &role.DailyTransferLimitCents, &role.RequiresApprovalAboveCents,
			&role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning role row: %w", err)
		}
		perms, err := r.GetPermissions(ctx, role.ID)
		if err != nil {
			return nil, fmt.Errorf("loading permissions for role %s: %w", role.ID, err)
		}
		role.Permissions = perms
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *pgBusinessRoleRepo) GetSystemRoleByName(ctx context.Context, name string) (*domain.BusinessRole, error) {
	role, err := r.scanRole(ctx,
		`SELECT id, business_id, name, description, is_system, is_default,
			max_transfer_cents, daily_transfer_limit_cents, requires_approval_above_cents,
			created_at, updated_at
		FROM business_roles WHERE name = $1 AND is_system AND business_id IS NULL`, name)
	if err != nil {
		return nil, err
	}
	perms, err := r.GetPermissions(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("loading role permissions: %w", err)
	}
	role.Permissions = perms
	return role, nil
}

func (r *pgBusinessRoleRepo) SetPermissions(ctx context.Context, roleID string, perms []domain.BusinessPermission) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM business_role_permissions WHERE role_id = $1`, roleID); err != nil {
		return fmt.Errorf("clearing permissions: %w", err)
	}
	for _, p := range perms {
		if _, err := r.db.Exec(ctx,
			`INSERT INTO business_role_permissions (role_id, permission) VALUES ($1, $2)`,
			roleID, string(p)); err != nil {
			return fmt.Errorf("inserting permission %s: %w", p, err)
		}
	}
	return nil
}

func (r *pgBusinessRoleRepo) GetPermissions(ctx context.Context, roleID string) ([]domain.BusinessPermission, error) {
	rows, err := r.db.Query(ctx,
		`SELECT permission FROM business_role_permissions WHERE role_id = $1 ORDER BY permission`, roleID)
	if err != nil {
		return nil, fmt.Errorf("querying permissions: %w", err)
	}
	defer rows.Close()
	var perms []domain.BusinessPermission
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning permission: %w", err)
		}
		perms = append(perms, domain.BusinessPermission(p))
	}
	return perms, rows.Err()
}

func (r *pgBusinessRoleRepo) CountMembersByRole(ctx context.Context, roleID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM business_members WHERE role_id = $1 AND is_active`, roleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting members by role: %w", err)
	}
	return count, nil
}

func (r *pgBusinessRoleRepo) scanRole(ctx context.Context, query string, args ...any) (*domain.BusinessRole, error) {
	var role domain.BusinessRole
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&role.ID, &role.BusinessID, &role.Name, &role.Description,
		&role.IsSystem, &role.IsDefault,
		&role.MaxTransferCents, &role.DailyTransferLimitCents, &role.RequiresApprovalAboveCents,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning role: %w", err)
	}
	return &role, nil
}

var _ BusinessRoleRepository = (*pgBusinessRoleRepo)(nil)
