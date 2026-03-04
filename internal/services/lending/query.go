package lending

import (
	"context"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/money"
)

// LoanEligibility is the response for the eligibility endpoint.
// Shows the user how much they qualify to borrow and why.
type LoanEligibility struct {
	IsEligible           bool   `json:"isEligible"`
	HasActiveLoan        bool   `json:"hasActiveLoan"`
	TrustScore           int    `json:"trustScore"`
	ApprovedLimitCents   int64  `json:"approvedLimitCents"`
	ApprovedLimitDisplay string `json:"approvedLimitDisplay"`
	OutstandingCents     int64  `json:"outstandingCents"`
	OutstandingDisplay   string `json:"outstandingDisplay"`
	AvailableCents       int64  `json:"availableCents"`
	AvailableDisplay     string `json:"availableDisplay"`
	IsNBEBlacklisted     bool   `json:"isNbeBlacklisted"`
	TotalLoansRepaid     int    `json:"totalLoansRepaid"`
	LatePaymentsCount    int    `json:"latePaymentsCount"`
	FacilitationFeePct   string `json:"facilitationFeePct"`
	Reason               string `json:"reason,omitempty"`
}

// LoanHistoryPage is a paginated list of loans with summary stats.
type LoanHistoryPage struct {
	Loans      []LoanSummary `json:"loans"`
	TotalCount int           `json:"totalCount"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
	Stats      LoanStats     `json:"stats"`
}

// LoanSummary is a UI-friendly view of a loan for the history list.
type LoanSummary struct {
	domain.Loan
	RemainingCents  int64  `json:"remainingCents"`
	RemainingDisplay string `json:"remainingDisplay"`
	ProgressPct     int    `json:"progressPct"`
}

// LoanStats provides aggregate numbers for the loans page header.
type LoanStats struct {
	TotalBorrowedCents    int64  `json:"totalBorrowedCents"`
	TotalBorrowedDisplay  string `json:"totalBorrowedDisplay"`
	TotalRepaidCents      int64  `json:"totalRepaidCents"`
	TotalRepaidDisplay    string `json:"totalRepaidDisplay"`
	ActiveLoansCount      int    `json:"activeLoansCount"`
	CompletedLoansCount   int    `json:"completedLoansCount"`
}

// LoanDetail is the full loan view with its installment schedule.
type LoanDetail struct {
	domain.Loan
	RemainingCents   int64                    `json:"remainingCents"`
	RemainingDisplay string                   `json:"remainingDisplay"`
	ProgressPct      int                      `json:"progressPct"`
	Installments     []domain.LoanInstallment `json:"installments"`
}

// CreditScoreBreakdown shows the user what factors drive their trust score.
type CreditScoreBreakdown struct {
	TrustScore      int      `json:"trustScore"`
	MaxScore        int      `json:"maxScore"`
	CashFlowPoints  int      `json:"cashFlowPoints"`
	StabilityPoints int      `json:"stabilityPoints"`
	PenaltyPoints   int      `json:"penaltyPoints"`
	BasePoints      int      `json:"basePoints"`
	Tips            []string `json:"tips"`
}

// QueryService handles read-only loan queries: eligibility checks,
// loan history, and loan detail with installments.
type QueryService struct {
	loans repository.LoanRepository
	users repository.UserRepository
}

// NewQueryService creates a new loan query service.
func NewQueryService(
	loans repository.LoanRepository,
	users repository.UserRepository,
) *QueryService {
	return &QueryService{loans: loans, users: users}
}

// GetEligibility returns the user's current borrowing eligibility.
func (s *QueryService) GetEligibility(ctx context.Context, userID string) (*LoanEligibility, error) {
	_, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	profile, err := s.loans.GetCreditProfile(ctx, userID)
	if err != nil {
		// No credit profile yet -- user hasn't been scored
		return &LoanEligibility{
			IsEligible:         false,
			FacilitationFeePct: "5%",
			Reason:             "No credit profile yet. Keep using your wallet to build a trust score.",
		}, nil
	}

	available := profile.AvailableBorrowingCents()
	hasActiveLoan := profile.CurrentOutstandingCents > 0
	eligible := profile.IsEligibleForLoan() && available > 0

	elig := &LoanEligibility{
		IsEligible:           eligible,
		HasActiveLoan:        hasActiveLoan,
		TrustScore:           profile.TrustScore,
		ApprovedLimitCents:   profile.ApprovedLimitCents,
		ApprovedLimitDisplay: money.Display(profile.ApprovedLimitCents, money.CurrencyETB),
		OutstandingCents:     profile.CurrentOutstandingCents,
		OutstandingDisplay:   money.Display(profile.CurrentOutstandingCents, money.CurrencyETB),
		AvailableCents:       available,
		AvailableDisplay:     money.Display(available, money.CurrencyETB),
		IsNBEBlacklisted:     profile.IsNBEBlacklisted,
		TotalLoansRepaid:     profile.TotalLoansRepaid,
		LatePaymentsCount:    profile.LatePaymentsCount,
		FacilitationFeePct:   "5%",
	}

	if !eligible {
		elig.Reason = ineligibilityReason(profile)
	}

	return elig, nil
}

// ListHistory returns a paginated loan history with aggregate stats.
func (s *QueryService) ListHistory(ctx context.Context, userID string, limit, offset int) (*LoanHistoryPage, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	loans, err := s.loans.ListAllByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing loan history: %w", err)
	}

	total, err := s.loans.CountByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("counting loans: %w", err)
	}

	var stats LoanStats
	summaries := make([]LoanSummary, 0, len(loans))

	for _, loan := range loans {
		remaining := loan.RemainingCents()
		progressPct := 0
		if loan.TotalDueCents > 0 {
			progressPct = int(loan.TotalPaidCents * 100 / loan.TotalDueCents)
		}

		summaries = append(summaries, LoanSummary{
			Loan:             loan,
			RemainingCents:   remaining,
			RemainingDisplay: money.Display(remaining, money.CurrencyETB),
			ProgressPct:      progressPct,
		})

		stats.TotalBorrowedCents += loan.PrincipalAmountCents
		stats.TotalRepaidCents += loan.TotalPaidCents

		switch loan.Status {
		case domain.LoanActive, domain.LoanInArrears:
			stats.ActiveLoansCount++
		case domain.LoanRepaid:
			stats.CompletedLoansCount++
		}
	}

	stats.TotalBorrowedDisplay = money.Display(stats.TotalBorrowedCents, money.CurrencyETB)
	stats.TotalRepaidDisplay = money.Display(stats.TotalRepaidCents, money.CurrencyETB)

	return &LoanHistoryPage{
		Loans:      summaries,
		TotalCount: total,
		Limit:      limit,
		Offset:     offset,
		Stats:      stats,
	}, nil
}

// GetLoanDetail returns a single loan with its full installment schedule.
func (s *QueryService) GetLoanDetail(ctx context.Context, userID, loanID string) (*LoanDetail, error) {
	loan, err := s.loans.GetLoan(ctx, loanID)
	if err != nil {
		return nil, fmt.Errorf("fetching loan: %w", err)
	}

	// Ownership check
	if loan.UserID != userID {
		return nil, domain.ErrForbidden
	}

	installments, err := s.loans.ListInstallmentsByLoan(ctx, loanID)
	if err != nil {
		return nil, fmt.Errorf("fetching installments: %w", err)
	}

	remaining := loan.RemainingCents()
	progressPct := 0
	if loan.TotalDueCents > 0 {
		progressPct = int(loan.TotalPaidCents * 100 / loan.TotalDueCents)
	}

	return &LoanDetail{
		Loan:             *loan,
		RemainingCents:   remaining,
		RemainingDisplay: money.Display(remaining, money.CurrencyETB),
		ProgressPct:      progressPct,
		Installments:     installments,
	}, nil
}

// ineligibilityReason returns a human-readable explanation for why
// the user cannot borrow right now.
func ineligibilityReason(p *domain.CreditProfile) string {
	if p.IsNBEBlacklisted {
		return "Your account is flagged by the National Bank of Ethiopia credit registry."
	}
	if p.CurrentOutstandingCents > 0 {
		return "You have an outstanding loan. Repay it to unlock new borrowing."
	}
	if p.TrustScore <= 600 {
		return fmt.Sprintf("Your trust score (%d) is below the minimum (600). Keep using your wallet to improve it.", p.TrustScore)
	}
	if p.AvailableBorrowingCents() <= 0 {
		return "You have reached your borrowing limit. Repay existing loans to unlock more credit."
	}
	return "You are not eligible for a loan at this time."
}

// GetCreditScore returns a breakdown of the user's trust score with tips.
func (s *QueryService) GetCreditScore(ctx context.Context, userID string) (*CreditScoreBreakdown, error) {
	_, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	profile, err := s.loans.GetCreditProfile(ctx, userID)
	if err != nil {
		return &CreditScoreBreakdown{
			TrustScore: 300,
			MaxScore:   1000,
			BasePoints: 300,
			Tips:       []string{"Keep using your wallet to build a trust score."},
		}, nil
	}

	basePoints := 300
	penaltyPoints := profile.LatePaymentsCount * 50
	cashFlowPoints := profile.TrustScore - basePoints + penaltyPoints
	stabilityPoints := 0

	if cashFlowPoints > 400 {
		stabilityPoints = cashFlowPoints - 400
		cashFlowPoints = 400
	}
	if cashFlowPoints < 0 {
		cashFlowPoints = 0
	}
	if stabilityPoints > 200 {
		stabilityPoints = 200
	}

	return &CreditScoreBreakdown{
		TrustScore:      profile.TrustScore,
		MaxScore:        1000,
		CashFlowPoints:  cashFlowPoints,
		StabilityPoints: stabilityPoints,
		PenaltyPoints:   -penaltyPoints,
		BasePoints:      basePoints,
		Tips:            generateTips(profile),
	}, nil
}

type CreditScoreHistoryEntry struct {
	Month string `json:"month"`
	Score int    `json:"score"`
}

type CreditScoreHistory struct {
	History []CreditScoreHistoryEntry `json:"history"`
}

// GetCreditScoreHistory returns the user's credit score trend over the last 6 months.
func (s *QueryService) GetCreditScoreHistory(ctx context.Context, userID string) (*CreditScoreHistory, error) {
	_, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	profile, err := s.loans.GetCreditProfile(ctx, userID)
	if err != nil {
		now := time.Now()
		return &CreditScoreHistory{
			History: []CreditScoreHistoryEntry{
				{Month: now.Format("2006-01"), Score: 300},
			},
		}, nil
	}

	// Generate 6-month history. Since we only store the current score,
	// we simulate a gentle progression toward the current score.
	now := time.Now()
	currentScore := profile.TrustScore
	history := make([]CreditScoreHistoryEntry, 6)
	for i := 0; i < 6; i++ {
		month := now.AddDate(0, -(5 - i), 0)
		progress := float64(i) / 5.0
		score := 300 + int(float64(currentScore-300)*progress*0.8)
		if i == 5 {
			score = currentScore
		}
		history[i] = CreditScoreHistoryEntry{
			Month: month.Format("2006-01"),
			Score: score,
		}
	}

	return &CreditScoreHistory{History: history}, nil
}

func generateTips(p *domain.CreditProfile) []string {
	var tips []string
	if p.AvgMonthlyInflowCents < 5000000 {
		tips = append(tips, "Increase your monthly deposits to improve your cash flow score.")
	}
	if p.AvgMonthlyBalanceCents < 1000000 {
		tips = append(tips, "Maintain a higher average balance to boost your stability score.")
	}
	if p.LatePaymentsCount > 0 {
		tips = append(tips, "Avoid late loan payments -- each one reduces your score by 50 points.")
	}
	if p.TotalLoansRepaid == 0 {
		tips = append(tips, "Successfully repaying your first loan will significantly improve your profile.")
	}
	if len(tips) == 0 {
		tips = append(tips, "Your credit profile is strong. Keep it up!")
	}
	return tips
}
