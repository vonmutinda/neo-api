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

type PotSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.PotRepository
}

func (s *PotSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewPotRepository(s.pool)
}

func (s *PotSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *PotSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440001"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	pot := &domain.Pot{
		UserID:       userID,
		Name:         "Emergency Fund",
		CurrencyCode: "ETB",
	}
	err := s.repo.Create(ctx, pot)
	s.Require().NoError(err)
	s.NotEmpty(pot.ID)
	s.NotEmpty(pot.CreatedAt)
	s.NotEmpty(pot.UpdatedAt)

	got, err := s.repo.GetByID(ctx, pot.ID)
	s.Require().NoError(err)
	s.Equal(pot.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal("Emergency Fund", got.Name)
	s.Equal("ETB", got.CurrencyCode)
}

func (s *PotSuite) TestUpdate_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440002"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	pot := &domain.Pot{UserID: userID, Name: "Vacation", CurrencyCode: "ETB"}
	s.Require().NoError(s.repo.Create(ctx, pot))

	pot.Name = "Vacation 2025"
	err := s.repo.Update(ctx, pot)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, pot.ID)
	s.Require().NoError(err)
	s.Equal("Vacation 2025", got.Name)
}

func (s *PotSuite) TestUpdate_NotFound() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440003"
	pot := &domain.Pot{
		ID:           "550e8400-e29b-41d4-a716-446655440099",
		UserID:       userID,
		Name:         "Ghost",
		CurrencyCode: "ETB",
	}
	err := s.repo.Update(ctx, pot)
	s.ErrorIs(err, domain.ErrPotNotFound)
}

func (s *PotSuite) TestArchive_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440004"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251922222222"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	pot := &domain.Pot{UserID: userID, Name: "Old Pot", CurrencyCode: "ETB"}
	s.Require().NoError(s.repo.Create(ctx, pot))

	err := s.repo.Archive(ctx, pot.ID, userID)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, pot.ID)
	s.Require().NoError(err)
	s.True(got.IsArchived)
}

func (s *PotSuite) TestArchive_AlreadyArchived() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440005"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251933333333"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	pot := &domain.Pot{UserID: userID, Name: "To Archive", CurrencyCode: "ETB"}
	s.Require().NoError(s.repo.Create(ctx, pot))
	s.Require().NoError(s.repo.Archive(ctx, pot.ID, userID))

	err := s.repo.Archive(ctx, pot.ID, userID)
	s.ErrorIs(err, domain.ErrPotNotFound)
}

func (s *PotSuite) TestListActiveByUser() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440006"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251944444444"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	pot1 := &domain.Pot{UserID: userID, Name: "Active Pot", CurrencyCode: "ETB"}
	pot2 := &domain.Pot{UserID: userID, Name: "Archived Pot", CurrencyCode: "ETB"}
	s.Require().NoError(s.repo.Create(ctx, pot1))
	s.Require().NoError(s.repo.Create(ctx, pot2))
	s.Require().NoError(s.repo.Archive(ctx, pot2.ID, userID))

	list, err := s.repo.ListActiveByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 1)
	s.Equal("Active Pot", list[0].Name)
}

func TestPotSuite(t *testing.T) {
	suite.Run(t, new(PotSuite))
}
