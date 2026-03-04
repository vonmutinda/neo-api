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

type FlagSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.FlagRepository
}

func (s *FlagSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewFlagRepository(s.pool)
}

func (s *FlagSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *FlagSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	flag := &domain.CustomerFlag{
		UserID:     userID,
		FlagType:   "suspicious_activity",
		Severity:   domain.FlagSeverityWarning,
		Description: "Unusual transfer pattern",
	}
	err := s.repo.Create(ctx, flag)
	s.Require().NoError(err)
	s.NotEmpty(flag.ID)
}

func (s *FlagSuite) TestGetByID() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	flag := &domain.CustomerFlag{
		UserID:      userID,
		FlagType:    "kyc_mismatch",
		Severity:    domain.FlagSeverityCritical,
		Description: "ID document mismatch",
	}
	s.Require().NoError(s.repo.Create(ctx, flag))

	got, err := s.repo.GetByID(ctx, flag.ID)
	s.Require().NoError(err)
	s.Equal(flag.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal("kyc_mismatch", got.FlagType)
}

func (s *FlagSuite) TestListByUser() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251922222222"))

	f1 := &domain.CustomerFlag{UserID: userID, FlagType: "flag_a", Severity: domain.FlagSeverityInfo, Description: "First"}
	f2 := &domain.CustomerFlag{UserID: userID, FlagType: "flag_b", Severity: domain.FlagSeverityWarning, Description: "Second"}
	s.Require().NoError(s.repo.Create(ctx, f1))
	s.Require().NoError(s.repo.Create(ctx, f2))

	list, err := s.repo.ListByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *FlagSuite) TestResolve() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251933333333"))

	staff := &domain.Staff{
		ID:           uuid.NewString(),
		Email:        "resolver@neo.et",
		FullName:     "Resolver",
		Role:         domain.RoleComplianceOfficer,
		Department:   "Compliance",
		IsActive:     true,
		PasswordHash: "$2a$10$fakehash",
	}
	s.Require().NoError(repository.NewStaffRepository(s.pool).Create(ctx, staff))

	flag := &domain.CustomerFlag{
		UserID:      userID,
		FlagType:    "resolvable",
		Severity:    domain.FlagSeverityInfo,
		Description: "To resolve",
	}
	s.Require().NoError(s.repo.Create(ctx, flag))

	err := s.repo.Resolve(ctx, flag.ID, staff.ID, "Verified and cleared")
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, flag.ID)
	s.Require().NoError(err)
	s.True(got.IsResolved)
	s.Require().NotNil(got.ResolvedBy)
	s.Equal(staff.ID, *got.ResolvedBy)
}

func TestFlagSuite(t *testing.T) {
	suite.Run(t, new(FlagSuite))
}
