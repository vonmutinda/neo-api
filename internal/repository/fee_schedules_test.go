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

type FeeScheduleSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.FeeScheduleRepository
}

func (s *FeeScheduleSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewFeeScheduleRepository(s.pool)
}

func (s *FeeScheduleSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *FeeScheduleSuite) TestCreate_Success() {
	ctx := context.Background()
	effectiveFrom := time.Now().Add(-time.Hour)
	fs := &domain.FeeSchedule{
		Name:            "P2P Flat Fee",
		FeeType:         domain.FeeTypeTransferFlat,
		TransactionType: domain.TxTypeP2P,
		FlatAmountCents: 100,
		PercentBps:      0,
		MinFeeCents:     0,
		MaxFeeCents:     0,
		IsActive:        true,
		EffectiveFrom:   effectiveFrom,
	}
	err := s.repo.Create(ctx, fs)
	s.Require().NoError(err)
	s.NotEmpty(fs.ID)
}

func (s *FeeScheduleSuite) TestGetByID() {
	ctx := context.Background()
	fs := &domain.FeeSchedule{
		Name:            "Get By ID Schedule",
		FeeType:         domain.FeeTypeTransferFlat,
		TransactionType: domain.TxTypeP2P,
		FlatAmountCents: 50,
		PercentBps:      0,
		MinFeeCents:     0,
		MaxFeeCents:     0,
		IsActive:        true,
		EffectiveFrom:   time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, fs))

	got, err := s.repo.GetByID(ctx, fs.ID)
	s.Require().NoError(err)
	s.Equal(fs.ID, got.ID)
	s.Equal("Get By ID Schedule", got.Name)
}

func (s *FeeScheduleSuite) TestListActive() {
	ctx := context.Background()
	effectiveFrom := time.Now().Add(-time.Hour)
	fs1 := &domain.FeeSchedule{
		Name:            "Active 1",
		FeeType:         domain.FeeTypeTransferFlat,
		TransactionType: domain.TxTypeP2P,
		FlatAmountCents: 100,
		PercentBps:      0,
		MinFeeCents:     0,
		MaxFeeCents:     0,
		IsActive:        true,
		EffectiveFrom:   effectiveFrom,
	}
	fs2 := &domain.FeeSchedule{
		Name:            "Active 2",
		FeeType:         domain.FeeTypeTransferPercent,
		TransactionType: domain.TxTypeP2P,
		FlatAmountCents: 0,
		PercentBps:      50,
		MinFeeCents:     0,
		MaxFeeCents:     500,
		IsActive:        true,
		EffectiveFrom:   effectiveFrom,
	}
	s.Require().NoError(s.repo.Create(ctx, fs1))
	s.Require().NoError(s.repo.Create(ctx, fs2))

	list, err := s.repo.ListActive(ctx, domain.TxTypeP2P)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *FeeScheduleSuite) TestDeactivate() {
	ctx := context.Background()
	fs := &domain.FeeSchedule{
		Name:            "To Deactivate",
		FeeType:         domain.FeeTypeTransferFlat,
		TransactionType: domain.TxTypeP2P,
		FlatAmountCents: 100,
		PercentBps:      0,
		MinFeeCents:     0,
		MaxFeeCents:     0,
		IsActive:        true,
		EffectiveFrom:   time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, fs))

	err := s.repo.Deactivate(ctx, fs.ID)
	s.Require().NoError(err)

	list, err := s.repo.ListActive(ctx, domain.TxTypeP2P)
	s.Require().NoError(err)
	for _, item := range list {
		s.NotEqual(fs.ID, item.ID)
	}
}

func (s *FeeScheduleSuite) TestFindMatching() {
	ctx := context.Background()
	fs := &domain.FeeSchedule{
		Name:            "P2P Match",
		FeeType:         domain.FeeTypeTransferFlat,
		TransactionType: domain.TxTypeP2P,
		Currency:        nil,
		Channel:        nil,
		FlatAmountCents: 75,
		PercentBps:      0,
		MinFeeCents:     0,
		MaxFeeCents:     0,
		IsActive:        true,
		EffectiveFrom:   time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, fs))

	matches, err := s.repo.FindMatching(ctx, domain.TxTypeP2P, nil, nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(matches)
	s.Equal("P2P Match", matches[0].Name)
}

func TestFeeScheduleSuite(t *testing.T) {
	suite.Run(t, new(FeeScheduleSuite))
}
