package business

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type BatchPaymentHandler struct {
	batchRepo repository.BatchPaymentRepository
	audit     repository.AuditRepository
}

func NewBatchPaymentHandler(batchRepo repository.BatchPaymentRepository, audit repository.AuditRepository) *BatchPaymentHandler {
	return &BatchPaymentHandler{batchRepo: batchRepo, audit: audit}
}

func (h *BatchPaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Name         string `json:"name"`
		CurrencyCode string `json:"currencyCode"`
		Items        []struct {
			RecipientName    string  `json:"recipientName"`
			RecipientPhone   *phone.PhoneNumber `json:"recipientPhone,omitempty"`
			RecipientBank    *string `json:"recipientBank,omitempty"`
			RecipientAccount *string `json:"recipientAccount,omitempty"`
			AmountCents      int64   `json:"amountCents"`
			Narration        *string `json:"narration,omitempty"`
			CategoryID       *string `json:"categoryId,omitempty"`
		} `json:"items"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	var totalCents int64
	for _, item := range req.Items {
		totalCents += item.AmountCents
	}

	currency := req.CurrencyCode
	if currency == "" {
		currency = "ETB"
	}

	batch := &domain.BatchPayment{
		BusinessID:   biz.ID,
		Name:         req.Name,
		TotalCents:   totalCents,
		CurrencyCode: currency,
		ItemCount:    len(req.Items),
		Status:       domain.BatchDraft,
		InitiatedBy:  userID,
	}
	if err := h.batchRepo.CreateBatch(r.Context(), batch); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	var items []domain.BatchPaymentItem
	for _, item := range req.Items {
		items = append(items, domain.BatchPaymentItem{
			BatchID:          batch.ID,
			RecipientName:    item.RecipientName,
			RecipientPhone:   item.RecipientPhone,
			RecipientBank:    item.RecipientBank,
			RecipientAccount: item.RecipientAccount,
			AmountCents:      item.AmountCents,
			Narration:        item.Narration,
			CategoryID:       item.CategoryID,
		})
	}
	if err := h.batchRepo.CreateItems(r.Context(), items); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	meta, _ := json.Marshal(map[string]string{"business_id": biz.ID, "batch_id": batch.ID})
	_ = h.audit.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditBusinessBatchCreated,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "batch_payment",
		ResourceID:   batch.ID,
		Metadata:     meta,
	})

	httputil.WriteJSON(w, http.StatusCreated, batch)
}

func (h *BatchPaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	batches, err := h.batchRepo.ListByBusiness(r.Context(), biz.ID, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, batches)
}

func (h *BatchPaymentHandler) Get(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batchId")
	batch, err := h.batchRepo.GetBatch(r.Context(), batchID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	items, err := h.batchRepo.ListItems(r.Context(), batchID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"batch": batch, "items": items})
}

func (h *BatchPaymentHandler) Approve(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batchId")
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.batchRepo.ApproveBatch(r.Context(), batchID, userID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *BatchPaymentHandler) Process(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batchId")
	batch, err := h.batchRepo.GetBatch(r.Context(), batchID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if batch.Status != domain.BatchApproved {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "batch must be approved before processing")
		return
	}
	if err := h.batchRepo.UpdateBatchStatus(r.Context(), batchID, domain.BatchProcessing); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "processing"})
}
