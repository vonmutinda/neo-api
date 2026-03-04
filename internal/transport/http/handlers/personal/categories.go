package personal

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type CategoryHandler struct {
	labelRepo repository.TransactionLabelRepository
}

func NewCategoryHandler(labelRepo repository.TransactionLabelRepository) *CategoryHandler {
	return &CategoryHandler{labelRepo: labelRepo}
}

func (h *CategoryHandler) LabelTransaction(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "txId")
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		CategoryID    *string `json:"categoryId,omitempty"`
		CustomLabel   *string `json:"customLabel,omitempty"`
		Notes         *string `json:"notes,omitempty"`
		TaxDeductible bool    `json:"taxDeductible"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	label := &domain.TransactionLabel{
		TransactionID: txID,
		CategoryID:    req.CategoryID,
		CustomLabel:   req.CustomLabel,
		Notes:         req.Notes,
		TaggedBy:      userID,
		TaxDeductible: req.TaxDeductible,
	}
	if err := h.labelRepo.Create(r.Context(), label); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, label)
}

func (h *CategoryHandler) UpdateLabel(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "txId")
	var req struct {
		CategoryID    *string `json:"categoryId,omitempty"`
		CustomLabel   *string `json:"customLabel,omitempty"`
		Notes         *string `json:"notes,omitempty"`
		TaxDeductible bool    `json:"taxDeductible"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	label := &domain.TransactionLabel{
		TransactionID: txID,
		CategoryID:    req.CategoryID,
		CustomLabel:   req.CustomLabel,
		Notes:         req.Notes,
		TaxDeductible: req.TaxDeductible,
	}
	if err := h.labelRepo.Update(r.Context(), label); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, label)
}

func (h *CategoryHandler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "txId")
	if err := h.labelRepo.Delete(r.Context(), txID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
