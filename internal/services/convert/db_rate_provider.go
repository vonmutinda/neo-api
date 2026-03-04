package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/cache"
)

type DatabaseRateProvider struct {
	repo          repository.FXRateRepository
	spreadPercent float64
	cacheTTL      time.Duration
	cache         cache.Cache
}

func NewDatabaseRateProvider(
	repo repository.FXRateRepository,
	spreadPercent float64,
	cacheTTL time.Duration,
	c cache.Cache,
) *DatabaseRateProvider {
	return &DatabaseRateProvider{
		repo:          repo,
		spreadPercent: spreadPercent,
		cacheTTL:      cacheTTL,
		cache:         c,
	}
}

func (p *DatabaseRateProvider) GetRate(ctx context.Context, from, to string) (Rate, error) {
	if from == to {
		return Rate{
			From: from, To: to,
			Mid: 1.0, Bid: 1.0, Ask: 1.0,
			Timestamp: time.Now(), Source: "identity",
		}, nil
	}

	key := "neo:fx:" + from + "_" + to

	if data, ok := p.cache.Get(ctx, key); ok {
		var r Rate
		if json.Unmarshal(data, &r) == nil {
			return r, nil
		}
	}

	dbRate, err := p.repo.GetLatest(ctx, from, to)
	if err != nil {
		return Rate{}, fmt.Errorf("getting latest rate for %s/%s: %w", from, to, err)
	}

	spreadFactor := p.spreadPercent / 100.0
	bid := dbRate.MidRate * (1 - spreadFactor/2)
	ask := dbRate.MidRate * (1 + spreadFactor/2)

	rate := Rate{
		From:      from,
		To:        to,
		Mid:       dbRate.MidRate,
		Bid:       bid,
		Ask:       ask,
		Spread:    p.spreadPercent,
		Timestamp: dbRate.FetchedAt,
		Source:    string(dbRate.Source),
	}

	if data, err := json.Marshal(rate); err == nil {
		_ = p.cache.Set(ctx, key, data, p.cacheTTL)
	}

	return rate, nil
}

var _ RateProvider = (*DatabaseRateProvider)(nil)
