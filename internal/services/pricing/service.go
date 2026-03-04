package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/remittance"
	"github.com/vonmutinda/neo/pkg/cache"
)

type Service struct {
	schedules repository.FeeScheduleRepository
	router    *remittance.ProviderRouter
	cache     cache.Cache
}

func NewService(
	schedules repository.FeeScheduleRepository,
	router *remittance.ProviderRouter,
	c cache.Cache,
) *Service {
	return &Service{
		schedules: schedules,
		router:    router,
		cache:     c,
	}
}

func (s *Service) CalculateFee(
	ctx context.Context,
	txType domain.TransactionType,
	amountCents int64,
	currency string,
	channel *string,
) (*domain.FeeBreakdown, error) {
	cacheKey := "neo:fees:" + string(txType) + "|" + currency + "|"
	if channel != nil {
		cacheKey += *channel
	}

	var schedules []domain.FeeSchedule
	if data, ok := s.cache.Get(ctx, cacheKey); ok {
		_ = json.Unmarshal(data, &schedules)
	}

	if schedules == nil {
		var err error
		schedules, err = s.schedules.FindMatching(ctx, txType, &currency, channel)
		if err != nil {
			return nil, fmt.Errorf("finding fee schedules: %w", err)
		}
		if len(schedules) == 0 {
			schedules, err = s.schedules.FindMatching(ctx, txType, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("finding global fee schedules: %w", err)
			}
		}
		if data, err := json.Marshal(schedules); err == nil {
			_ = s.cache.Set(ctx, cacheKey, data, 60*time.Second)
		}
	}

	breakdown := &domain.FeeBreakdown{}
	for _, sched := range schedules {
		if sched.FeeType == domain.FeeTypeFXSpread {
			continue
		}
		fee := computeFee(sched, amountCents)
		breakdown.OurFeeCents += fee
		breakdown.Details = append(breakdown.Details, domain.FeeDetail{
			Name:        sched.Name,
			AmountCents: fee,
			Type:        string(sched.FeeType),
		})
	}
	breakdown.TotalFeeCents = breakdown.OurFeeCents + breakdown.PartnerFeeCents
	return breakdown, nil
}

func (s *Service) GetQuote(
	ctx context.Context,
	sourceCurrency, targetCurrency string,
	sourceAmountCents int64,
) (*domain.RemittanceQuote, error) {
	if s.router == nil {
		return nil, domain.ErrNoProviderForCorridor
	}

	ranked, err := s.router.BestQuote(ctx, remittance.QuoteRequest{
		SourceCurrency: sourceCurrency,
		TargetCurrency: targetCurrency,
		SourceAmount:   sourceAmountCents,
	})
	if err != nil {
		return nil, err
	}

	markupSchedules, _ := s.schedules.FindMatching(ctx, domain.TxTypeInternationalTransfer, nil, nil)
	var ourMarkupCents int64
	for _, sched := range markupSchedules {
		if sched.FeeType == domain.FeeTypeCorridorMarkup {
			ourMarkupCents += computeFee(sched, sourceAmountCents)
		}
	}

	quote := ranked.Winner
	quote.Fee = &domain.FeeBreakdown{
		OurFeeCents:     ourMarkupCents,
		PartnerFeeCents: quote.ProviderFeeCents,
		TotalFeeCents:   ourMarkupCents + quote.ProviderFeeCents,
		Details: []domain.FeeDetail{
			{Name: "Neo transfer fee", AmountCents: ourMarkupCents, Type: string(domain.FeeTypeCorridorMarkup)},
			{Name: quote.ProviderName + " processing fee", AmountCents: quote.ProviderFeeCents, Type: "partner"},
		},
	}

	return quote, nil
}

func (s *Service) GetFXSpreadBps(ctx context.Context) (int, error) {
	cacheKey := "neo:fees:fx_conversion||"
	if data, ok := s.cache.Get(ctx, cacheKey); ok {
		var schedules []domain.FeeSchedule
		if json.Unmarshal(data, &schedules) == nil {
			for _, sched := range schedules {
				if sched.FeeType == domain.FeeTypeFXSpread {
					return sched.PercentBps, nil
				}
			}
		}
	}

	schedules, err := s.schedules.FindMatching(ctx, domain.TxTypeFXConversion, nil, nil)
	if err != nil {
		return 150, nil
	}
	if data, err := json.Marshal(schedules); err == nil {
		_ = s.cache.Set(ctx, cacheKey, data, 60*time.Second)
	}
	for _, sched := range schedules {
		if sched.FeeType == domain.FeeTypeFXSpread {
			return sched.PercentBps, nil
		}
	}
	return 150, nil
}

func computeFee(sched domain.FeeSchedule, amountCents int64) int64 {
	switch sched.FeeType {
	case domain.FeeTypeTransferFlat:
		return sched.FlatAmountCents
	case domain.FeeTypeTransferPercent, domain.FeeTypeCorridorMarkup:
		fee := amountCents * int64(sched.PercentBps) / 10000
		if sched.MinFeeCents > 0 && fee < sched.MinFeeCents {
			fee = sched.MinFeeCents
		}
		if sched.MaxFeeCents > 0 && fee > sched.MaxFeeCents {
			fee = sched.MaxFeeCents
		}
		return fee
	default:
		return 0
	}
}
