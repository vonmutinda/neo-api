package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
)

type ReconciliationRepository interface {
	CreateException(ctx context.Context, exc *domain.ReconException) error
	ListOpenExceptions(ctx context.Context) ([]domain.ReconException, error)
	ResolveException(ctx context.Context, id, notes, action string) error
	CreateRun(ctx context.Context, run *domain.ReconRun) error
	FinishRun(ctx context.Context, id string, matched, exceptions int, errMsg *string) error
	GetRunByDate(ctx context.Context, date time.Time) (*domain.ReconRun, error)
}

type pgReconRepo struct{ db DBTX }

func NewReconciliationRepository(db DBTX) ReconciliationRepository {
	return &pgReconRepo{db: db}
}

func (r *pgReconRepo) CreateException(ctx context.Context, exc *domain.ReconException) error {
	query := `
		INSERT INTO reconciliation_exceptions (
			ethswitch_reference, idempotency_key, error_type,
			ethswitch_reported_amount_cents, ledger_reported_amount_cents,
			postgres_reported_amount_cents, amount_difference_cents,
			recon_run_date, clearing_file_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		exc.EthSwitchReference, exc.IdempotencyKey, exc.ErrorType,
		exc.EthSwitchReportedAmountCents, exc.LedgerReportedAmountCents,
		exc.PostgresReportedAmountCents, exc.AmountDifferenceCents,
		exc.ReconRunDate, exc.ClearingFileName,
	).Scan(&exc.ID, &exc.CreatedAt, &exc.UpdatedAt)
}

func (r *pgReconRepo) ListOpenExceptions(ctx context.Context) ([]domain.ReconException, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, ethswitch_reference, idempotency_key, error_type,
			ethswitch_reported_amount_cents, ledger_reported_amount_cents,
			postgres_reported_amount_cents, amount_difference_cents,
			status, assigned_to, resolution_notes, resolution_action,
			recon_run_date, clearing_file_name, created_at, resolved_at, updated_at
		FROM reconciliation_exceptions WHERE status IN ('open','investigating') ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing open exceptions: %w", err)
	}
	defer rows.Close()
	var result []domain.ReconException
	for rows.Next() {
		var e domain.ReconException
		if err := rows.Scan(&e.ID, &e.EthSwitchReference, &e.IdempotencyKey, &e.ErrorType,
			&e.EthSwitchReportedAmountCents, &e.LedgerReportedAmountCents,
			&e.PostgresReportedAmountCents, &e.AmountDifferenceCents,
			&e.Status, &e.AssignedTo, &e.ResolutionNotes, &e.ResolutionAction,
			&e.ReconRunDate, &e.ClearingFileName, &e.CreatedAt, &e.ResolvedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning exception: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *pgReconRepo) ResolveException(ctx context.Context, id, notes, action string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE reconciliation_exceptions SET status='resolved', resolution_notes=$2, resolution_action=$3, resolved_at=NOW(), updated_at=NOW() WHERE id=$1`,
		id, notes, action)
	return err
}

func (r *pgReconRepo) CreateRun(ctx context.Context, run *domain.ReconRun) error {
	query := `INSERT INTO reconciliation_runs (run_date, clearing_file_name) VALUES ($1, $2) RETURNING id, started_at, created_at`
	return r.db.QueryRow(ctx, query, run.RunDate, run.ClearingFileName).Scan(&run.ID, &run.StartedAt, &run.CreatedAt)
}

func (r *pgReconRepo) FinishRun(ctx context.Context, id string, matched, exceptions int, errMsg *string) error {
	status := "completed"
	if errMsg != nil {
		status = "failed"
	}
	_, err := r.db.Exec(ctx,
		`UPDATE reconciliation_runs SET matched_count=$2, exception_count=$3, total_records=$2+$3, status=$4, error_message=$5, finished_at=NOW() WHERE id=$1`,
		id, matched, exceptions, status, errMsg)
	return err
}

func (r *pgReconRepo) GetRunByDate(ctx context.Context, date time.Time) (*domain.ReconRun, error) {
	var run domain.ReconRun
	err := r.db.QueryRow(ctx, `SELECT id, run_date, clearing_file_name, total_records, matched_count, exception_count, started_at, finished_at, status, error_message, created_at FROM reconciliation_runs WHERE run_date = $1`, date).Scan(
		&run.ID, &run.RunDate, &run.ClearingFileName, &run.TotalRecords, &run.MatchedCount, &run.ExceptionCount,
		&run.StartedAt, &run.FinishedAt, &run.Status, &run.ErrorMessage, &run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting recon run: %w", err)
	}
	return &run, nil
}

var _ ReconciliationRepository = (*pgReconRepo)(nil)
