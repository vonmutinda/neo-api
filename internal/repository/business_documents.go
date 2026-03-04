package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type BusinessDocumentRepository interface {
	Create(ctx context.Context, doc *domain.BusinessDocument) error
	GetByID(ctx context.Context, id string) (*domain.BusinessDocument, error)
	Update(ctx context.Context, doc *domain.BusinessDocument) error
	Archive(ctx context.Context, id string) error
	ListByBusiness(ctx context.Context, businessID string, docType *string, limit, offset int) ([]domain.BusinessDocument, error)
	ListExpiring(ctx context.Context, businessID string, daysAhead int) ([]domain.BusinessDocument, error)
}

type pgBusinessDocRepo struct{ db DBTX }

func NewBusinessDocumentRepository(db DBTX) BusinessDocumentRepository {
	return &pgBusinessDocRepo{db: db}
}

func (r *pgBusinessDocRepo) Create(ctx context.Context, doc *domain.BusinessDocument) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO business_documents (business_id, name, document_type, file_key, file_size_bytes,
			mime_type, uploaded_by, description, tags, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date) RETURNING id, created_at, updated_at`,
		doc.BusinessID, doc.Name, doc.DocumentType, doc.FileKey, doc.FileSizeBytes,
		doc.MimeType, doc.UploadedBy, doc.Description, doc.Tags, doc.ExpiresAt,
	).Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)
}

func (r *pgBusinessDocRepo) GetByID(ctx context.Context, id string) (*domain.BusinessDocument, error) {
	var doc domain.BusinessDocument
	err := r.db.QueryRow(ctx,
		`SELECT id, business_id, name, document_type, file_key, file_size_bytes, mime_type,
			uploaded_by, description, tags, is_archived, expires_at, created_at, updated_at
		FROM business_documents WHERE id = $1`, id).Scan(
		&doc.ID, &doc.BusinessID, &doc.Name, &doc.DocumentType, &doc.FileKey,
		&doc.FileSizeBytes, &doc.MimeType, &doc.UploadedBy, &doc.Description,
		&doc.Tags, &doc.IsArchived, &doc.ExpiresAt, &doc.CreatedAt, &doc.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting document: %w", err)
	}
	return &doc, nil
}

func (r *pgBusinessDocRepo) Update(ctx context.Context, doc *domain.BusinessDocument) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_documents SET name=$2, description=$3, tags=$4, updated_at=NOW()
		WHERE id=$1 AND NOT is_archived`, doc.ID, doc.Name, doc.Description, doc.Tags)
	if err != nil {
		return fmt.Errorf("updating document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *pgBusinessDocRepo) Archive(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_documents SET is_archived=TRUE, updated_at=NOW() WHERE id=$1 AND NOT is_archived`, id)
	if err != nil {
		return fmt.Errorf("archiving document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *pgBusinessDocRepo) ListByBusiness(ctx context.Context, businessID string, docType *string, limit, offset int) ([]domain.BusinessDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, business_id, name, document_type, file_key, file_size_bytes, mime_type,
			uploaded_by, description, tags, is_archived, expires_at, created_at, updated_at
		FROM business_documents
		WHERE business_id = $1 AND NOT is_archived AND ($2::text IS NULL OR document_type::text = $2)
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`, businessID, docType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	defer rows.Close()
	var result []domain.BusinessDocument
	for rows.Next() {
		var doc domain.BusinessDocument
		if err := rows.Scan(&doc.ID, &doc.BusinessID, &doc.Name, &doc.DocumentType, &doc.FileKey,
			&doc.FileSizeBytes, &doc.MimeType, &doc.UploadedBy, &doc.Description,
			&doc.Tags, &doc.IsArchived, &doc.ExpiresAt, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		result = append(result, doc)
	}
	return result, rows.Err()
}

func (r *pgBusinessDocRepo) ListExpiring(ctx context.Context, businessID string, daysAhead int) ([]domain.BusinessDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, business_id, name, document_type, file_key, file_size_bytes, mime_type,
			uploaded_by, description, tags, is_archived, expires_at, created_at, updated_at
		FROM business_documents
		WHERE business_id = $1 AND NOT is_archived AND expires_at IS NOT NULL
			AND expires_at <= CURRENT_DATE + $2 * INTERVAL '1 day'
		ORDER BY expires_at ASC`, businessID, daysAhead)
	if err != nil {
		return nil, fmt.Errorf("listing expiring documents: %w", err)
	}
	defer rows.Close()
	var result []domain.BusinessDocument
	for rows.Next() {
		var doc domain.BusinessDocument
		if err := rows.Scan(&doc.ID, &doc.BusinessID, &doc.Name, &doc.DocumentType, &doc.FileKey,
			&doc.FileSizeBytes, &doc.MimeType, &doc.UploadedBy, &doc.Description,
			&doc.Tags, &doc.IsArchived, &doc.ExpiresAt, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning expiring document: %w", err)
		}
		result = append(result, doc)
	}
	return result, rows.Err()
}

var _ BusinessDocumentRepository = (*pgBusinessDocRepo)(nil)
