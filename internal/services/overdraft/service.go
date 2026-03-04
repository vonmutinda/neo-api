package overdraft

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/money"
)

const (
	OverdraftLimitPctOfLoan = 10
	OverdraftMaxLimitCents  = 5000000 // 50,000 ETB
	AssetETB                = "ETB"
)

func isETB(asset string) bool {
	return asset == "ETB" || asset == "ETB/2"
}

// Service handles overdraft eligibility, opt-in/opt-out, use-cover, and repayment.
type Service struct {
	overdraftRepo repository.OverdraftRepository
	loanRepo      repository.LoanRepository
	userRepo      repository.UserRepository
	ledgerClient  ledger.Client
	auditRepo     repository.AuditRepository
}

// NewService creates a new overdraft service.
func NewService(
	overdraftRepo repository.OverdraftRepository,
	loanRepo repository.LoanRepository,
	userRepo repository.UserRepository,
	ledgerClient ledger.Client,
	auditRepo repository.AuditRepository,
) *Service {
	return &Service{
		overdraftRepo: overdraftRepo,
		loanRepo:      loanRepo,
		userRepo:      userRepo,
		ledgerClient:  ledgerClient,
		auditRepo:     auditRepo,
	}
}

// StatusResponse is the GET /v1/overdraft response including fee summary.
type StatusResponse struct {
	domain.Overdraft
	FeeSummary string `json:"feeSummary"`
}

// RepayRequest is the body for POST /v1/overdraft/repay.
type RepayRequest struct {
	AmountCents int64 `json:"amountCents" validate:"required,gt=0"`
}

// Eligibility computes overdraft limit from credit profile: 10% of approved loan limit, capped at 50,000 ETB.
// Returns 0 if not eligible (no profile, NBE blacklisted, or trust score too low).
func (s *Service) Eligibility(ctx context.Context, userID string) (limitCents int64, eligible bool, err error) {
	profile, err := s.loanRepo.GetCreditProfile(ctx, userID)
	if err != nil {
		return 0, false, nil
	}
	if profile.IsNBEBlacklisted || profile.TrustScore <= 600 || profile.ApprovedLimitCents <= 0 {
		return 0, false, nil
	}
	limitCents = (profile.ApprovedLimitCents * OverdraftLimitPctOfLoan) / 100
	if limitCents > OverdraftMaxLimitCents {
		limitCents = OverdraftMaxLimitCents
	}
	return limitCents, limitCents > 0, nil
}

// GetStatus returns the user's overdraft status and fee summary for API/UI.
func (s *Service) GetStatus(ctx context.Context, userID string) (*StatusResponse, error) {
	o, err := s.overdraftRepo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := &StatusResponse{}
	if o != nil {
		resp.Overdraft = *o
		resp.FeeSummary = buildFeeSummary(o)
	} else {
		limit, _, _ := s.Eligibility(ctx, userID)
		resp.UserID = userID
		resp.Status = domain.OverdraftInactive
		resp.LimitCents = limit
		resp.AvailableCents = limit
		resp.DailyFeeBasisPoints = 15
		resp.InterestFreeDays = 7
		resp.FeeSummary = buildFeeSummary(&resp.Overdraft)
	}
	return resp, nil
}

func buildFeeSummary(o *domain.Overdraft) string {
	if o == nil {
		return "No fee for the first 7 days. After that, 0.15% per day on the amount you use. Example: 1,000 ETB for 10 days (3 days after free period) ≈ 4.50 ETB."
	}
	pct := float64(o.DailyFeeBasisPoints) / 100.0
	return fmt.Sprintf("No fee for the first %d days. After that, %.2f%% per day on the amount you use. Example: 1,000 ETB for 10 days (3 days after free period) ≈ 4.50 ETB.",
		o.InterestFreeDays, pct)
}

// OptIn enables overdraft for the user (creates or updates row to active, sets limit from eligibility).
func (s *Service) OptIn(ctx context.Context, userID string) (*domain.Overdraft, error) {
	limitCents, eligible, err := s.Eligibility(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, domain.ErrOverdraftNotEligible
	}

	o, err := s.overdraftRepo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if o != nil && o.Status == domain.OverdraftActive {
		return nil, domain.ErrOverdraftAlreadyActive
	}
	if o != nil && o.Status == domain.OverdraftUsed {
		return nil, domain.ErrOverdraftInUse
	}

	now := time.Now().UTC()
	if o == nil {
		o = &domain.Overdraft{
			UserID:              userID,
			LimitCents:          limitCents,
			DailyFeeBasisPoints: 15,
			InterestFreeDays:    7,
			Status:              domain.OverdraftActive,
			OptedInAt:           &now,
		}
	} else {
		o.LimitCents = limitCents
		o.Status = domain.OverdraftActive
		o.OptedInAt = &now
	}

	if err := s.overdraftRepo.CreateOrUpdate(ctx, o); err != nil {
		return nil, err
	}

	meta, _ := json.Marshal(map[string]int64{"limit_cents": limitCents})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditOverdraftOptedIn,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "overdraft",
		ResourceID:   o.ID,
		Metadata:     meta,
	})
	return o, nil
}

// OptOut disables overdraft only when used_cents == 0.
func (s *Service) OptOut(ctx context.Context, userID string) error {
	o, err := s.overdraftRepo.GetByUser(ctx, userID)
	if err != nil {
		return err
	}
	if o == nil {
		return domain.ErrOverdraftNotActive
	}
	if o.UsedCents > 0 || o.AccruedFeeCents > 0 {
		return domain.ErrOverdraftInUse
	}
	if o.Status == domain.OverdraftInactive {
		return domain.ErrOverdraftNotActive
	}

	o.Status = domain.OverdraftInactive
	o.OptedInAt = nil
	if err := s.overdraftRepo.CreateOrUpdate(ctx, o); err != nil {
		return err
	}

	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditOverdraftOptedOut,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "overdraft",
		ResourceID:   o.ID,
	})
	return nil
}

// UseCover is called by the payment service when ETB balance is insufficient.
// It credits the wallet from overdraft capital and updates used_cents. ETB only.
func (s *Service) UseCover(ctx context.Context, userID, walletID, idempotencyKey string, shortfallCents int64, asset string) error {
	if !isETB(asset) {
		return domain.ErrOverdraftETBOnly
	}
	o, err := s.overdraftRepo.GetByUser(ctx, userID)
	if err != nil {
		return err
	}
	if o == nil || (o.Status != domain.OverdraftActive && o.Status != domain.OverdraftUsed) {
		return domain.ErrOverdraftNotActive
	}
	available := o.LimitCents - o.UsedCents
	if available < shortfallCents {
		return domain.ErrOverdraftLimitExceeded
	}

	_, err = s.ledgerClient.CreditFromOverdraft(ctx, idempotencyKey, walletID, shortfallCents, asset)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	newStatus := domain.OverdraftUsed
	var overdrawnSince *time.Time
	if o.UsedCents == 0 {
		overdrawnSince = &now
	}
	if err := s.overdraftRepo.UpdateUsedAndStatus(ctx, userID, shortfallCents, newStatus, overdrawnSince); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]any{"shortfall_cents": shortfallCents})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditOverdraftUsed,
		ActorType:    "system",
		ResourceType: "overdraft",
		ResourceID:   o.ID,
		Metadata:     meta,
	})
	return nil
}

// Repay applies a manual repayment: debit wallet to overdraft capital, reduce used_cents and accrued_fee_cents.
func (s *Service) Repay(ctx context.Context, userID, idempotencyKey string, amountCents int64) error {
	if amountCents <= 0 {
		return domain.ErrZeroAmount
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	o, err := s.overdraftRepo.GetByUser(ctx, userID)
	if err != nil {
		return err
	}
	if o == nil || o.Status != domain.OverdraftUsed {
		return domain.ErrOverdraftNotActive
	}
	totalOwed := o.UsedCents + o.AccruedFeeCents
	if amountCents > totalOwed {
		amountCents = totalOwed
	}

	walletID := user.LedgerWalletID
	assetETB := money.FormatAsset(money.CurrencyETB)
	if err := s.ledgerClient.DebitToOverdraft(ctx, idempotencyKey, walletID, amountCents, assetETB); err != nil {
		return err
	}

	// Apply to fee first, then principal
	remaining := amountCents
	feeRepay := remaining
	if feeRepay > o.AccruedFeeCents {
		feeRepay = o.AccruedFeeCents
	}
	remaining -= feeRepay
	principalRepay := remaining
	if principalRepay > o.UsedCents {
		principalRepay = o.UsedCents
	}
	newUsed := o.UsedCents - principalRepay
	newAccrued := o.AccruedFeeCents - feeRepay
	if newAccrued < 0 {
		newAccrued = 0
	}
	newStatus := domain.OverdraftActive
	if newUsed > 0 || newAccrued > 0 {
		newStatus = domain.OverdraftUsed
	}
	if err := s.overdraftRepo.UpdateRepaid(ctx, userID, newUsed, newAccrued, newStatus); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]any{"amount_cents": amountCents})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditOverdraftRepaid,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "overdraft",
		ResourceID:   o.ID,
		Metadata:     meta,
	})
	return nil
}

// AutoRepayOnInflow is called after an ETB credit to the user (e.g. P2P receive, loan disbursement).
// Repays min(creditAmountCents, used_cents + accrued_fee_cents) from wallet to overdraft capital.
// Returns the amount repaid in cents, or 0 if none; callers use this to set receipt metadata.
func (s *Service) AutoRepayOnInflow(ctx context.Context, userID, walletID, idempotencyKey string, creditAmountCents int64) (repayCents int64, err error) {
	o, err := s.overdraftRepo.GetByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if o == nil || o.Status != domain.OverdraftUsed {
		return 0, nil
	}
	totalOwed := o.UsedCents + o.AccruedFeeCents
	if totalOwed <= 0 {
		return 0, nil
	}
	repayCents = creditAmountCents
	if repayCents > totalOwed {
		repayCents = totalOwed
	}
	if repayCents <= 0 {
		return 0, nil
	}

	assetETB := money.FormatAsset(money.CurrencyETB)
	if err := s.ledgerClient.DebitToOverdraft(ctx, idempotencyKey, walletID, repayCents, assetETB); err != nil {
		return 0, err
	}
	feeRepay := repayCents
	if feeRepay > o.AccruedFeeCents {
		feeRepay = o.AccruedFeeCents
	}
	principalRepay := repayCents - feeRepay
	if principalRepay > o.UsedCents {
		principalRepay = o.UsedCents
	}
	newUsed := o.UsedCents - principalRepay
	newAccrued := o.AccruedFeeCents - feeRepay
	if newAccrued < 0 {
		newAccrued = 0
	}
	newStatus := domain.OverdraftActive
	if newUsed > 0 || newAccrued > 0 {
		newStatus = domain.OverdraftUsed
	}
	if err := s.overdraftRepo.UpdateRepaid(ctx, userID, newUsed, newAccrued, newStatus); err != nil {
		return 0, err
	}
	meta, _ := json.Marshal(map[string]any{"amount_cents": repayCents})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditOverdraftRepaid,
		ActorType:    "system",
		ResourceType: "overdraft",
		ResourceID:   o.ID,
		Metadata:     meta,
	})
	return repayCents, nil
}

// FeeSummaryForUI returns a human-readable fee description (used when no overdraft row yet).
func FeeSummaryForUI() string {
	return fmt.Sprintf("No fee for the first 7 days. After that, 0.15%% per day on the amount you use. Example: %s for 10 days (3 days after free period) ≈ 4.50 ETB.",
		money.Display(100000, money.CurrencyETB))
}
