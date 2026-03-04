package pricing_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/pricing"
	"github.com/vonmutinda/neo/internal/testutil"
)

type PricingSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	svc       *pricing.Service
	feeRepo   repository.FeeScheduleRepository
	mockCache *testutil.MockCache
}

func (s *PricingSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.mockCache = testutil.NewMockCache()
	s.feeRepo = repository.NewFeeScheduleRepository(s.pool)
	s.svc = pricing.NewService(s.feeRepo, nil, s.mockCache)
}

func (s *PricingSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	_ = s.mockCache.DeleteByPrefix(context.Background(), "neo:")
}

func (s *PricingSuite) seedFeeSchedule(name string, txType domain.TransactionType, feeType domain.FeeType, flatCents int64, percentBps int, minCents, maxCents int64) {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO fee_schedules (name, transaction_type, fee_type, flat_amount_cents, percent_bps, min_fee_cents, max_fee_cents, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, true)`,
		name, txType, feeType, flatCents, percentBps, minCents, maxCents,
	)
	s.Require().NoError(err)
}

func (s *PricingSuite) TestCalculateFee_FlatFee() {
	s.seedFeeSchedule("P2P flat fee", domain.TxTypeP2P, domain.FeeTypeTransferFlat, 500, 0, 0, 0)

	breakdown, err := s.svc.CalculateFee(context.Background(), domain.TxTypeP2P, 10000, "ETB", nil)
	s.Require().NoError(err)
	s.Equal(int64(500), breakdown.OurFeeCents)
	s.Equal(int64(500), breakdown.TotalFeeCents)
}

func (s *PricingSuite) TestCalculateFee_PercentFee() {
	s.seedFeeSchedule("P2P percent fee", domain.TxTypeP2P, domain.FeeTypeTransferPercent, 0, 100, 0, 0)

	breakdown, err := s.svc.CalculateFee(context.Background(), domain.TxTypeP2P, 100000, "ETB", nil)
	s.Require().NoError(err)
	s.Equal(int64(1000), breakdown.OurFeeCents)
}

func (s *PricingSuite) TestCalculateFee_MinMaxCapping() {
	s.seedFeeSchedule("Capped percent fee", domain.TxTypeP2P, domain.FeeTypeTransferPercent, 0, 100, 200, 5000)

	// Small amount: 1% of 5000 = 50, should be clamped to min 200
	breakdown, err := s.svc.CalculateFee(context.Background(), domain.TxTypeP2P, 5000, "ETB", nil)
	s.Require().NoError(err)
	s.Equal(int64(200), breakdown.OurFeeCents, "min fee should be applied")

	// Large amount: 1% of 10_000_000 = 100_000, should be clamped to max 5000
	breakdown, err = s.svc.CalculateFee(context.Background(), domain.TxTypeP2P, 10_000_000, "ETB", nil)
	s.Require().NoError(err)
	s.Equal(int64(5000), breakdown.OurFeeCents, "max fee should be applied")
}

func (s *PricingSuite) TestGetFXSpreadBps() {
	s.seedFeeSchedule("FX spread", domain.TxTypeFXConversion, domain.FeeTypeFXSpread, 0, 150, 0, 0)

	bps, err := s.svc.GetFXSpreadBps(context.Background())
	s.Require().NoError(err)
	s.Equal(150, bps)
}

func TestPricingSuite(t *testing.T) {
	suite.Run(t, new(PricingSuite))
}
