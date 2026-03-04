package personal

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/services/beneficiary"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type BeneficiaryHandler struct {
	svc *beneficiary.Service
}

func NewBeneficiaryHandler(svc *beneficiary.Service) *BeneficiaryHandler {
	return &BeneficiaryHandler{svc: svc}
}

func (h *BeneficiaryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req beneficiary.CreateBeneficiaryRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	b, err := h.svc.Create(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, b)
}

func (h *BeneficiaryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	list, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if list == nil {
		list = []domain.Beneficiary{}
	}

	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *BeneficiaryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
