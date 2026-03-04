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

type CardSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.CardRepository
}

func (s *CardSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewCardRepository(s.pool)
}

func (s *CardSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *CardSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	card := &domain.Card{
		UserID:             userID,
		TokenizedPAN:       "tok_abc123",
		LastFour:           "4242",
		ExpiryMonth:        12,
		ExpiryYear:         2027,
		Type:               domain.CardTypeVirtual,
		Status:             domain.CardStatusActive,
		AllowOnline:        true,
		AllowContactless:   true,
		AllowATM:           true,
		AllowInternational: false,
		DailyLimitCents:    100000,
		MonthlyLimitCents:  500000,
		PerTxnLimitCents:   50000,
	}
	err := s.repo.Create(ctx, card)
	s.Require().NoError(err)
	s.NotEmpty(card.ID)

	got, err := s.repo.GetByID(ctx, card.ID)
	s.Require().NoError(err)
	s.Equal(card.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal("tok_abc123", got.TokenizedPAN)
	s.Equal("4242", got.LastFour)
	s.Equal(domain.CardTypeVirtual, got.Type)
	s.Equal(domain.CardStatusActive, got.Status)
	s.True(got.AllowOnline)
	s.True(got.AllowContactless)
	s.True(got.AllowATM)
	s.False(got.AllowInternational)
	s.Equal(int64(100000), got.DailyLimitCents)
	s.Equal(int64(500000), got.MonthlyLimitCents)
	s.Equal(int64(50000), got.PerTxnLimitCents)
}

func (s *CardSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrCardNotFound)
}

func (s *CardSuite) TestGetByToken() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345679"))

	card := &domain.Card{
		UserID:             userID,
		TokenizedPAN:       "tok_xyz789",
		LastFour:           "1234",
		ExpiryMonth:        6,
		ExpiryYear:         2026,
		Type:               domain.CardTypeVirtual,
		Status:             domain.CardStatusActive,
		AllowOnline:        true,
		AllowContactless:   true,
		AllowATM:           true,
		AllowInternational: false,
		DailyLimitCents:    100000,
		MonthlyLimitCents:  500000,
		PerTxnLimitCents:   50000,
	}
	s.Require().NoError(s.repo.Create(ctx, card))

	got, err := s.repo.GetByToken(ctx, "tok_xyz789")
	s.Require().NoError(err)
	s.Equal(card.ID, got.ID)
	s.Equal("tok_xyz789", got.TokenizedPAN)
}

func (s *CardSuite) TestListByUserID() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345680"))

	for i, tok := range []string{"tok_card1", "tok_card2"} {
		card := &domain.Card{
			UserID:             userID,
			TokenizedPAN:       tok,
			LastFour:           "4242",
			ExpiryMonth:        12,
			ExpiryYear:         2027,
			Type:               domain.CardTypeVirtual,
			Status:             domain.CardStatusActive,
			AllowOnline:        true,
			AllowContactless:   true,
			AllowATM:           true,
			AllowInternational: false,
			DailyLimitCents:    100000,
			MonthlyLimitCents:  500000,
			PerTxnLimitCents:   50000,
		}
		_ = i
		s.Require().NoError(s.repo.Create(ctx, card))
	}

	list, err := s.repo.ListByUserID(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *CardSuite) TestUpdateStatus() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345681"))

	card := &domain.Card{
		UserID:             userID,
		TokenizedPAN:       "tok_active",
		LastFour:           "4242",
		ExpiryMonth:        12,
		ExpiryYear:         2027,
		Type:               domain.CardTypeVirtual,
		Status:             domain.CardStatusActive,
		AllowOnline:        true,
		AllowContactless:   true,
		AllowATM:           true,
		AllowInternational: false,
		DailyLimitCents:    100000,
		MonthlyLimitCents:  500000,
		PerTxnLimitCents:   50000,
	}
	s.Require().NoError(s.repo.Create(ctx, card))

	err := s.repo.UpdateStatus(ctx, card.ID, domain.CardStatusFrozen)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, card.ID)
	s.Require().NoError(err)
	s.Equal(domain.CardStatusFrozen, got.Status)
}

func (s *CardSuite) TestUpdateStatus_NotFound() {
	ctx := context.Background()
	err := s.repo.UpdateStatus(ctx, uuid.NewString(), domain.CardStatusFrozen)
	s.ErrorIs(err, domain.ErrCardNotFound)
}

func (s *CardSuite) TestUpdateLimits() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345682"))

	card := &domain.Card{
		UserID:             userID,
		TokenizedPAN:       "tok_limits",
		LastFour:           "4242",
		ExpiryMonth:        12,
		ExpiryYear:         2027,
		Type:               domain.CardTypeVirtual,
		Status:             domain.CardStatusActive,
		AllowOnline:        true,
		AllowContactless:   true,
		AllowATM:           true,
		AllowInternational: false,
		DailyLimitCents:    100000,
		MonthlyLimitCents:  500000,
		PerTxnLimitCents:   50000,
	}
	s.Require().NoError(s.repo.Create(ctx, card))

	err := s.repo.UpdateLimits(ctx, card.ID, 50000, 200000, 10000)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, card.ID)
	s.Require().NoError(err)
	s.Equal(int64(50000), got.DailyLimitCents)
	s.Equal(int64(200000), got.MonthlyLimitCents)
	s.Equal(int64(10000), got.PerTxnLimitCents)
}

func (s *CardSuite) TestUpdateToggles() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345683"))

	card := &domain.Card{
		UserID:             userID,
		TokenizedPAN:       "tok_toggles",
		LastFour:           "4242",
		ExpiryMonth:        12,
		ExpiryYear:         2027,
		Type:               domain.CardTypeVirtual,
		Status:             domain.CardStatusActive,
		AllowOnline:        true,
		AllowContactless:   true,
		AllowATM:           true,
		AllowInternational: false,
		DailyLimitCents:    100000,
		MonthlyLimitCents:  500000,
		PerTxnLimitCents:   50000,
	}
	s.Require().NoError(s.repo.Create(ctx, card))

	err := s.repo.UpdateToggles(ctx, card.ID, false, true, true, false)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, card.ID)
	s.Require().NoError(err)
	s.False(got.AllowOnline)
	s.True(got.AllowContactless)
	s.True(got.AllowATM)
	s.False(got.AllowInternational)
}

func TestCardSuite(t *testing.T) {
	suite.Run(t, new(CardSuite))
}
