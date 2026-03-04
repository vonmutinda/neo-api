package personal

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/services/lending"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type LoanHandler struct {
	disburse *lending.DisbursementService
	query    *lending.QueryService
}

func NewLoanHandler(disburse *lending.DisbursementService, query *lending.QueryService) *LoanHandler {
	return &LoanHandler{disburse: disburse, query: query}
}

// Apply handles new loan applications.
func (h *LoanHandler) Apply(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req lending.LoanApplyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	loan, err := h.disburse.DisburseLoan(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, loan)
}

// GetEligibility returns the user's borrowing eligibility and credit profile.
func (h *LoanHandler) GetEligibility(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	elig, err := h.query.GetEligibility(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, elig)
}

// ListHistory returns the user's paginated loan history with aggregate stats.
func (h *LoanHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	page, err := h.query.ListHistory(r.Context(), userID, limit, offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, page)
}

// GetLoan returns a single loan with its full installment schedule.
func (h *LoanHandler) GetLoan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	loanID := chi.URLParam(r, "id")
	if loanID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "loan id is required")
		return
	}

	detail, err := h.query.GetLoanDetail(r.Context(), userID, loanID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, detail)
}

// Repay handles manual partial or full loan repayment.
func (h *LoanHandler) Repay(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	loanID := chi.URLParam(r, "id")
	if loanID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "loan id is required")
		return
	}

	var req lending.LoanRepayRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.disburse.ManualRepay(r.Context(), userID, loanID, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "repaid"})
}

// GetCreditScoreHistory returns the user's credit score trend over the last 6 months.
func (h *LoanHandler) GetCreditScoreHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	history, err := h.query.GetCreditScoreHistory(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, history)
}

// GetCreditScore returns the user's credit score breakdown with tips.
func (h *LoanHandler) GetCreditScore(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	breakdown, err := h.query.GetCreditScore(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, breakdown)
}
