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

type RemittanceTransferSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.RemittanceTransferRepository
}

func (s *RemittanceTransferSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewRemittanceTransferRepository(s.pool)
}

func (s *RemittanceTransferSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *RemittanceTransferSuite) seedProvider(id string) {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO remittance_providers (id, name, api_base_url) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		id, "Test Provider", "https://api.test.com",
	)
	s.Require().NoError(err)
}

func (s *RemittanceTransferSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	providerID := "provider-" + uuid.NewString()[:8]
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))
	s.seedProvider(providerID)

	rt := &domain.RemittanceTransfer{
		UserID:             userID,
		ProviderID:         providerID,
		ProviderTransferID: "",
		QuoteID:            "quote-" + uuid.NewString(),
		SourceCurrency:     "ETB",
		TargetCurrency:     "USD",
		SourceAmountCents:  100000,
		TargetAmountCents:  1700,
		ExchangeRate:       58.82,
		OurFeeCents:        500,
		ProviderFeeCents:   300,
		TotalFeeCents:      800,
		Status:             domain.RemittanceStatusPending,
		RecipientName:      "John Doe",
		RecipientCountry:   "US",
	}
	err := s.repo.Create(ctx, rt)
	s.Require().NoError(err)
	s.NotEmpty(rt.ID)
}

func (s *RemittanceTransferSuite) TestGetByID() {
	ctx := context.Background()
	userID := uuid.NewString()
	providerID := "provider-" + uuid.NewString()[:8]
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))
	s.seedProvider(providerID)

	rt := &domain.RemittanceTransfer{
		UserID:             userID,
		ProviderID:         providerID,
		ProviderTransferID: "",
		QuoteID:            "quote-" + uuid.NewString(),
		SourceCurrency:     "ETB",
		TargetCurrency:     "USD",
		SourceAmountCents:  50000,
		TargetAmountCents:  850,
		ExchangeRate:       58.82,
		OurFeeCents:        250,
		ProviderFeeCents:   150,
		TotalFeeCents:     400,
		Status:             domain.RemittanceStatusPending,
		RecipientName:      "Jane Doe",
		RecipientCountry:   "UK",
	}
	s.Require().NoError(s.repo.Create(ctx, rt))

	got, err := s.repo.GetByID(ctx, rt.ID)
	s.Require().NoError(err)
	s.Equal(rt.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal(int64(50000), got.SourceAmountCents)
}

func TestRemittanceTransferSuite(t *testing.T) {
	suite.Run(t, new(RemittanceTransferSuite))
}
