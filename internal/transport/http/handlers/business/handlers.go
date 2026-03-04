package business

import (
	bizsvc "github.com/vonmutinda/neo/internal/services/business"

	"github.com/vonmutinda/neo/internal/repository"
)

type Handlers struct {
	Core             *BusinessHandler
	Categories       *CategoryHandler
	TaxPots          *TaxPotHandler
	PendingTransfers *PendingTransferHandler
	BatchPayments    *BatchPaymentHandler
	Invoices         *InvoiceHandler
	Documents        *DocumentHandler
	Loans            *BusinessLoanHandler
	Accounting       *AccountingHandler
}

func NewHandlers(
	businessSvc *bizsvc.Service,
	catRepo repository.TransactionCategoryRepository,
	labelRepo repository.TransactionLabelRepository,
	taxPotRepo repository.TaxPotRepository,
	potRepo repository.PotRepository,
	ptRepo repository.PendingTransferRepository,
	auditRepo repository.AuditRepository,
	batchRepo repository.BatchPaymentRepository,
	invoiceRepo repository.InvoiceRepository,
	docRepo repository.BusinessDocumentRepository,
	bizLoanRepo repository.BusinessLoanRepository,
) *Handlers {
	return &Handlers{
		Core:             NewBusinessHandler(businessSvc),
		Categories:       NewCategoryHandler(catRepo, labelRepo),
		TaxPots:          NewTaxPotHandler(taxPotRepo, potRepo),
		PendingTransfers: NewPendingTransferHandler(ptRepo, auditRepo),
		BatchPayments:    NewBatchPaymentHandler(batchRepo, auditRepo),
		Invoices:         NewInvoiceHandler(invoiceRepo),
		Documents:        NewDocumentHandler(docRepo),
		Loans:            NewBusinessLoanHandler(bizLoanRepo),
		Accounting:       NewAccountingHandler(invoiceRepo, labelRepo),
	}
}
