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

type BusinessRoleSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.BusinessRoleRepository
	bizRepo  repository.BusinessRepository
}

func (s *BusinessRoleSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewBusinessRoleRepository(s.pool)
	s.bizRepo = repository.NewBusinessRepository(s.pool)
}

func (s *BusinessRoleSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BusinessRoleSuite) seedBusiness(userID, bizID string) *domain.Business {
	p := phone.MustParse("+251911100011")
	testutil.SeedUser(s.T(), s.pool, userID, p)
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
		PhoneNumber: phone.MustParse("+251911100025"),
	}
	s.Require().NoError(s.bizRepo.Create(context.Background(), biz))
	return biz
}

func (s *BusinessRoleSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	role := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "Finance Manager",
		IsSystem:   false,
	}
	err := s.repo.Create(ctx, role)
	s.Require().NoError(err)
	s.NotEmpty(role.ID)
	s.NotEmpty(role.CreatedAt)

	got, err := s.repo.GetByID(ctx, role.ID)
	s.Require().NoError(err)
	s.Equal(role.ID, got.ID)
	s.Equal(bizID, *got.BusinessID)
	s.Equal("Finance Manager", got.Name)
	s.False(got.IsSystem)
}

func (s *BusinessRoleSuite) TestGetByID() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	role := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "Accountant",
		IsSystem:   false,
	}
	s.Require().NoError(s.repo.Create(ctx, role))

	got, err := s.repo.GetByID(ctx, role.ID)
	s.Require().NoError(err)
	s.Equal(role.ID, got.ID)
	s.Equal("Accountant", got.Name)
}

func (s *BusinessRoleSuite) TestUpdate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	role := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "Original Name",
		IsSystem:   false,
	}
	s.Require().NoError(s.repo.Create(ctx, role))

	role.Name = "Updated Name"
	err := s.repo.Update(ctx, role)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, role.ID)
	s.Require().NoError(err)
	s.Equal("Updated Name", got.Name)
}

func (s *BusinessRoleSuite) TestDelete_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	role := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "To Delete",
		IsSystem:   false,
	}
	s.Require().NoError(s.repo.Create(ctx, role))

	err := s.repo.Delete(ctx, role.ID)
	s.Require().NoError(err)

	_, err = s.repo.GetByID(ctx, role.ID)
	s.ErrorIs(err, domain.ErrRoleNotFound)
}

func (s *BusinessRoleSuite) TestListByBusiness() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	role1 := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "Role A",
		IsSystem:   false,
	}
	role2 := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "Role B",
		IsSystem:   false,
	}
	s.Require().NoError(s.repo.Create(ctx, role1))
	s.Require().NoError(s.repo.Create(ctx, role2))

	list, err := s.repo.ListByBusiness(ctx, bizID)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(list), 2)
	// Verify our 2 roles are in the list
	var foundA, foundB bool
	for _, r := range list {
		if r.ID == role1.ID {
			foundA = true
		}
		if r.ID == role2.ID {
			foundB = true
		}
	}
	s.True(foundA, "Role A should be in list")
	s.True(foundB, "Role B should be in list")
}

func (s *BusinessRoleSuite) TestSetAndGetPermissions() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	role := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       "Perm Role",
		IsSystem:   false,
	}
	s.Require().NoError(s.repo.Create(ctx, role))

	perms := []domain.BusinessPermission{
		domain.BPermViewDashboard,
		domain.BPermViewBalances,
		domain.BPermTransferInternal,
	}
	err := s.repo.SetPermissions(ctx, role.ID, perms)
	s.Require().NoError(err)

	got, err := s.repo.GetPermissions(ctx, role.ID)
	s.Require().NoError(err)
	s.Len(got, 3)
	s.Contains(got, domain.BPermViewDashboard)
	s.Contains(got, domain.BPermViewBalances)
	s.Contains(got, domain.BPermTransferInternal)
}

func TestBusinessRoleSuite(t *testing.T) {
	suite.Run(t, new(BusinessRoleSuite))
}
