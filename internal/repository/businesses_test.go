package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type BusinessSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.BusinessRepository
}

func (s *BusinessSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewBusinessRepository(s.pool)
}

func (s *BusinessSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BusinessSuite) seedBusiness(userID, bizID string) *domain.Business {
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911100001"))
	tradeName := "Test Corp Trading"
	biz := &domain.Business{
		ID:                 bizID,
		OwnerUserID:        userID,
		Name:               "Test Corp",
		TradeName:          &tradeName,
		TINNumber:          uuid.NewString()[:10],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:6],
		IndustryCategory:   domain.IndustryRetail,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber:        phone.MustParse("+251911100002"),
	}
	err := s.repo.Create(context.Background(), biz)
	s.Require().NoError(err)
	return biz
}

func (s *BusinessSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911100003"))

	biz := &domain.Business{
		ID:                 uuid.NewString(),
		OwnerUserID:        userID,
		Name:               "Test Corp",
		TradeName:          strPtr("Test Corp Trading"),
		TINNumber:          uuid.NewString()[:10],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:6],
		IndustryCategory:   domain.IndustryRetail,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber:        phone.MustParse("+251911100004"),
	}
	err := s.repo.Create(ctx, biz)
	s.Require().NoError(err)
	s.NotEmpty(biz.ID)
	s.NotEmpty(biz.CreatedAt)

	got, err := s.repo.GetByID(ctx, biz.ID)
	s.Require().NoError(err)
	s.Equal(biz.ID, got.ID)
	s.Equal(userID, got.OwnerUserID)
	s.Equal("Test Corp", got.Name)
	s.Equal(domain.BusinessStatusActive, got.Status)
}

func (s *BusinessSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrBusinessNotFound)
}

func (s *BusinessSuite) TestUpdate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	biz, err := s.repo.GetByID(ctx, bizID)
	s.Require().NoError(err)
	biz.Name = "Updated Corp Name"
	err = s.repo.Update(ctx, biz)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, bizID)
	s.Require().NoError(err)
	s.Equal("Updated Corp Name", got.Name)
}

func (s *BusinessSuite) TestListByOwner() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911100005"))

	biz1 := &domain.Business{
		ID:                 uuid.NewString(),
		OwnerUserID:        userID,
		Name:               "Biz One",
		TINNumber:          uuid.NewString()[:10],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:6],
		IndustryCategory:   domain.IndustryRetail,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber:        phone.MustParse("+251911100006"),
	}
	biz2 := &domain.Business{
		ID:                 uuid.NewString(),
		OwnerUserID:        userID,
		Name:               "Biz Two",
		TINNumber:          uuid.NewString()[:10],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:6],
		IndustryCategory:   domain.IndustryWholesale,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber:        phone.MustParse("+251911100007"),
	}
	s.Require().NoError(s.repo.Create(ctx, biz1))
	s.Require().NoError(s.repo.Create(ctx, biz2))

	list, err := s.repo.ListByOwner(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *BusinessSuite) TestFreeze_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	err := s.repo.Freeze(ctx, bizID, "compliance")
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, bizID)
	s.Require().NoError(err)
	s.True(got.IsFrozen)
	s.Require().NotNil(got.FrozenReason)
	s.Equal("compliance", *got.FrozenReason)
}

func (s *BusinessSuite) TestUnfreeze_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	s.Require().NoError(s.repo.Freeze(ctx, bizID, "compliance"))
	s.Require().NoError(s.repo.Unfreeze(ctx, bizID))

	got, err := s.repo.GetByID(ctx, bizID)
	s.Require().NoError(err)
	s.False(got.IsFrozen)
	s.Nil(got.FrozenReason)
}

func strPtr(s string) *string {
	return &s
}

func TestBusinessSuite(t *testing.T) {
	suite.Run(t, new(BusinessSuite))
}
