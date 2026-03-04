package business

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type AccountingHandler struct {
	invoiceRepo repository.InvoiceRepository
	labelRepo   repository.TransactionLabelRepository
}

func NewAccountingHandler(invoiceRepo repository.InvoiceRepository, labelRepo repository.TransactionLabelRepository) *AccountingHandler {
	return &AccountingHandler{invoiceRepo: invoiceRepo, labelRepo: labelRepo}
}

func (h *AccountingHandler) ExportTransactions(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	labels, err := h.labelRepo.ListLabeled(r.Context(), biz.ID, nil, nil, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, labels)
}

func (h *AccountingHandler) ProfitAndLoss(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	summary, err := h.labelRepo.TaxSummary(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"categories": summary})
}

func (h *AccountingHandler) BalanceSheet(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "balance sheet endpoint"})
}

func (h *AccountingHandler) TaxReport(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	summary, err := h.labelRepo.TaxSummary(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"taxDeductible": summary})
}
