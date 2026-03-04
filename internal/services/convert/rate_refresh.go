package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

// RateRefreshService periodically fetches rates from a RateSource and
// persists them to the fx_rates table.
type RateRefreshService struct {
	source        RateSource
	repo          repository.FXRateRepository
	audit         repository.AuditRepository
	spreadPercent float64
}

func NewRateRefreshService(
	source RateSource,
	repo repository.FXRateRepository,
	audit repository.AuditRepository,
	spreadPercent float64,
) *RateRefreshService {
	return &RateRefreshService{
		source:        source,
		repo:          repo,
		audit:         audit,
		spreadPercent: spreadPercent,
	}
}

// Refresh fetches rates from the upstream source, computes bid/ask with spread,
// and inserts them into the database.
func (s *RateRefreshService) Refresh(ctx context.Context) error {
	rawRates, err := s.source.FetchRates(ctx)
	if err != nil {
		return fmt.Errorf("fetching rates from %s: %w", s.source.Name(), err)
	}

	spreadFactor := s.spreadPercent / 100.0
	inserted := 0

	for _, raw := range rawRates {
		bid := raw.MidRate * (1 - spreadFactor/2)
		ask := raw.MidRate * (1 + spreadFactor/2)

		rate := &domain.FXRate{
			FromCurrency:  raw.From,
			ToCurrency:    raw.To,
			MidRate:       raw.MidRate,
			BidRate:       bid,
			AskRate:       ask,
			SpreadPercent: s.spreadPercent,
			Source:        domain.FXRateSource(s.source.Name()),
			FetchedAt:     raw.FetchedAt,
		}

		if err := s.repo.Insert(ctx, rate); err != nil {
			slog.Warn("failed to insert rate",
				slog.String("pair", raw.From+"/"+raw.To),
				slog.String("error", err.Error()))
			continue
		}
		inserted++
	}

	slog.Info("FX rate refresh complete",
		slog.String("source", s.source.Name()),
		slog.Int("inserted", inserted),
		slog.Int("total", len(rawRates)))

	meta, _ := json.Marshal(map[string]any{
		"source": s.source.Name(),
		"pairs":  inserted,
	})
	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditFXRateUpdated,
		ActorType:    "system",
		ResourceType: "fx_rates",
		ResourceID:   "batch",
		Metadata:     meta,
	})

	return nil
}

// Cleanup removes rate rows older than the given retention period.
func (s *RateRefreshService) Cleanup(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	n, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleaning up old fx rates: %w", err)
	}
	return n, nil
}
