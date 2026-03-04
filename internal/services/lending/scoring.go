package lending

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/gateway/nbe"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/money"
)

// ScoringService recalculates trust scores for all users.
// Run by the lending-worker cron job weekly.
type ScoringService struct {
	loans     repository.LoanRepository
	users     repository.UserRepository
	audit     repository.AuditRepository
	ledger    ledger.Client
	nbeClient nbe.Client
	chart     *ledger.Chart
}

func NewScoringService(
	loans repository.LoanRepository,
	users repository.UserRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
	nbeClient nbe.Client,
	chart *ledger.Chart,
) *ScoringService {
	return &ScoringService{
		loans:     loans,
		users:     users,
		audit:     audit,
		ledger:    ledgerClient,
		nbeClient: nbeClient,
		chart:     chart,
	}
}

// CalculateTrustScore computes the trust score for a single user based on
// their 90-day Formance cash-flow history.
//
// Scoring pillars:
//   - Base: 300 (floor)
//   - Pillar 1: Cash Flow Velocity (max +400) -- monthly inflow volume
//   - Pillar 2: Account Stability  (max +200) -- average balance retention
//   - Pillar 3: Penalties          (-50 per late payment)
func (s *ScoringService) CalculateTrustScore(ctx context.Context, userID string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("looking up user for scoring: %w", err)
	}

	asset := money.FormatAsset()
	walletAccount := s.chart.MainAccount(user.LedgerWalletID)

	history, err := s.ledger.GetAccountHistory(ctx, walletAccount, 500)
	if err != nil {
		return fmt.Errorf("fetching account history: %w", err)
	}

	score := 300.0
	var totalInflow int64
	for _, tx := range history {
		if tx.IsCredit && tx.Asset == asset {
			totalInflow += tx.AmountCents
		}
	}

	// Pillar 1: Cash Flow Velocity (max +400 points)
	monthlyInflowETB := (float64(totalInflow) / 3.0) / float64(money.CentsFactor)
	cashFlowPoints := math.Min(400, (monthlyInflowETB/1000.0)*10.0)
	score += cashFlowPoints

	// Pillar 2: Account Stability (max +200 points)
	balance, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, asset)
	if err != nil {
		return fmt.Errorf("fetching wallet balance: %w", err)
	}
	balanceETB := float64(balance.Int64()) / float64(money.CentsFactor)
	stabilityPoints := math.Min(200, (balanceETB/500.0)*20.0)
	score += stabilityPoints

	// Pillar 3: Penalties
	latePayments, err := s.loans.CountLatePayments(ctx, userID)
	if err != nil {
		return fmt.Errorf("counting late payments: %w", err)
	}
	score -= float64(latePayments * 50)

	finalScore := int(math.Max(300, math.Min(1000, score)))

	// Calculate approved limit: 20% of average monthly inflow if score > 600
	var newLimitCents int64
	if finalScore > 600 {
		newLimitCents = int64(float64(totalInflow/3) * 0.20)
	}

	// Check NBE blacklist
	isBlacklisted := false
	if user.FaydaIDNumber != nil {
		isBlacklisted, _ = s.nbeClient.IsBlacklisted(ctx, *user.FaydaIDNumber)
	}

	now := time.Now()
	profile := &domain.CreditProfile{
		UserID:                 userID,
		TrustScore:             finalScore,
		ApprovedLimitCents:     newLimitCents,
		AvgMonthlyInflowCents:  totalInflow / 3,
		AvgMonthlyBalanceCents: balance.Int64(),
		ActiveDaysPerMonth:     0,
		LatePaymentsCount:      latePayments,
		IsNBEBlacklisted:       isBlacklisted,
		BlacklistCheckedAt:     &now,
		LastCalculatedAt:       now,
	}

	if err := s.loans.UpsertCreditProfile(ctx, profile); err != nil {
		return fmt.Errorf("upserting credit profile: %w", err)
	}

	userIDCopy := userID
	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditCreditScoreUpdated,
		ActorType:    "cron",
		ActorID:      &userIDCopy,
		ResourceType: "credit_profile",
		ResourceID:   userID,
	})

	return nil
}
