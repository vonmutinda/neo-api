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
)

type StaffSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.StaffRepository
}

func (s *StaffSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewStaffRepository(s.pool)
}

func (s *StaffSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *StaffSuite) TestCreate_Success() {
	ctx := context.Background()
	staff := &domain.Staff{
		ID:           uuid.NewString(),
		Email:        "admin@neo.et",
		FullName:     "Admin User",
		Role:         domain.RoleSuperAdmin,
		Department:   "Engineering",
		IsActive:     true,
		PasswordHash: "$2a$10$fakehash",
	}
	err := s.repo.Create(ctx, staff)
	s.Require().NoError(err)
	s.NotEmpty(staff.ID)
	s.NotEmpty(staff.CreatedAt)

	got, err := s.repo.GetByID(ctx, staff.ID)
	s.Require().NoError(err)
	s.Equal(staff.ID, got.ID)
	s.Equal("admin@neo.et", got.Email)
	s.Equal("Admin User", got.FullName)
	s.Equal(domain.RoleSuperAdmin, got.Role)
}

func (s *StaffSuite) TestGetByEmail() {
	ctx := context.Background()
	staff := &domain.Staff{
		ID:           uuid.NewString(),
		Email:        "support@neo.et",
		FullName:     "Support Agent",
		Role:         domain.RoleCustomerSupport,
		Department:   "Support",
		IsActive:     true,
		PasswordHash: "$2a$10$fakehash",
	}
	s.Require().NoError(s.repo.Create(ctx, staff))

	got, err := s.repo.GetByEmail(ctx, "support@neo.et")
	s.Require().NoError(err)
	s.Equal(staff.ID, got.ID)
	s.Equal("support@neo.et", got.Email)
}

func (s *StaffSuite) TestUpdate() {
	ctx := context.Background()
	staff := &domain.Staff{
		ID:           uuid.NewString(),
		Email:        "compliance@neo.et",
		FullName:     "Compliance Officer",
		Role:         domain.RoleComplianceOfficer,
		Department:   "Compliance",
		IsActive:     true,
		PasswordHash: "$2a$10$fakehash",
	}
	s.Require().NoError(s.repo.Create(ctx, staff))

	staff.FullName = "Senior Compliance Officer"
	err := s.repo.Update(ctx, staff)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, staff.ID)
	s.Require().NoError(err)
	s.Equal("Senior Compliance Officer", got.FullName)
}

func (s *StaffSuite) TestDeactivate() {
	ctx := context.Background()
	staff := &domain.Staff{
		ID:           uuid.NewString(),
		Email:        "deactivate@neo.et",
		FullName:     "To Deactivate",
		Role:         domain.RoleCustomerSupport,
		Department:   "Support",
		IsActive:     true,
		PasswordHash: "$2a$10$fakehash",
	}
	s.Require().NoError(s.repo.Create(ctx, staff))

	err := s.repo.Deactivate(ctx, staff.ID)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, staff.ID)
	s.Require().NoError(err)
	s.False(got.IsActive)
}

func TestStaffSuite(t *testing.T) {
	suite.Run(t, new(StaffSuite))
}
