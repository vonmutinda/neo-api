package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type OverdraftSuite struct {
	suite.Suite
	pool *pgxpool.Pool
	repo repository.OverdraftRepository
}

func (s *OverdraftSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewOverdraftRepository(s.pool)
}

func (s *OverdraftSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *OverdraftSuite) TestGetByUser_NotFound() {
	ctx := context.Background()
	got, err := s.repo.GetByUser(ctx, uuid.NewString())
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *OverdraftSuite) TestCreateOrUpdate_Create() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000001"))

	now := time.Now().UTC()
	o := &domain.Overdraft{
		UserID:              userID,
		LimitCents:          50000,
		DailyFeeBasisPoints:  15,
		InterestFreeDays:     7,
		Status:               domain.OverdraftActive,
		OptedInAt:            &now,
	}
	err := s.repo.CreateOrUpdate(ctx, o)
	s.Require().NoError(err)
	s.NotEmpty(o.ID)
	s.Equal(int64(0), o.UsedCents)
	s.Equal(int64(50000), o.AvailableCents)
}

func (s *OverdraftSuite) TestCreateOrUpdate_Update() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000002"))

	now := time.Now().UTC()
	o := &domain.Overdraft{
		UserID:              userID,
		LimitCents:          30000,
		DailyFeeBasisPoints:  15,
		InterestFreeDays:     7,
		Status:               domain.OverdraftActive,
		OptedInAt:            &now,
	}
	s.Require().NoError(s.repo.CreateOrUpdate(ctx, o))

	o.LimitCents = 60000
	s.Require().NoError(s.repo.CreateOrUpdate(ctx, o))

	got, err := s.repo.GetByUser(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(int64(60000), got.LimitCents)
	s.Equal(o.ID, got.ID)
}

func (s *OverdraftSuite) TestUpdateUsedAndStatus() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000003"))

	now := time.Now().UTC()
	o := &domain.Overdraft{
		UserID:              userID,
		LimitCents:          50000,
		DailyFeeBasisPoints:  15,
		InterestFreeDays:     7,
		Status:               domain.OverdraftActive,
		OptedInAt:            &now,
	}
	s.Require().NoError(s.repo.CreateOrUpdate(ctx, o))

	overdrawnAt := time.Now().UTC()
	err := s.repo.UpdateUsedAndStatus(ctx, userID, 10000, domain.OverdraftUsed, &overdrawnAt)
	s.Require().NoError(err)

	got, err := s.repo.GetByUser(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(int64(10000), got.UsedCents)
	s.Equal(domain.OverdraftUsed, got.Status)
	s.NotNil(got.OverdrawnSince)
}

func (s *OverdraftSuite) TestUpdateRepaid() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000004"))

	now := time.Now().UTC()
	o := &domain.Overdraft{
		UserID:              userID,
		LimitCents:          50000,
		UsedCents:           10000,
		DailyFeeBasisPoints:  15,
		InterestFreeDays:     7,
		Status:               domain.OverdraftUsed,
		OptedInAt:            &now,
	}
	s.Require().NoError(s.repo.CreateOrUpdate(ctx, o))
	// Simulate used state
	_, err := s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 10000, accrued_fee_cents = 50, status = 'used', overdrawn_since = NOW() WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	err = s.repo.UpdateRepaid(ctx, userID, 0, 0, domain.OverdraftActive)
	s.Require().NoError(err)

	got, err := s.repo.GetByUser(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(int64(0), got.UsedCents)
	s.Equal(int64(0), got.AccruedFeeCents)
	s.Equal(domain.OverdraftActive, got.Status)
	s.Nil(got.OverdrawnSince)
}

func (s *OverdraftSuite) TestUpdateFeeAccrual() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000005"))

	now := time.Now().UTC()
	o := &domain.Overdraft{
		UserID:              userID,
		LimitCents:          50000,
		DailyFeeBasisPoints:  15,
		InterestFreeDays:     7,
		Status:               domain.OverdraftActive,
		OptedInAt:            &now,
	}
	s.Require().NoError(s.repo.CreateOrUpdate(ctx, o))
	_, err := s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 10000, status = 'used' WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	accrualAt := time.Now().UTC()
	err = s.repo.UpdateFeeAccrual(ctx, userID, 45, accrualAt)
	s.Require().NoError(err)

	got, err := s.repo.GetByUser(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(int64(45), got.AccruedFeeCents)
	s.Require().NotNil(got.LastFeeAccrualAt)
}

func TestOverdraftSuite(t *testing.T) {
	suite.Run(t, new(OverdraftSuite))
}
