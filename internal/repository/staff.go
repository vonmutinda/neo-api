package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type StaffRepository interface {
	Create(ctx context.Context, s *domain.Staff) error
	GetByID(ctx context.Context, id string) (*domain.Staff, error)
	GetByEmail(ctx context.Context, email string) (*domain.Staff, error)
	ListAll(ctx context.Context, filter domain.StaffFilter) (*domain.PaginatedResult[domain.Staff], error)
	Update(ctx context.Context, s *domain.Staff) error
	Deactivate(ctx context.Context, id string) error
	UpdateLastLogin(ctx context.Context, id string) error
}

type pgStaffRepo struct{ db DBTX }

func NewStaffRepository(db DBTX) StaffRepository { return &pgStaffRepo{db: db} }

func (r *pgStaffRepo) Create(ctx context.Context, s *domain.Staff) error {
	query := `
		INSERT INTO staff (id, email, full_name, role, department, password_hash, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		s.ID, s.Email, s.FullName, s.Role, s.Department, s.PasswordHash, s.CreatedBy,
	).Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *pgStaffRepo) GetByID(ctx context.Context, id string) (*domain.Staff, error) {
	return r.scanStaff(ctx, `SELECT * FROM staff WHERE id = $1`, id)
}

func (r *pgStaffRepo) GetByEmail(ctx context.Context, email string) (*domain.Staff, error) {
	return r.scanStaff(ctx, `SELECT * FROM staff WHERE email = $1`, email)
}

func (r *pgStaffRepo) ListAll(ctx context.Context, filter domain.StaffFilter) (*domain.PaginatedResult[domain.Staff], error) {
	limit, offset := domain.NormalizePagination(filter.Limit, filter.Offset)

	where, args := buildStaffWhere(filter)

	countQuery := `SELECT COUNT(*) FROM staff` + where
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting staff: %w", err)
	}

	orderBy := " ORDER BY created_at DESC"
	if filter.SortBy != "" {
		col := sanitizeStaffSortColumn(filter.SortBy)
		dir := "DESC"
		if strings.EqualFold(filter.SortOrder, "asc") {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf(" ORDER BY %s %s", col, dir)
	}

	dataQuery := `SELECT * FROM staff` + where + orderBy +
		fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("listing staff: %w", err)
	}
	defer rows.Close()

	var staff []domain.Staff
	for rows.Next() {
		s, err := scanStaffRow(rows)
		if err != nil {
			return nil, err
		}
		staff = append(staff, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating staff rows: %w", err)
	}

	return domain.NewPaginatedResult(staff, total, limit, offset), nil
}

func (r *pgStaffRepo) Update(ctx context.Context, s *domain.Staff) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE staff SET full_name = $2, role = $3, department = $4, password_hash = $5, updated_at = NOW()
		WHERE id = $1`,
		s.ID, s.FullName, s.Role, s.Department, s.PasswordHash)
	if err != nil {
		return fmt.Errorf("updating staff: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStaffNotFound
	}
	return nil
}

func (r *pgStaffRepo) Deactivate(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE staff SET is_active = FALSE, deactivated_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deactivating staff: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStaffNotFound
	}
	return nil
}

func (r *pgStaffRepo) UpdateLastLogin(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE staff SET last_login_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *pgStaffRepo) scanStaff(ctx context.Context, query string, args ...any) (*domain.Staff, error) {
	row := r.db.QueryRow(ctx, query, args...)
	var s domain.Staff
	err := row.Scan(
		&s.ID, &s.Email, &s.FullName, &s.Role, &s.Department,
		&s.IsActive, &s.PasswordHash, &s.LastLoginAt, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.DeactivatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrStaffNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning staff: %w", err)
	}
	return &s, nil
}

func scanStaffRow(rows pgx.Rows) (*domain.Staff, error) {
	var s domain.Staff
	err := rows.Scan(
		&s.ID, &s.Email, &s.FullName, &s.Role, &s.Department,
		&s.IsActive, &s.PasswordHash, &s.LastLoginAt, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.DeactivatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning staff row: %w", err)
	}
	return &s, nil
}

func buildStaffWhere(f domain.StaffFilter) (string, []any) {
	var conditions []string
	var args []any
	idx := 1

	if f.Search != nil && *f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(email ILIKE $%d OR full_name ILIKE $%d)", idx, idx))
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	if f.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", idx))
		args = append(args, *f.Role)
		idx++
	}
	if f.Department != nil {
		conditions = append(conditions, fmt.Sprintf("department = $%d", idx))
		args = append(args, *f.Department)
		idx++
	}
	if f.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *f.IsActive)
		idx++
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func sanitizeStaffSortColumn(col string) string {
	switch col {
	case "email", "full_name", "role", "department", "created_at", "last_login_at":
		return col
	default:
		return "created_at"
	}
}

var _ StaffRepository = (*pgStaffRepo)(nil)
