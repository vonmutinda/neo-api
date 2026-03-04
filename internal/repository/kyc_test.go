package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type KYCSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.KYCRepository
}

func (s *KYCSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewKYCRepository(s.pool)
}

func (s *KYCSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *KYCSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440001"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	v := &domain.KYCVerification{
		UserID:             userID,
		FaydaFIN:           "123456789012",
		FaydaTransactionID: "tx-001",
		Status:             domain.KYCStatusPending,
	}
	err := s.repo.Create(ctx, v)
	s.Require().NoError(err)
	s.NotEmpty(v.ID)
	s.NotEmpty(v.CreatedAt)
}

func (s *KYCSuite) TestGetByUserID() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440002"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	v1 := &domain.KYCVerification{
		UserID:             userID,
		FaydaFIN:           "111111111111",
		FaydaTransactionID: "tx-1",
		Status:             domain.KYCStatusPending,
	}
	v2 := &domain.KYCVerification{
		UserID:             userID,
		FaydaFIN:           "222222222222",
		FaydaTransactionID: "tx-2",
		Status:             domain.KYCStatusPending,
	}
	s.Require().NoError(s.repo.Create(ctx, v1))
	s.Require().NoError(s.repo.Create(ctx, v2))

	list, err := s.repo.GetByUserID(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
	s.Equal("tx-2", list[0].FaydaTransactionID)
	s.Equal("tx-1", list[1].FaydaTransactionID)
}

func (s *KYCSuite) TestUpdateStatus_Verified() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440003"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251922222222"))

	v := &domain.KYCVerification{
		UserID:             userID,
		FaydaFIN:           "333333333333",
		FaydaTransactionID: "tx-3",
		Status:             domain.KYCStatusPending,
	}
	s.Require().NoError(s.repo.Create(ctx, v))

	err := s.repo.UpdateStatus(ctx, v.ID, domain.KYCStatusVerified)
	s.Require().NoError(err)

	list, err := s.repo.GetByUserID(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 1)
	s.Equal(domain.KYCStatusVerified, list[0].Status)
	s.Require().NotNil(list[0].VerifiedAt)
}

func (s *KYCSuite) TestUpdateStatus_Failed() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440004"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251933333333"))

	v := &domain.KYCVerification{
		UserID:             userID,
		FaydaFIN:           "444444444444",
		FaydaTransactionID: "tx-4",
		Status:             domain.KYCStatusPending,
	}
	s.Require().NoError(s.repo.Create(ctx, v))

	err := s.repo.UpdateStatus(ctx, v.ID, domain.KYCStatusFailed)
	s.Require().NoError(err)

	list, err := s.repo.GetByUserID(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 1)
	s.Equal(domain.KYCStatusFailed, list[0].Status)
	s.Nil(list[0].VerifiedAt)
}

func TestKYCSuite(t *testing.T) {
	suite.Run(t, new(KYCSuite))
}
