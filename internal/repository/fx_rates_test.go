package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
)

type FXRateSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.FXRateRepository
}

func (s *FXRateSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewFXRateRepository(s.pool)
}

func (s *FXRateSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *FXRateSuite) TestInsert_Success() {
	ctx := context.Background()
	rate := &domain.FXRate{
		FromCurrency:  "ETB",
		ToCurrency:    "USD",
		MidRate:       58.5,
		BidRate:       58.3,
		AskRate:       58.7,
		SpreadPercent: 0.5,
		Source:        domain.FXRateSourceNBE,
		FetchedAt:     time.Now(),
	}
	err := s.repo.Insert(ctx, rate)
	s.Require().NoError(err)
	s.NotEmpty(rate.ID)

	got, err := s.repo.GetLatest(ctx, "ETB", "USD")
	s.Require().NoError(err)
	s.Equal(rate.ID, got.ID)
	s.Equal(58.5, got.MidRate)
}

func (s *FXRateSuite) TestGetLatest() {
	ctx := context.Background()
	now := time.Now()
	r1 := &domain.FXRate{
		FromCurrency:  "ETB",
		ToCurrency:    "USD",
		MidRate:       58.0,
		BidRate:       57.8,
		AskRate:       58.2,
		SpreadPercent: 0.5,
		Source:        domain.FXRateSourceNBE,
		FetchedAt:     now.Add(-2 * time.Hour),
	}
	r2 := &domain.FXRate{
		FromCurrency:  "ETB",
		ToCurrency:    "USD",
		MidRate:       59.0,
		BidRate:       58.8,
		AskRate:       59.2,
		SpreadPercent: 0.5,
		Source:        domain.FXRateSourceNBE,
		FetchedAt:     now.Add(-1 * time.Hour),
	}
	s.Require().NoError(s.repo.Insert(ctx, r1))
	s.Require().NoError(s.repo.Insert(ctx, r2))

	got, err := s.repo.GetLatest(ctx, "ETB", "USD")
	s.Require().NoError(err)
	s.Equal(r2.ID, got.ID)
	s.Equal(59.0, got.MidRate)
}

func (s *FXRateSuite) TestGetLatestAll() {
	ctx := context.Background()
	now := time.Now()
	s.Require().NoError(s.repo.Insert(ctx, &domain.FXRate{
		FromCurrency:  "ETB",
		ToCurrency:    "USD",
		MidRate:       58.5,
		BidRate:       58.3,
		AskRate:       58.7,
		SpreadPercent: 0.5,
		Source:        domain.FXRateSourceNBE,
		FetchedAt:     now,
	}))
	s.Require().NoError(s.repo.Insert(ctx, &domain.FXRate{
		FromCurrency:  "ETB",
		ToCurrency:    "EUR",
		MidRate:       62.0,
		BidRate:       61.8,
		AskRate:       62.2,
		SpreadPercent: 0.5,
		Source:        domain.FXRateSourceNBE,
		FetchedAt:     now,
	}))

	rates, err := s.repo.GetLatestAll(ctx)
	s.Require().NoError(err)
	s.Len(rates, 2)
}

func (s *FXRateSuite) TestListHistory() {
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 3; i++ {
		s.Require().NoError(s.repo.Insert(ctx, &domain.FXRate{
			FromCurrency:  "ETB",
			ToCurrency:    "USD",
			MidRate:       58.0 + float64(i),
			BidRate:       57.8 + float64(i),
			AskRate:       58.2 + float64(i),
			SpreadPercent: 0.5,
			Source:        domain.FXRateSourceNBE,
			FetchedAt:     now.Add(time.Duration(i) * time.Hour),
		}))
	}

	rates, err := s.repo.ListHistory(ctx, "ETB", "USD", now.Add(-1*time.Hour), 10)
	s.Require().NoError(err)
	s.Len(rates, 3)
}

func (s *FXRateSuite) TestDeleteOlderThan() {
	ctx := context.Background()
	// Insert via raw SQL so we can set created_at in the past (repo Insert uses NOW() for created_at)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO fx_rates (from_currency, to_currency, mid_rate, bid_rate, ask_rate, spread_percent, source, fetched_at, created_at)
		VALUES ('ETB', 'GBP', 72.0, 71.8, 72.2, 0.5, 'nbe_indicative', NOW() - INTERVAL '48 hours', NOW() - INTERVAL '48 hours')`)
	s.Require().NoError(err)

	deleted, err := s.repo.DeleteOlderThan(ctx, time.Now().Add(-24*time.Hour))
	s.Require().NoError(err)
	s.GreaterOrEqual(deleted, int64(1))

	_, err = s.repo.GetLatest(ctx, "ETB", "GBP")
	s.ErrorIs(err, domain.ErrFXRateNotFound)
}

func TestFXRateSuite(t *testing.T) {
	suite.Run(t, new(FXRateSuite))
}
