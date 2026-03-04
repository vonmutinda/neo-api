package remittance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/cache"
)

type ProviderRouter struct {
	providers []RemittanceProvider
	cache     cache.Cache
	ttl       time.Duration
}

func NewProviderRouter(providers []RemittanceProvider, cacheTTL time.Duration, c cache.Cache) *ProviderRouter {
	return &ProviderRouter{
		providers: providers,
		cache:     c,
		ttl:       cacheTTL,
	}
}

type RankedQuote struct {
	Winner    *domain.RemittanceQuote
	Provider  RemittanceProvider
	AllQuotes []*domain.RemittanceQuote
}

func (r *ProviderRouter) BestQuote(ctx context.Context, req QuoteRequest) (*RankedQuote, error) {
	key := "neo:quotes:" + req.SourceCurrency + "_" + req.TargetCurrency + "_" + fmt.Sprintf("%d", req.SourceAmount)

	if data, ok := r.cache.Get(ctx, key); ok {
		var quote domain.RemittanceQuote
		if json.Unmarshal(data, &quote) == nil {
			provider := r.GetProvider(quote.ProviderID)
			return &RankedQuote{Winner: &quote, Provider: provider}, nil
		}
	}

	var matching []RemittanceProvider
	for _, p := range r.providers {
		for _, c := range p.SupportedCorridors() {
			if c.SourceCurrency == req.SourceCurrency && c.TargetCurrency == req.TargetCurrency {
				matching = append(matching, p)
				break
			}
		}
	}

	if len(matching) == 0 {
		return nil, domain.ErrNoProviderForCorridor
	}

	type quoteResult struct {
		quote    *domain.RemittanceQuote
		provider RemittanceProvider
		err      error
	}

	results := make(chan quoteResult, len(matching))
	quoteCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, p := range matching {
		wg.Add(1)
		go func(prov RemittanceProvider) {
			defer wg.Done()
			q, err := prov.GetQuote(quoteCtx, req)
			results <- quoteResult{quote: q, provider: prov, err: err}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allQuotes []*domain.RemittanceQuote
	var bestQuote *domain.RemittanceQuote
	var bestProvider RemittanceProvider
	var bestCost int64 = -1

	for res := range results {
		if res.err != nil {
			slog.Warn("provider quote failed",
				slog.String("provider", res.provider.ID()),
				slog.String("error", res.err.Error()))
			continue
		}
		allQuotes = append(allQuotes, res.quote)
		cost := res.quote.SourceAmount - res.quote.TargetAmount
		if bestCost < 0 || cost < bestCost {
			bestCost = cost
			bestQuote = res.quote
			bestProvider = res.provider
		}
	}

	if bestQuote == nil {
		return nil, domain.ErrProviderUnavailable
	}

	ranked := &RankedQuote{
		Winner:    bestQuote,
		Provider:  bestProvider,
		AllQuotes: allQuotes,
	}

	if data, err := json.Marshal(ranked.Winner); err == nil {
		_ = r.cache.Set(ctx, key, data, r.ttl)
	}
	return ranked, nil
}

func (r *ProviderRouter) GetProvider(id string) RemittanceProvider {
	for _, p := range r.providers {
		if p.ID() == id {
			return p
		}
	}
	return nil
}
