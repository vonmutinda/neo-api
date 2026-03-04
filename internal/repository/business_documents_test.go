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

type BusinessDocumentSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.BusinessDocumentRepository
}

func (s *BusinessDocumentSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewBusinessDocumentRepository(s.pool)
}

func (s *BusinessDocumentSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BusinessDocumentSuite) seedBusiness(userID, bizID string) {
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))
	tradeName := "Test Corp"
	biz := &domain.Business{
		ID:                 bizID,
		OwnerUserID:        userID,
		Name:               "Test Corp",
		TradeName:          &tradeName,
		TINNumber:          "TIN-" + uuid.NewString()[:8],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:8],
		IndustryCategory:   domain.IndustryRetail,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber: phone.MustParse("+251911100028"),
	}
	bizRepo := repository.NewBusinessRepository(s.pool)
	s.Require().NoError(bizRepo.Create(context.Background(), biz))
}

func (s *BusinessDocumentSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	doc := &domain.BusinessDocument{
		BusinessID:    bizID,
		Name:          "Trade License 2026",
		DocumentType:  domain.DocTradeLicense,
		FileKey:       "docs/" + uuid.NewString() + ".pdf",
		FileSizeBytes: 102400,
		MimeType:      "application/pdf",
		UploadedBy:    userID,
	}
	err := s.repo.Create(ctx, doc)
	s.Require().NoError(err)
	s.NotEmpty(doc.ID)

	got, err := s.repo.GetByID(ctx, doc.ID)
	s.Require().NoError(err)
	s.Equal(doc.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal("Trade License 2026", got.Name)
	s.Equal(domain.DocTradeLicense, got.DocumentType)
	s.Equal(userID, got.UploadedBy)
}

func (s *BusinessDocumentSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrDocumentNotFound)
}

func TestBusinessDocumentSuite(t *testing.T) {
	suite.Run(t, new(BusinessDocumentSuite))
}
