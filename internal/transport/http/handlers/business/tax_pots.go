package business

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type TaxPotHandler struct {
	taxPotRepo repository.TaxPotRepository
	potRepo    repository.PotRepository
}

func NewTaxPotHandler(taxPotRepo repository.TaxPotRepository, potRepo repository.PotRepository) *TaxPotHandler {
	return &TaxPotHandler{taxPotRepo: taxPotRepo, potRepo: potRepo}
}

func (h *TaxPotHandler) Create(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	var req struct {
		PotID            string   `json:"potId"`
		TaxType          string   `json:"taxType"`
		AutoSweepPercent *float64 `json:"autoSweepPercent,omitempty"`
		DueDate          *string  `json:"dueDate,omitempty"`
		Notes            *string  `json:"notes,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	tp := &domain.TaxPot{
		BusinessID:       biz.ID,
		PotID:            req.PotID,
		TaxType:          domain.TaxType(req.TaxType),
		AutoSweepPercent: req.AutoSweepPercent,
		DueDate:          req.DueDate,
		Notes:            req.Notes,
	}
	if err := h.taxPotRepo.Create(r.Context(), tp); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, tp)
}

func (h *TaxPotHandler) List(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	pots, err := h.taxPotRepo.ListByBusiness(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	for i := range pots {
		pot, potErr := h.potRepo.GetByID(r.Context(), pots[i].PotID)
		if potErr == nil {
			pots[i].Pot = pot
		}
	}
	httputil.WriteJSON(w, http.StatusOK, pots)
}

func (h *TaxPotHandler) Update(w http.ResponseWriter, r *http.Request) {
	tpID := chi.URLParam(r, "taxPotId")
	var req struct {
		AutoSweepPercent *float64 `json:"autoSweepPercent,omitempty"`
		DueDate          *string  `json:"dueDate,omitempty"`
		Notes            *string  `json:"notes,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	tp := &domain.TaxPot{ID: tpID, AutoSweepPercent: req.AutoSweepPercent, DueDate: req.DueDate, Notes: req.Notes}
	if err := h.taxPotRepo.Update(r.Context(), tp); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, tp)
}

func (h *TaxPotHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tpID := chi.URLParam(r, "taxPotId")
	if err := h.taxPotRepo.Delete(r.Context(), tpID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaxPotHandler) Summary(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	pots, err := h.taxPotRepo.ListByBusiness(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	type potSummary struct {
		TaxType string  `json:"taxType"`
		DueDate *string `json:"dueDate,omitempty"`
		PotID   string  `json:"potId"`
	}
	var items []potSummary
	for _, tp := range pots {
		items = append(items, potSummary{
			TaxType: string(tp.TaxType),
			DueDate: tp.DueDate,
			PotID:   tp.PotID,
		})
	}
	httputil.WriteJSON(w, http.StatusOK, items)
}
