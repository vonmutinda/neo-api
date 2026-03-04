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

type CardAuthorizationSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.CardAuthorizationRepository
	cardRepo repository.CardRepository
}

func (s *CardAuthorizationSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewCardAuthorizationRepository(s.pool)
	s.cardRepo = repository.NewCardRepository(s.pool)
}

func (s *CardAuthorizationSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *CardAuthorizationSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	card := &domain.Card{
		UserID:            userID,
		TokenizedPAN:      "tok_" + uuid.NewString(),
		LastFour:          "4242",
		ExpiryMonth:       12,
		ExpiryYear:        2028,
		Type:              domain.CardTypeVirtual,
		Status:            domain.CardStatusActive,
		AllowOnline:       true,
		AllowContactless:  true,
		AllowATM:          true,
		AllowInternational: true,
		DailyLimitCents:   500000,
		MonthlyLimitCents: 5000000,
		PerTxnLimitCents:  500000,
	}
	s.Require().NoError(s.cardRepo.Create(ctx, card))

	auth := &domain.CardAuthorization{
		CardID:                   card.ID,
		RetrievalReferenceNumber: "RRN" + uuid.NewString()[:8],
		STAN:                     "000001",
		AuthAmountCents:          10000,
		Currency:                 "ETB",
		Status:                   domain.AuthApproved,
		AuthorizedAt:             time.Now(),
		ExpiresAt:                time.Now().Add(7 * 24 * time.Hour),
	}
	err := s.repo.Create(ctx, auth)
	s.Require().NoError(err)
	s.NotEmpty(auth.ID)
}

func (s *CardAuthorizationSuite) TestGetByID() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	card := &domain.Card{
		UserID:            userID,
		TokenizedPAN:      "tok_" + uuid.NewString(),
		LastFour:          "1234",
		ExpiryMonth:       6,
		ExpiryYear:        2027,
		Type:              domain.CardTypeVirtual,
		Status:            domain.CardStatusActive,
		AllowOnline:       true,
		AllowContactless:  false,
		AllowATM:          true,
		AllowInternational: false,
		DailyLimitCents:   100000,
		MonthlyLimitCents: 1000000,
		PerTxnLimitCents:  100000,
	}
	s.Require().NoError(s.cardRepo.Create(ctx, card))

	auth := &domain.CardAuthorization{
		CardID:                   card.ID,
		RetrievalReferenceNumber: "RRN" + uuid.NewString()[:8],
		STAN:                     "000002",
		AuthAmountCents:          5000,
		Currency:                 "ETB",
		Status:                   domain.AuthApproved,
		AuthorizedAt:             time.Now(),
		ExpiresAt:                time.Now().Add(7 * 24 * time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, auth))

	got, err := s.repo.GetByID(ctx, auth.ID)
	s.Require().NoError(err)
	s.Equal(auth.ID, got.ID)
	s.Equal(card.ID, got.CardID)
	s.Equal(int64(5000), got.AuthAmountCents)
}

func TestCardAuthorizationSuite(t *testing.T) {
	suite.Run(t, new(CardAuthorizationSuite))
}
