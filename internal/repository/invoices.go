package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/jackc/pgx/v5"
)

type InvoiceRepository interface {
	Create(ctx context.Context, inv *domain.Invoice) error
	GetByID(ctx context.Context, id string) (*domain.Invoice, error)
	Update(ctx context.Context, inv *domain.Invoice) error
	UpdateStatus(ctx context.Context, id string, status domain.InvoiceStatus) error
	UpdatePaidCents(ctx context.Context, id string, paidCents int64, status domain.InvoiceStatus) error
	ListByBusiness(ctx context.Context, businessID string, status *string, limit, offset int) ([]domain.Invoice, error)
	NextInvoiceNumber(ctx context.Context, businessID string) (string, error)

	CreateLineItem(ctx context.Context, item *domain.InvoiceLineItem) error
	ListLineItems(ctx context.Context, invoiceID string) ([]domain.InvoiceLineItem, error)
	DeleteLineItems(ctx context.Context, invoiceID string) error

	Summary(ctx context.Context, businessID string) (*InvoiceSummary, error)
}

type InvoiceSummary struct {
	TotalOutstandingCents int64 `json:"totalOutstandingCents"`
	TotalOverdueCents     int64 `json:"totalOverdueCents"`
	PaidThisMonthCents    int64 `json:"paidThisMonthCents"`
	InvoiceCount          int   `json:"invoiceCount"`
}

type pgInvoiceRepo struct{ db DBTX }

func NewInvoiceRepository(db DBTX) InvoiceRepository {
	return &pgInvoiceRepo{db: db}
}

func (r *pgInvoiceRepo) Create(ctx context.Context, inv *domain.Invoice) error {
	var custPhone *string
	var custCC *string
	if inv.CustomerPhone != nil {
		e := inv.CustomerPhone.E164()
		custPhone = &e
		cc := inv.CustomerPhone.CountryCode
		custCC = &cc
	}
	return r.db.QueryRow(ctx,
		`INSERT INTO invoices (business_id, invoice_number, customer_name, customer_phone, customer_country_code, customer_email,
			customer_user_id, currency_code, subtotal_cents, tax_cents, total_cents, status,
			issue_date, due_date, notes, payment_link, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::date,$14::date,$15,$16,$17)
		RETURNING id, created_at, updated_at`,
		inv.BusinessID, inv.InvoiceNumber, inv.CustomerName, custPhone, custCC, inv.CustomerEmail,
		inv.CustomerUserID, inv.CurrencyCode, inv.SubtotalCents, inv.TaxCents, inv.TotalCents,
		inv.Status, inv.IssueDate, inv.DueDate, inv.Notes, inv.PaymentLink, inv.CreatedBy,
	).Scan(&inv.ID, &inv.CreatedAt, &inv.UpdatedAt)
}

func (r *pgInvoiceRepo) GetByID(ctx context.Context, id string) (*domain.Invoice, error) {
	var inv domain.Invoice
	var custPhone *string
	var custCC *string
	err := r.db.QueryRow(ctx,
		`SELECT id, business_id, invoice_number, customer_name, customer_phone, customer_country_code, customer_email,
			customer_user_id, currency_code, subtotal_cents, tax_cents, total_cents, paid_cents,
			status, issue_date::text, due_date::text, notes, payment_link, created_by, created_at, updated_at
		FROM invoices WHERE id = $1`, id).Scan(
		&inv.ID, &inv.BusinessID, &inv.InvoiceNumber, &inv.CustomerName, &custPhone, &custCC,
		&inv.CustomerEmail, &inv.CustomerUserID, &inv.CurrencyCode, &inv.SubtotalCents,
		&inv.TaxCents, &inv.TotalCents, &inv.PaidCents, &inv.Status,
		&inv.IssueDate, &inv.DueDate, &inv.Notes, &inv.PaymentLink, &inv.CreatedBy,
		&inv.CreatedAt, &inv.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrInvoiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting invoice: %w", err)
	}
	if custPhone != nil {
		p, _ := phone.Parse(*custPhone)
		inv.CustomerPhone = &p
	}
	return &inv, nil
}

func (r *pgInvoiceRepo) Update(ctx context.Context, inv *domain.Invoice) error {
	var custPhone *string
	var custCC *string
	if inv.CustomerPhone != nil {
		e := inv.CustomerPhone.E164()
		custPhone = &e
		cc := inv.CustomerPhone.CountryCode
		custCC = &cc
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE invoices SET customer_name=$2, customer_phone=$3, customer_country_code=$4, customer_email=$5,
			subtotal_cents=$6, tax_cents=$7, total_cents=$8, due_date=$9::date, notes=$10, updated_at=NOW()
		WHERE id=$1 AND status='draft'`,
		inv.ID, inv.CustomerName, custPhone, custCC, inv.CustomerEmail,
		inv.SubtotalCents, inv.TaxCents, inv.TotalCents, inv.DueDate, inv.Notes)
	if err != nil {
		return fmt.Errorf("updating invoice: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvoiceNotFound
	}
	return nil
}

func (r *pgInvoiceRepo) UpdateStatus(ctx context.Context, id string, status domain.InvoiceStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE invoices SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return fmt.Errorf("updating invoice status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvoiceNotFound
	}
	return nil
}

func (r *pgInvoiceRepo) UpdatePaidCents(ctx context.Context, id string, paidCents int64, status domain.InvoiceStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE invoices SET paid_cents=$2, status=$3, updated_at=NOW() WHERE id=$1`, id, paidCents, status)
	if err != nil {
		return fmt.Errorf("updating paid cents: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvoiceNotFound
	}
	return nil
}

func (r *pgInvoiceRepo) ListByBusiness(ctx context.Context, businessID string, status *string, limit, offset int) ([]domain.Invoice, error) {
	query := `
		SELECT id, business_id, invoice_number, customer_name, customer_phone, customer_country_code, customer_email,
			customer_user_id, currency_code, subtotal_cents, tax_cents, total_cents, paid_cents,
			status, issue_date::text, due_date::text, notes, payment_link, created_by, created_at, updated_at
		FROM invoices WHERE business_id = $1 AND ($2::text IS NULL OR status::text = $2)
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(ctx, query, businessID, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}
	defer rows.Close()
	var result []domain.Invoice
	for rows.Next() {
		var inv domain.Invoice
		var custPhone *string
		var custCC *string
		if err := rows.Scan(
			&inv.ID, &inv.BusinessID, &inv.InvoiceNumber, &inv.CustomerName, &custPhone, &custCC,
			&inv.CustomerEmail, &inv.CustomerUserID, &inv.CurrencyCode, &inv.SubtotalCents,
			&inv.TaxCents, &inv.TotalCents, &inv.PaidCents, &inv.Status,
			&inv.IssueDate, &inv.DueDate, &inv.Notes, &inv.PaymentLink, &inv.CreatedBy,
			&inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning invoice: %w", err)
		}
		if custPhone != nil {
			p, _ := phone.Parse(*custPhone)
			inv.CustomerPhone = &p
		}
		result = append(result, inv)
	}
	return result, rows.Err()
}

func (r *pgInvoiceRepo) NextInvoiceNumber(ctx context.Context, businessID string) (string, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoices WHERE business_id = $1`, businessID).Scan(&count)
	if err != nil {
		return "", fmt.Errorf("counting invoices: %w", err)
	}
	return fmt.Sprintf("INV-2026-%05d", count+1), nil
}

func (r *pgInvoiceRepo) CreateLineItem(ctx context.Context, item *domain.InvoiceLineItem) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO invoice_line_items (invoice_id, description, quantity, unit_price_cents, total_cents, category_id, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`,
		item.InvoiceID, item.Description, item.Quantity, item.UnitPriceCents, item.TotalCents, item.CategoryID, item.SortOrder,
	).Scan(&item.ID, &item.CreatedAt)
}

func (r *pgInvoiceRepo) ListLineItems(ctx context.Context, invoiceID string) ([]domain.InvoiceLineItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, invoice_id, description, quantity, unit_price_cents, total_cents, category_id, sort_order, created_at
		FROM invoice_line_items WHERE invoice_id = $1 ORDER BY sort_order`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("listing line items: %w", err)
	}
	defer rows.Close()
	var result []domain.InvoiceLineItem
	for rows.Next() {
		var item domain.InvoiceLineItem
		if err := rows.Scan(&item.ID, &item.InvoiceID, &item.Description, &item.Quantity,
			&item.UnitPriceCents, &item.TotalCents, &item.CategoryID, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning line item: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *pgInvoiceRepo) DeleteLineItems(ctx context.Context, invoiceID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM invoice_line_items WHERE invoice_id = $1`, invoiceID)
	if err != nil {
		return fmt.Errorf("deleting line items: %w", err)
	}
	return nil
}

func (r *pgInvoiceRepo) Summary(ctx context.Context, businessID string) (*InvoiceSummary, error) {
	var s InvoiceSummary
	err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status NOT IN ('paid','cancelled') THEN total_cents - paid_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'overdue' THEN total_cents - paid_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' AND updated_at >= DATE_TRUNC('month', NOW()) THEN paid_cents ELSE 0 END), 0),
			COUNT(*)
		FROM invoices WHERE business_id = $1`, businessID).Scan(
		&s.TotalOutstandingCents, &s.TotalOverdueCents, &s.PaidThisMonthCents, &s.InvoiceCount)
	if err != nil {
		return nil, fmt.Errorf("invoice summary: %w", err)
	}
	return &s, nil
}

var _ InvoiceRepository = (*pgInvoiceRepo)(nil)
