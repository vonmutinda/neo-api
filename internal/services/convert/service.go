package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/validate"
	"github.com/google/uuid"
)

// OverdraftRepayOnInflow is the subset of overdraft needed to repay on ETB inflow (e.g. after FX convert to ETB).
type OverdraftRepayOnInflow interface {
	AutoRepayOnInflow(ctx context.Context, userID, walletID, idempotencyKey string, creditAmountCents int64) (repayCents int64, err error)
}

type Service struct {
	users      repository.UserRepository
	ledger     ledger.Client
	chart      *ledger.Chart
	rates      RateProvider
	regulatory *regulatory.Service
	receipts   repository.TransactionReceiptRepository
	overdraft  OverdraftRepayOnInflow
}

func NewService(
	users repository.UserRepository,
	ledgerClient ledger.Client,
	chart *ledger.Chart,
	rates RateProvider,
	regulatorySvc *regulatory.Service,
	receipts repository.TransactionReceiptRepository,
	overdraft OverdraftRepayOnInflow,
) *Service {
	return &Service{
		users:      users,
		ledger:     ledgerClient,
		chart:      chart,
		rates:      rates,
		regulatory: regulatorySvc,
		receipts:   receipts,
		overdraft:  overdraft,
	}
}

func (s *Service) Convert(ctx context.Context, userID string, req *domain.ConvertRequest) (*domain.ConvertResponse, error) {
	if err := validate.CurrencyCode(req.FromCurrency); err != nil {
		return nil, err
	}
	if err := validate.CurrencyCode(req.ToCurrency); err != nil {
		return nil, err
	}
	if err := money.ValidateAmountCents(req.AmountCents); err != nil {
		return nil, err
	}
	if req.FromCurrency == req.ToCurrency {
		return nil, fmt.Errorf("cannot convert %s to itself: %w", req.FromCurrency, domain.ErrInvalidInput)
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	// Check if FX conversion is enabled
	if s.regulatory != nil {
		enabled, err := s.regulatory.IsEnabled(ctx, "fx_conversion_enabled", user)
		if err == nil && !enabled {
			return nil, domain.ErrFXConversionDisabled
		}
	}

	// Get rate from provider
	rate, err := s.rates.GetRate(ctx, req.FromCurrency, req.ToCurrency)
	if err != nil {
		return nil, fmt.Errorf("getting exchange rate: %w", err)
	}

	// Check rate staleness
	if s.regulatory != nil {
		// threshold, err := s.regulatory.GetDurationValue(ctx, "fx_rate_staleness_threshold")
		// if err == nil && threshold > 0 {
		// 	if time.Since(rate.Timestamp) > threshold {
		// 		return nil, domain.ErrRateStaleness
		// 	}
		// }
	}

	toCents, effectiveRate := ConvertWithSpread(rate, req.AmountCents)

	fromAsset := money.FormatAsset(req.FromCurrency)
	toAsset := money.FormatAsset(req.ToCurrency)

	balance, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, fromAsset)
	if err != nil {
		return nil, fmt.Errorf("checking balance: %w", err)
	}
	if balance.Cmp(new(big.Int).SetInt64(req.AmountCents)) < 0 {
		return nil, domain.ErrInsufficientFunds
	}

	ik := uuid.NewString()
	txID, err := s.ledger.ConvertCurrency(ctx, ik, user.LedgerWalletID, req.AmountCents, fromAsset, toCents, toAsset, s.chart.SystemFX())
	if err != nil {
		return nil, fmt.Errorf("converting currency: %w", err)
	}

	narration := fmt.Sprintf("Converted %s %s to %s %s",
		money.FormatMinorUnits(req.AmountCents), req.FromCurrency,
		money.FormatMinorUnits(toCents), req.ToCurrency,
	)

	convertOutMeta := domain.ConvertMetadata{FromCurrency: req.FromCurrency, ToCurrency: req.ToCurrency}
	outMetaBytes, _ := json.Marshal(convertOutMeta)
	outMetaRaw := json.RawMessage(outMetaBytes)
	_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: txID,
		IdempotencyKey:      &ik,
		Type:                domain.ReceiptConvertOut,
		Status:              domain.ReceiptCompleted,
		AmountCents:         req.AmountCents,
		Currency:            req.FromCurrency,
		Narration:           &narration,
		Metadata:            &outMetaRaw,
	})

	convertInMeta := domain.ConvertMetadata{FromCurrency: req.FromCurrency, ToCurrency: req.ToCurrency}
	if req.ToCurrency == money.CurrencyETB && s.overdraft != nil {
		repayCents, _ := s.overdraft.AutoRepayOnInflow(ctx, userID, user.LedgerWalletID, ik+"-od-repay", toCents)
		if repayCents > 0 {
			convertInMeta.TotalInflowCents = toCents
			convertInMeta.OverdraftRepaymentCents = repayCents
			convertInMeta.NetInflowCents = toCents - repayCents
		}
	}
	inMetaBytes, _ := json.Marshal(convertInMeta)
	inMetaRaw := json.RawMessage(inMetaBytes)
	_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: txID,
		IdempotencyKey:      &ik,
		Type:                domain.ReceiptConvertIn,
		Status:              domain.ReceiptCompleted,
		AmountCents:         toCents,
		Currency:            req.ToCurrency,
		Narration:           &narration,
		Metadata:            &inMetaRaw,
	})

	resp := &domain.ConvertResponse{
		FromCurrency:    req.FromCurrency,
		ToCurrency:      req.ToCurrency,
		FromAmountCents: req.AmountCents,
		ToAmountCents:   toCents,
		Rate:            rate.Mid,
		TransactionID:   txID,
	}

	if rate.Spread > 0 {
		resp.Spread = &rate.Spread
		resp.EffectiveRate = &effectiveRate
	}

	return resp, nil
}

// GetRate returns the full rate quote between two currencies.
func (s *Service) GetRate(ctx context.Context, fromCode, toCode string) (*Rate, error) {
	if err := validate.CurrencyCode(fromCode); err != nil {
		return nil, err
	}
	if err := validate.CurrencyCode(toCode); err != nil {
		return nil, err
	}

	rate, err := s.rates.GetRate(ctx, fromCode, toCode)
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

// GetRateProvider exposes the underlying RateProvider for use by other services
// (e.g., card auth, wallet summary).
func (s *Service) GetRateProvider() RateProvider {
	return s.rates
}
