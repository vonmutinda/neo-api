package business

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type InvoiceHandler struct {
	invoiceRepo repository.InvoiceRepository
}

func NewInvoiceHandler(invoiceRepo repository.InvoiceRepository) *InvoiceHandler {
	return &InvoiceHandler{invoiceRepo: invoiceRepo}
}

func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		CustomerName  string             `json:"customerName"`
		CustomerPhone *phone.PhoneNumber `json:"customerPhone,omitempty"`
		CustomerEmail string             `json:"customerEmail,omitempty"`
		CurrencyCode  string `json:"currencyCode"`
		SubtotalCents int64  `json:"subtotalCents"`
		TaxCents      int64  `json:"taxCents"`
		TotalCents    int64  `json:"totalCents"`
		IssueDate     string `json:"issueDate"`
		DueDate       string `json:"dueDate"`
		Notes         string `json:"notes,omitempty"`
		LineItems     []struct {
			Description    string  `json:"description"`
			Quantity       float64 `json:"quantity"`
			UnitPriceCents int64   `json:"unitPriceCents"`
			TotalCents     int64   `json:"totalCents"`
			CategoryID     *string `json:"categoryId,omitempty"`
			SortOrder      int     `json:"sortOrder"`
		} `json:"lineItems,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	invoiceNumber, err := h.invoiceRepo.NextInvoiceNumber(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	currency := req.CurrencyCode
	if currency == "" {
		currency = "ETB"
	}

	phone := req.CustomerPhone
	email := strPtrOrNil(req.CustomerEmail)
	notes := strPtrOrNil(req.Notes)

	inv := &domain.Invoice{
		BusinessID:    biz.ID,
		InvoiceNumber: invoiceNumber,
		CustomerName:  req.CustomerName,
		CustomerPhone: phone,
		CustomerEmail: email,
		CurrencyCode:  currency,
		SubtotalCents: req.SubtotalCents,
		TaxCents:      req.TaxCents,
		TotalCents:    req.TotalCents,
		Status:        domain.InvoiceDraft,
		IssueDate:     req.IssueDate,
		DueDate:       req.DueDate,
		Notes:         notes,
		CreatedBy:     userID,
	}
	if err := h.invoiceRepo.Create(r.Context(), inv); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	for _, li := range req.LineItems {
		item := &domain.InvoiceLineItem{
			InvoiceID:      inv.ID,
			Description:    li.Description,
			Quantity:       li.Quantity,
			UnitPriceCents: li.UnitPriceCents,
			TotalCents:     li.TotalCents,
			CategoryID:     li.CategoryID,
			SortOrder:      li.SortOrder,
		}
		if err := h.invoiceRepo.CreateLineItem(r.Context(), item); err != nil {
			httputil.HandleError(w, r, err)
			return
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, inv)
}

func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	status := r.URL.Query().Get("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	invoices, err := h.invoiceRepo.ListByBusiness(r.Context(), biz.ID, statusPtr, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, invoices)
}

func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	invID := chi.URLParam(r, "invId")
	inv, err := h.invoiceRepo.GetByID(r.Context(), invID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	items, err := h.invoiceRepo.ListLineItems(r.Context(), invID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	inv.LineItems = items
	httputil.WriteJSON(w, http.StatusOK, inv)
}

func (h *InvoiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	invID := chi.URLParam(r, "invId")
	var req struct {
		CustomerName  string             `json:"customerName,omitempty"`
		CustomerPhone *phone.PhoneNumber `json:"customerPhone,omitempty"`
		CustomerEmail string             `json:"customerEmail,omitempty"`
		SubtotalCents int64  `json:"subtotalCents,omitempty"`
		TaxCents      int64  `json:"taxCents,omitempty"`
		TotalCents    int64  `json:"totalCents,omitempty"`
		DueDate       string `json:"dueDate,omitempty"`
		Notes         string `json:"notes,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	inv := &domain.Invoice{
		ID:            invID,
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		CustomerEmail: strPtrOrNil(req.CustomerEmail),
		SubtotalCents: req.SubtotalCents,
		TaxCents:      req.TaxCents,
		TotalCents:    req.TotalCents,
		DueDate:       req.DueDate,
		Notes:         strPtrOrNil(req.Notes),
	}
	if err := h.invoiceRepo.Update(r.Context(), inv); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, inv)
}

func (h *InvoiceHandler) Send(w http.ResponseWriter, r *http.Request) {
	invID := chi.URLParam(r, "invId")
	if err := h.invoiceRepo.UpdateStatus(r.Context(), invID, domain.InvoiceSent); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *InvoiceHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	invID := chi.URLParam(r, "invId")
	if err := h.invoiceRepo.UpdateStatus(r.Context(), invID, domain.InvoiceCancelled); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *InvoiceHandler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	invID := chi.URLParam(r, "invId")
	var req struct {
		AmountCents int64 `json:"amountCents"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	inv, err := h.invoiceRepo.GetByID(r.Context(), invID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	newPaid := inv.PaidCents + req.AmountCents
	status := domain.InvoicePartiallyPaid
	if newPaid >= inv.TotalCents {
		status = domain.InvoicePaid
	}
	if err := h.invoiceRepo.UpdatePaidCents(r.Context(), invID, newPaid, status); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"paidCents": newPaid, "status": status})
}

func (h *InvoiceHandler) Summary(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	summary, err := h.invoiceRepo.Summary(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, summary)
}
