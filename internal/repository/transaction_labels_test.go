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

type TransactionLabelSuite struct {
	suite.Suite
	pool         *pgxpool.Pool
	categoryRepo repository.TransactionCategoryRepository
	labelRepo    repository.TransactionLabelRepository
	bizRepo      repository.BusinessRepository
	receiptRepo  repository.TransactionReceiptRepository
}

func (s *TransactionLabelSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.categoryRepo = repository.NewTransactionCategoryRepository(s.pool)
	s.labelRepo = repository.NewTransactionLabelRepository(s.pool)
	s.bizRepo = repository.NewBusinessRepository(s.pool)
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
}

func (s *TransactionLabelSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *TransactionLabelSuite) seedBusiness(userID, bizID string) *domain.Business {
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911100012"))
	tradeName := "Test Corp Trading"
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
		PhoneNumber: phone.MustParse("+251911100021"),
	}
	s.Require().NoError(s.bizRepo.Create(context.Background(), biz))
	return biz
}

func (s *TransactionLabelSuite) seedReceipt(userID string) *domain.TransactionReceipt {
	rec := &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: "ledger-tx-" + uuid.NewString(),
		Type:                domain.ReceiptP2PSend,
		Status:              domain.ReceiptCompleted,
		AmountCents:         100000,
		Currency:            "ETB",
	}
	s.Require().NoError(s.receiptRepo.Create(context.Background(), rec))
	return rec
}

// --- Category tests ---

func (s *TransactionLabelSuite) TestCreateCategory() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	color := "#FF0000"
	cat := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Office Supplies",
		Color:      &color,
		IsSystem:   false,
	}
	err := s.categoryRepo.Create(ctx, cat)
	s.Require().NoError(err)
	s.NotEmpty(cat.ID)
	s.NotEmpty(cat.CreatedAt)

	got, err := s.categoryRepo.GetByID(ctx, cat.ID)
	s.Require().NoError(err)
	s.Equal(cat.ID, got.ID)
	s.Equal("Office Supplies", got.Name)
}

func (s *TransactionLabelSuite) TestListCategories() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	cat1 := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Category A",
		IsSystem:   false,
	}
	cat2 := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Category B",
		IsSystem:   false,
	}
	s.Require().NoError(s.categoryRepo.Create(ctx, cat1))
	s.Require().NoError(s.categoryRepo.Create(ctx, cat2))

	list, err := s.categoryRepo.ListByBusiness(ctx, bizID)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(list), 2)
	var foundA, foundB bool
	for _, c := range list {
		if c.ID == cat1.ID {
			foundA = true
		}
		if c.ID == cat2.ID {
			foundB = true
		}
	}
	s.True(foundA)
	s.True(foundB)
}

func (s *TransactionLabelSuite) TestUpdateCategory() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	cat := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Original",
		IsSystem:   false,
	}
	s.Require().NoError(s.categoryRepo.Create(ctx, cat))

	cat.Name = "Updated Name"
	err := s.categoryRepo.Update(ctx, cat)
	s.Require().NoError(err)

	got, err := s.categoryRepo.GetByID(ctx, cat.ID)
	s.Require().NoError(err)
	s.Equal("Updated Name", got.Name)
}

func (s *TransactionLabelSuite) TestDeleteCategory() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	cat := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "To Delete",
		IsSystem:   false,
	}
	s.Require().NoError(s.categoryRepo.Create(ctx, cat))

	err := s.categoryRepo.Delete(ctx, cat.ID)
	s.Require().NoError(err)

	_, err = s.categoryRepo.GetByID(ctx, cat.ID)
	s.ErrorIs(err, domain.ErrCategoryNotFound)
}

// --- Label tests ---

func (s *TransactionLabelSuite) TestCreateLabel() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	receipt := s.seedReceipt(userID)

	color := "#00FF00"
	cat := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Label Category",
		Color:      &color,
		IsSystem:   false,
	}
	s.Require().NoError(s.categoryRepo.Create(ctx, cat))

	label := &domain.TransactionLabel{
		TransactionID: receipt.ID,
		CategoryID:    &cat.ID,
		TaggedBy:      userID,
		TaxDeductible: true,
	}
	err := s.labelRepo.Create(ctx, label)
	s.Require().NoError(err)
	s.NotEmpty(label.ID)

	got, err := s.labelRepo.GetByTransactionID(ctx, receipt.ID)
	s.Require().NoError(err)
	s.Equal(label.ID, got.ID)
	s.Equal(receipt.ID, got.TransactionID)
	s.Require().NotNil(got.CategoryID)
	s.Equal(cat.ID, *got.CategoryID)
	s.True(got.TaxDeductible)
}

func (s *TransactionLabelSuite) TestUpdateLabel() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	receipt := s.seedReceipt(userID)

	cat := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Cat",
		IsSystem:   false,
	}
	s.Require().NoError(s.categoryRepo.Create(ctx, cat))

	notes := "Original notes"
	label := &domain.TransactionLabel{
		TransactionID: receipt.ID,
		CategoryID:    &cat.ID,
		Notes:         &notes,
		TaggedBy:      userID,
		TaxDeductible: false,
	}
	s.Require().NoError(s.labelRepo.Create(ctx, label))

	updatedNotes := "Updated notes"
	label.Notes = &updatedNotes
	label.TaxDeductible = true
	err := s.labelRepo.Update(ctx, label)
	s.Require().NoError(err)

	got, err := s.labelRepo.GetByTransactionID(ctx, receipt.ID)
	s.Require().NoError(err)
	s.Require().NotNil(got.Notes)
	s.Equal("Updated notes", *got.Notes)
	s.True(got.TaxDeductible)
}

func (s *TransactionLabelSuite) TestDeleteLabel() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	receipt := s.seedReceipt(userID)

	cat := &domain.TransactionCategory{
		BusinessID: &bizID,
		Name:       "Cat",
		IsSystem:   false,
	}
	s.Require().NoError(s.categoryRepo.Create(ctx, cat))

	label := &domain.TransactionLabel{
		TransactionID: receipt.ID,
		CategoryID:    &cat.ID,
		TaggedBy:      userID,
	}
	s.Require().NoError(s.labelRepo.Create(ctx, label))

	err := s.labelRepo.Delete(ctx, receipt.ID)
	s.Require().NoError(err)

	_, err = s.labelRepo.GetByTransactionID(ctx, receipt.ID)
	s.ErrorIs(err, domain.ErrNotFound)
}

func TestTransactionLabelSuite(t *testing.T) {
	suite.Run(t, new(TransactionLabelSuite))
}
