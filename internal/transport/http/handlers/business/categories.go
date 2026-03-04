package business

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type CategoryHandler struct {
	catRepo   repository.TransactionCategoryRepository
	labelRepo repository.TransactionLabelRepository
}

func NewCategoryHandler(catRepo repository.TransactionCategoryRepository, labelRepo repository.TransactionLabelRepository) *CategoryHandler {
	return &CategoryHandler{catRepo: catRepo, labelRepo: labelRepo}
}

func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	cats, err := h.catRepo.ListByBusiness(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if cats == nil {
		cats = []domain.TransactionCategory{}
	}
	httputil.WriteJSON(w, http.StatusOK, cats)
}

func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	var req struct {
		Name  string  `json:"name"`
		Color *string `json:"color,omitempty"`
		Icon  *string `json:"icon,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	cat := &domain.TransactionCategory{
		BusinessID: &biz.ID,
		Name:       req.Name,
		Color:      req.Color,
		Icon:       req.Icon,
	}
	if err := h.catRepo.Create(r.Context(), cat); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, cat)
}

func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	catID := chi.URLParam(r, "catId")
	var req struct {
		Name  string  `json:"name,omitempty"`
		Color *string `json:"color,omitempty"`
		Icon  *string `json:"icon,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	cat := &domain.TransactionCategory{ID: catID, Name: req.Name, Color: req.Color, Icon: req.Icon}
	if err := h.catRepo.Update(r.Context(), cat); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	catID := chi.URLParam(r, "catId")
	if err := h.catRepo.Delete(r.Context(), catID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CategoryHandler) ListLabeled(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	catID := r.URL.Query().Get("categoryId")
	taxStr := r.URL.Query().Get("taxDeductible")
	var catPtr *string
	if catID != "" {
		catPtr = &catID
	}
	var taxPtr *bool
	if taxStr != "" {
		v := taxStr == "true"
		taxPtr = &v
	}
	labels, err := h.labelRepo.ListLabeled(r.Context(), biz.ID, catPtr, taxPtr, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, labels)
}

func (h *CategoryHandler) TaxSummary(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	summary, err := h.labelRepo.TaxSummary(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, summary)
}
