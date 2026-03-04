package regulatory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/vonmutinda/neo/pkg/money"
)

// RateFunc is a function that returns the mid-market rate between two currencies.
// Used to avoid import cycles with the convert package.
type RateFunc func(ctx context.Context, from, to string) (float64, error)

// TransferCheckRequest contains all context needed to evaluate regulatory rules
// against a proposed transfer.
type TransferCheckRequest struct {
	UserID      string
	User        *domain.User
	Direction   string // "outbound", "inbound", "p2p", "card_international"
	AmountCents int64
	Currency    string
	Purpose     string // "general", "medical", "education", "travel", "family_support", "investment", "trade"
	Destination string // Country code or "domestic"
}

// Service evaluates regulatory rules from the database with in-memory caching.
// Rule precedence: currency-specific > account_type > kyc_level > global.
type Service struct {
	repo    repository.RegulatoryRuleRepository
	totals  repository.TransferTotalsRepository
	ratesFn RateFunc
	cache   cache.Cache
	ttl     time.Duration
}

func NewService(
	repo repository.RegulatoryRuleRepository,
	totals repository.TransferTotalsRepository,
	ratesFn RateFunc,
	cacheTTL time.Duration,
	c cache.Cache,
) *Service {
	return &Service{
		repo:    repo,
		totals:  totals,
		ratesFn: ratesFn,
		cache:   c,
		ttl:     cacheTTL,
	}
}

// GetAmountLimit resolves the effective amount_cents rule for a given key + user context.
// Precedence: currency-specific > account_type > kyc_level > global.
func (s *Service) GetAmountLimit(ctx context.Context, key string, user *domain.User, currency string) (int64, error) {
	rules, err := s.getEffectiveRules(ctx, key)
	if err != nil {
		return 0, err
	}

	var best *domain.RegulatoryRule
	bestPriority := -1

	for i := range rules {
		r := &rules[i]
		priority := s.matchPriority(r, user, currency)
		if priority > bestPriority {
			best = r
			bestPriority = priority
		}
	}

	if best == nil {
		return 0, fmt.Errorf("no rule found for %s: %w", key, domain.ErrRegulatoryRuleNotFound)
	}

	cents, err := strconv.ParseInt(best.Value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing rule value %q as amount_cents: %w", best.Value, err)
	}
	return cents, nil
}

// IsEnabled resolves a boolean rule.
func (s *Service) IsEnabled(ctx context.Context, key string, user *domain.User) (bool, error) {
	rules, err := s.getEffectiveRules(ctx, key)
	if err != nil {
		return false, err
	}

	var best *domain.RegulatoryRule
	bestPriority := -1

	for i := range rules {
		r := &rules[i]
		priority := s.matchPriority(r, user, "")
		if priority > bestPriority {
			best = r
			bestPriority = priority
		}
	}

	if best == nil {
		return false, fmt.Errorf("no rule found for %s: %w", key, domain.ErrRegulatoryRuleNotFound)
	}

	return best.Value == "true", nil
}

// GetPercentValue resolves a percent rule.
func (s *Service) GetPercentValue(ctx context.Context, key string, user *domain.User) (float64, error) {
	rules, err := s.getEffectiveRules(ctx, key)
	if err != nil {
		return 0, err
	}

	var best *domain.RegulatoryRule
	bestPriority := -1

	for i := range rules {
		r := &rules[i]
		priority := s.matchPriority(r, user, "")
		if priority > bestPriority {
			best = r
			bestPriority = priority
		}
	}

	if best == nil {
		return 0, fmt.Errorf("no rule found for %s: %w", key, domain.ErrRegulatoryRuleNotFound)
	}

	pct, err := strconv.ParseFloat(best.Value, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing rule value %q as percent: %w", best.Value, err)
	}
	return pct, nil
}

// GetDurationValue resolves a duration rule.
func (s *Service) GetDurationValue(ctx context.Context, key string) (time.Duration, error) {
	rules, err := s.getEffectiveRules(ctx, key)
	if err != nil {
		return 0, err
	}

	for i := range rules {
		if rules[i].Scope == domain.RuleScopeGlobal {
			d, err := time.ParseDuration(rules[i].Value)
			if err != nil {
				return 0, fmt.Errorf("parsing rule value %q as duration: %w", rules[i].Value, err)
			}
			return d, nil
		}
	}
	return 0, fmt.Errorf("no rule found for %s: %w", key, domain.ErrRegulatoryRuleNotFound)
}

// CheckTransferAllowed evaluates all relevant rules for a transfer.
// Returns nil if allowed, or a structured error describing which rule blocked it.
func (s *Service) CheckTransferAllowed(ctx context.Context, req *TransferCheckRequest) error {
	user := req.User

	// 1. KYC daily limit
	dailyLimit, err := s.GetAmountLimit(ctx, "daily_transfer_limit", user, req.Currency)
	if err == nil && dailyLimit > 0 {
		dailyTotal, _ := s.totals.GetDailyTotal(ctx, req.UserID, req.Currency, req.Direction)
		if dailyTotal+req.AmountCents > dailyLimit {
			return fmt.Errorf("daily %s limit of %s exceeded: %w",
				req.Currency, money.Display(dailyLimit, req.Currency), domain.ErrDailyLimitExceeded)
		}
	}

	// 2. Outbound remittance cap (Clause 8)
	if req.Direction == "outbound" || req.Direction == "card_international" {
		monthlyCapCents, err := s.GetAmountLimit(ctx, "outbound_remittance_monthly_cap", user, req.Currency)
		if err == nil && monthlyCapCents > 0 {
			monthlyTotal, _ := s.totals.GetMonthlyTotal(ctx, req.UserID, "USD", req.Direction)
			amountInUSD, err := s.convertToUSD(ctx, req.Currency, req.AmountCents)
			if err != nil {
				return fmt.Errorf("converting to USD for remittance cap check: %w", err)
			}
			if monthlyTotal+amountInUSD > monthlyCapCents {
				return fmt.Errorf("monthly outbound remittance cap of USD %s exceeded: %w",
					money.FormatMinorUnits(monthlyCapCents), domain.ErrRemittanceCapExceeded)
			}
		}
	}

	// 3. Purpose-specific limits (Clause 15)
	if req.Purpose == string(domain.PurposeMedical) || req.Purpose == string(domain.PurposeEducation) {
		advanceCap, err := s.GetAmountLimit(ctx, "advance_payment_medical_education_cap", user, "USD")
		if err == nil && advanceCap > 0 {
			amountInUSD, err := s.convertToUSD(ctx, req.Currency, req.AmountCents)
			if err != nil {
				return fmt.Errorf("converting to USD for advance cap check: %w", err)
			}
			if amountInUSD > advanceCap {
				return fmt.Errorf("advance payment cap of USD %s exceeded for %s: %w",
					money.FormatMinorUnits(advanceCap), req.Purpose, domain.ErrTransferBlocked)
			}
		}
	}

	// 4. Currency restrictions: ETB transfers domestic only
	if req.Currency == money.CurrencyETB && req.Destination != "" && req.Destination != "domestic" && req.Direction == "outbound" {
		return fmt.Errorf("ETB transfers are domestic only: %w", domain.ErrTransferBlocked)
	}

	// 5. KYC level minimum for outbound international
	if req.Direction == "outbound" && req.Destination != "domestic" && req.Destination != "" {
		if user.KYCLevel < domain.KYCVerified {
			return fmt.Errorf("outbound international requires KYC Verified: %w", domain.ErrKYCInsufficientForFX)
		}
	}

	// 6. Investment check
	if req.Purpose == string(domain.PurposeInvestment) {
		enabled, _ := s.IsEnabled(ctx, "outbound_investment_enabled", user)
		if !enabled {
			return domain.ErrInvestmentNotEnabled
		}
	}

	// 7. Document threshold for outbound remittance
	if req.Direction == "outbound" {
		docThreshold, err := s.GetAmountLimit(ctx, "outbound_remittance_doc_threshold", user, "USD")
		if err == nil && docThreshold > 0 {
			amountInUSD, err := s.convertToUSD(ctx, req.Currency, req.AmountCents)
			if err != nil {
				return fmt.Errorf("converting to USD for doc threshold check: %w", err)
			}
			if amountInUSD > docThreshold && req.Purpose == string(domain.PurposeGeneral) {
				return fmt.Errorf("transfers above USD %s require supporting documents: %w",
					money.FormatMinorUnits(docThreshold), domain.ErrDocumentRequired)
			}
		}
	}

	return nil
}

// convertToUSD converts an amount from the given currency to USD using the
// injected RateProvider. Uses mid-market rate for regulatory cap checks.
func (s *Service) convertToUSD(ctx context.Context, currency string, amountCents int64) (int64, error) {
	if currency == "USD" {
		return amountCents, nil
	}
	mid, err := s.ratesFn(ctx, currency, "USD")
	if err != nil {
		return 0, fmt.Errorf("getting FX rate for %s/USD: %w", currency, err)
	}
	return int64(math.Round(float64(amountCents) * mid)), nil
}

// InvalidateCache clears all cached rules. Called when rules are updated via admin API.
func (s *Service) InvalidateCache() {
	_ = s.cache.DeleteByPrefix(context.Background(), "neo:rules:")
}

// getEffectiveRules fetches all effective rules for a key, using the cache.
func (s *Service) getEffectiveRules(ctx context.Context, key string) ([]domain.RegulatoryRule, error) {
	cacheKey := "neo:rules:" + key
	if data, ok := s.cache.Get(ctx, cacheKey); ok {
		var rules []domain.RegulatoryRule
		if json.Unmarshal(data, &rules) == nil {
			return rules, nil
		}
	}

	rules, err := s.repo.ListEffectiveByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(rules); err == nil {
		_ = s.cache.Set(ctx, cacheKey, data, s.ttl)
	}
	return rules, nil
}

// matchPriority returns a priority score for how well a rule matches the user context.
// Higher is more specific. -1 means no match.
func (s *Service) matchPriority(r *domain.RegulatoryRule, user *domain.User, currency string) int {
	switch r.Scope {
	case domain.RuleScopeCurrency:
		if currency != "" && r.ScopeValue == currency {
			return 4
		}
		return -1
	case domain.RuleScopeAccountType:
		if string(user.AccountType) == r.ScopeValue {
			return 3
		}
		// "fx_holder" matches any user who has activated a non-ETB currency.
		// The caller should set AccountType accordingly, but we also match
		// the special scope value for flexibility.
		if r.ScopeValue == "fx_holder" {
			return 2
		}
		return -1
	case domain.RuleScopeKYCLevel:
		kycStr := strconv.Itoa(int(user.KYCLevel))
		if kycStr == r.ScopeValue {
			return 2
		}
		return -1
	case domain.RuleScopeGlobal:
		return 1
	default:
		return -1
	}
}
