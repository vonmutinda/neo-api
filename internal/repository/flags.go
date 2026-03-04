package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type FlagRepository interface {
	Create(ctx context.Context, f *domain.CustomerFlag) error
	GetByID(ctx context.Context, id string) (*domain.CustomerFlag, error)
	ListAll(ctx context.Context, filter domain.FlagFilter) (*domain.PaginatedResult[domain.CustomerFlag], error)
	ListByUser(ctx context.Context, userID string) ([]domain.CustomerFlag, error)
	Resolve(ctx context.Context, id, resolvedBy, note string) error
	CountOpen(ctx context.Context) (int64, error)
}

type pgFlagRepo struct{ db DBTX }

func NewFlagRepository(db DBTX) FlagRepository { return &pgFlagRepo{db: db} }

func (r *pgFlagRepo) Create(ctx context.Context, f *domain.CustomerFlag) error {
	query := `
		INSERT INTO customer_flags (user_id, flag_type, severity, description, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		f.UserID, f.FlagType, f.Severity, f.Description, f.CreatedBy,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
}

func (r *pgFlagRepo) GetByID(ctx context.Context, id string) (*domain.CustomerFlag, error) {
	var f domain.CustomerFlag
	err := r.db.QueryRow(ctx, `SELECT * FROM customer_flags WHERE id = $1`, id).Scan(
		&f.ID, &f.UserID, &f.FlagType, &f.Severity, &f.Description,
		&f.CreatedBy, &f.IsResolved, &f.ResolvedBy, &f.ResolvedAt,
		&f.ResolutionNote, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrFlagNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting flag: %w", err)
	}
	return &f, nil
}

func (r *pgFlagRepo) ListAll(ctx context.Context, filter domain.FlagFilter) (*domain.PaginatedResult[domain.CustomerFlag], error) {
	limit, offset := domain.NormalizePagination(filter.Limit, filter.Offset)

	where, args := buildFlagWhere(filter)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM customer_flags`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting flags: %w", err)
	}

	dir := "DESC"
	if strings.EqualFold(filter.SortOrder, "asc") {
		dir = "ASC"
	}
	dataQuery := `SELECT * FROM customer_flags` + where +
		fmt.Sprintf(" ORDER BY created_at %s LIMIT %d OFFSET %d", dir, limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("listing flags: %w", err)
	}
	defer rows.Close()

	var flags []domain.CustomerFlag
	for rows.Next() {
		var f domain.CustomerFlag
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.FlagType, &f.Severity, &f.Description,
			&f.CreatedBy, &f.IsResolved, &f.ResolvedBy, &f.ResolvedAt,
			&f.ResolutionNote, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning flag: %w", err)
		}
		flags = append(flags, f)
	}

	return domain.NewPaginatedResult(flags, total, limit, offset), nil
}

func (r *pgFlagRepo) ListByUser(ctx context.Context, userID string) ([]domain.CustomerFlag, error) {
	rows, err := r.db.Query(ctx,
		`SELECT * FROM customer_flags WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing flags by user: %w", err)
	}
	defer rows.Close()

	var flags []domain.CustomerFlag
	for rows.Next() {
		var f domain.CustomerFlag
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.FlagType, &f.Severity, &f.Description,
			&f.CreatedBy, &f.IsResolved, &f.ResolvedBy, &f.ResolvedAt,
			&f.ResolutionNote, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning flag: %w", err)
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

func (r *pgFlagRepo) Resolve(ctx context.Context, id, resolvedBy, note string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE customer_flags
		SET is_resolved = TRUE, resolved_by = $2, resolved_at = NOW(),
		    resolution_note = $3, updated_at = NOW()
		WHERE id = $1 AND NOT is_resolved`,
		id, resolvedBy, note)
	if err != nil {
		return fmt.Errorf("resolving flag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFlagNotFound
	}
	return nil
}

func (r *pgFlagRepo) CountOpen(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM customer_flags WHERE NOT is_resolved`).Scan(&count)
	return count, err
}

func buildFlagWhere(f domain.FlagFilter) (string, []any) {
	var conditions []string
	var args []any
	idx := 1

	if f.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.FlagType != nil {
		conditions = append(conditions, fmt.Sprintf("flag_type = $%d", idx))
		args = append(args, *f.FlagType)
		idx++
	}
	if f.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", idx))
		args = append(args, *f.Severity)
		idx++
	}
	if f.Resolved != nil {
		conditions = append(conditions, fmt.Sprintf("is_resolved = $%d", idx))
		args = append(args, *f.Resolved)
		idx++
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

var _ FlagRepository = (*pgFlagRepo)(nil)
