package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type LoanHandler struct {
	svc *adminsvc.LoanService
}

func NewLoanHandler(svc *adminsvc.LoanService) *LoanHandler {
	return &LoanHandler{svc: svc}
}

func (h *LoanHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := parseLoanFilter(r.URL.Query())
	result, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *LoanHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	loan, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, loan)
}

func (h *LoanHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.Summary(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, summary)
}

func (h *LoanHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	loanID := chi.URLParam(r, "id")
	var req adminsvc.WriteOffRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.WriteOff(r.Context(), staffID, loanID, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "written_off"})
}

func (h *LoanHandler) ListCreditProfiles(w http.ResponseWriter, r *http.Request) {
	filter := parseCreditProfileFilter(r.URL.Query())
	result, err := h.svc.ListCreditProfiles(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *LoanHandler) GetCreditProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	profile, err := h.svc.GetCreditProfile(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, profile)
}

func (h *LoanHandler) OverrideCredit(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	userID := chi.URLParam(r, "userId")
	var req adminsvc.CreditOverrideRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.OverrideCredit(r.Context(), staffID, userID, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "credit_overridden"})
}
