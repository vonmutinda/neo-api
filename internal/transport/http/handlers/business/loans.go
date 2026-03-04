package business

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type BusinessLoanHandler struct {
	loanRepo repository.BusinessLoanRepository
}

func NewBusinessLoanHandler(loanRepo repository.BusinessLoanRepository) *BusinessLoanHandler {
	return &BusinessLoanHandler{loanRepo: loanRepo}
}

func (h *BusinessLoanHandler) GetEligibility(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	cp, err := h.loanRepo.GetCreditProfile(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"creditProfile":  cp,
		"eligible":       cp.IsEligibleForLoan(),
		"availableCents": cp.AvailableBorrowingCents(),
	})
}

func (h *BusinessLoanHandler) List(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	loans, err := h.loanRepo.ListByBusiness(r.Context(), biz.ID, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, loans)
}

func (h *BusinessLoanHandler) Get(w http.ResponseWriter, r *http.Request) {
	loanID := chi.URLParam(r, "loanId")
	loan, err := h.loanRepo.GetLoan(r.Context(), loanID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	installments, err := h.loanRepo.ListInstallments(r.Context(), loanID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"loan": loan, "installments": installments})
}

func (h *BusinessLoanHandler) Apply(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		AmountCents           int64  `json:"amountCents"`
		DurationDays          int    `json:"durationDays"`
		Purpose               string `json:"purpose,omitempty"`
		CollateralDescription string `json:"collateralDescription,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	cp, err := h.loanRepo.GetCreditProfile(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if !cp.IsEligibleForLoan() {
		httputil.HandleError(w, r, domain.ErrNoEligibleProfile)
		return
	}
	if req.AmountCents > cp.AvailableBorrowingCents() {
		httputil.HandleError(w, r, domain.ErrLoanLimitExceeded)
		return
	}

	interestRate := 0.08
	interestFee := int64(float64(req.AmountCents) * interestRate)
	dueDate := time.Now().AddDate(0, 0, req.DurationDays)

	loan := &domain.BusinessLoan{
		BusinessID:            biz.ID,
		PrincipalAmountCents:  req.AmountCents,
		InterestFeeCents:      interestFee,
		TotalDueCents:         req.AmountCents + interestFee,
		DurationDays:          req.DurationDays,
		DueDate:               dueDate,
		Purpose:               strPtrOrNil(req.Purpose),
		CollateralDescription: strPtrOrNil(req.CollateralDescription),
		LedgerLoanAccount:     "biz-loan:" + biz.ID,
		AppliedBy:             userID,
	}
	if err := h.loanRepo.CreateLoan(r.Context(), loan); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, loan)
}
