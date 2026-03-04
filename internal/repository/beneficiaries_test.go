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

type BeneficiarySuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.BeneficiaryRepository
}

func (s *BeneficiarySuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewBeneficiaryRepository(s.pool)
}

func (s *BeneficiarySuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BeneficiarySuite) TestCreate_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440001"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	beneficiary := &domain.Beneficiary{
		UserID:       userID,
		FullName:     "Abebe Kebede",
		Relationship: domain.BeneficiarySpouse,
	}
	err := s.repo.Create(ctx, beneficiary)
	s.Require().NoError(err)
	s.NotEmpty(beneficiary.ID)
	s.NotEmpty(beneficiary.CreatedAt)

	got, err := s.repo.GetByID(ctx, beneficiary.ID)
	s.Require().NoError(err)
	s.Equal(beneficiary.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal("Abebe Kebede", got.FullName)
	s.Equal(domain.BeneficiarySpouse, got.Relationship)
}

func (s *BeneficiarySuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, "550e8400-e29b-41d4-a716-446655440099")
	s.ErrorIs(err, domain.ErrBeneficiaryNotFound)
}

func (s *BeneficiarySuite) TestListByUser() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440002"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	b1 := &domain.Beneficiary{UserID: userID, FullName: "First", Relationship: domain.BeneficiaryChild}
	b2 := &domain.Beneficiary{UserID: userID, FullName: "Second", Relationship: domain.BeneficiaryParent}
	s.Require().NoError(s.repo.Create(ctx, b1))
	s.Require().NoError(s.repo.Create(ctx, b2))

	list, err := s.repo.ListByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *BeneficiarySuite) TestSoftDelete_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440003"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251922222222"))

	beneficiary := &domain.Beneficiary{UserID: userID, FullName: "To Delete", Relationship: domain.BeneficiarySpouse}
	s.Require().NoError(s.repo.Create(ctx, beneficiary))

	err := s.repo.SoftDelete(ctx, beneficiary.ID, userID)
	s.Require().NoError(err)

	_, err = s.repo.GetByID(ctx, beneficiary.ID)
	s.ErrorIs(err, domain.ErrBeneficiaryNotFound)
}

func (s *BeneficiarySuite) TestSoftDelete_NotFound() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440004"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251933333333"))

	err := s.repo.SoftDelete(ctx, "550e8400-e29b-41d4-a716-446655440099", userID)
	s.ErrorIs(err, domain.ErrBeneficiaryNotFound)
}

func (s *BeneficiarySuite) TestListByUser_ExcludesDeleted() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440005"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251944444444"))

	b1 := &domain.Beneficiary{UserID: userID, FullName: "Keep", Relationship: domain.BeneficiaryChild}
	b2 := &domain.Beneficiary{UserID: userID, FullName: "Delete", Relationship: domain.BeneficiaryParent}
	s.Require().NoError(s.repo.Create(ctx, b1))
	s.Require().NoError(s.repo.Create(ctx, b2))
	s.Require().NoError(s.repo.SoftDelete(ctx, b2.ID, userID))

	list, err := s.repo.ListByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 1)
	s.Equal("Keep", list[0].FullName)
}

func TestBeneficiarySuite(t *testing.T) {
	suite.Run(t, new(BeneficiarySuite))
}
