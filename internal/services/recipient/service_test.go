package recipient_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/recipient"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type RecipientServiceSuite struct {
	suite.Suite
	pool          *pgxpool.Pool
	svc           *recipient.Service
	recipientRepo repository.RecipientRepository
	auditRepo     repository.AuditRepository
}

func (s *RecipientServiceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.recipientRepo = repository.NewRecipientRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.svc = recipient.NewService(s.recipientRepo, userRepo, s.auditRepo)
}

func (s *RecipientServiceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *RecipientServiceSuite) countAuditActions(action domain.AuditAction) int {
	var n int
	err := s.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = $1`, string(action),
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

// --- SaveFromTransfer ---

func (s *RecipientServiceSuite) TestSaveFromTransfer_NeoUser_CreatesRecipient() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	err := s.svc.SaveFromTransfer(ctx, ownerID, recipient.TransferCounterparty{
		Type:        domain.RecipientNeoUser,
		DisplayName: "Abebe Kebede",
		NeoUserID:   targetID,
		CountryCode: "251",
		Number:      "911111111",
	}, "ETB")
	s.Require().NoError(err)

	list, total, err := s.svc.List(ctx, ownerID, repository.RecipientFilter{Limit: 10})
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal("Abebe Kebede", list[0].DisplayName)
	s.Equal(domain.RecipientNeoUser, list[0].Type)
	s.Equal(1, list[0].TransferCount)
	s.NotNil(list[0].LastUsedAt)
	s.NotNil(list[0].LastUsedCurrency)
	s.Equal("ETB", *list[0].LastUsedCurrency)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditRecipientSaved), 1)
}

func (s *RecipientServiceSuite) TestSaveFromTransfer_NeoUser_IncrementsOnRepeat() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	cp := recipient.TransferCounterparty{
		Type:        domain.RecipientNeoUser,
		DisplayName: "Abebe Kebede",
		NeoUserID:   targetID,
		CountryCode: "251",
		Number:      "911111111",
	}
	s.Require().NoError(s.svc.SaveFromTransfer(ctx, ownerID, cp, "ETB"))
	s.Require().NoError(s.svc.SaveFromTransfer(ctx, ownerID, cp, "USD"))

	list, _, err := s.svc.List(ctx, ownerID, repository.RecipientFilter{Limit: 10})
	s.Require().NoError(err)
	s.Len(list, 1)
	s.Equal(2, list[0].TransferCount)
	s.Equal("USD", *list[0].LastUsedCurrency)
}

func (s *RecipientServiceSuite) TestSaveFromTransfer_BankAccount_CreatesRecipient() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	err := s.svc.SaveFromTransfer(ctx, ownerID, recipient.TransferCounterparty{
		Type:            domain.RecipientBankAccount,
		DisplayName:     "CBE Account",
		InstitutionCode: "CBE",
		AccountNumber:   "1000123456789",
	}, "ETB")
	s.Require().NoError(err)

	list, total, err := s.svc.List(ctx, ownerID, repository.RecipientFilter{Limit: 10})
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(domain.RecipientBankAccount, list[0].Type)
	s.NotNil(list[0].BankName)
	s.Equal("Commercial Bank of Ethiopia", *list[0].BankName)
	s.NotNil(list[0].AccountNumberMasked)
	s.Equal("****6789", *list[0].AccountNumberMasked)
}

func (s *RecipientServiceSuite) TestSaveFromTransfer_BankAccount_UnknownInstitution() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	err := s.svc.SaveFromTransfer(ctx, ownerID, recipient.TransferCounterparty{
		Type:            domain.RecipientBankAccount,
		DisplayName:     "Unknown Bank",
		InstitutionCode: "FAKEBANK",
		AccountNumber:   "9999999999",
	}, "ETB")
	s.Error(err)
	s.ErrorIs(err, domain.ErrUnknownInstitution)
}

// --- GetByID ---

func (s *RecipientServiceSuite) TestGetByID_Success() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	got, err := s.svc.GetByID(ctx, ownerID, recID)
	s.Require().NoError(err)
	s.Equal("Abebe", got.DisplayName)
}

func (s *RecipientServiceSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	_, err := s.svc.GetByID(ctx, ownerID, uuid.NewString())
	s.ErrorIs(err, domain.ErrRecipientNotFound)
}

func (s *RecipientServiceSuite) TestGetByID_ArchivedReturnsError() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")
	s.Require().NoError(s.recipientRepo.Archive(ctx, recID, ownerID))

	_, err := s.svc.GetByID(ctx, ownerID, recID)
	s.ErrorIs(err, domain.ErrRecipientArchived)
}

// --- SetFavorite ---

func (s *RecipientServiceSuite) TestSetFavorite_Toggle() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	s.Require().NoError(s.svc.SetFavorite(ctx, ownerID, recID, true))
	got, err := s.svc.GetByID(ctx, ownerID, recID)
	s.Require().NoError(err)
	s.True(got.IsFavorite)

	s.Require().NoError(s.svc.SetFavorite(ctx, ownerID, recID, false))
	got, err = s.svc.GetByID(ctx, ownerID, recID)
	s.Require().NoError(err)
	s.False(got.IsFavorite)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditRecipientFavorited), 2)
}

// --- Archive ---

func (s *RecipientServiceSuite) TestArchive_Success() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	s.Require().NoError(s.svc.Archive(ctx, ownerID, recID))

	_, err := s.svc.GetByID(ctx, ownerID, recID)
	s.ErrorIs(err, domain.ErrRecipientArchived)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditRecipientArchived), 1)
}

// --- Owner Scoping ---

func (s *RecipientServiceSuite) TestOwnerScoping_CannotSeeOtherUsersRecipients() {
	ctx := context.Background()
	ownerA := uuid.NewString()
	ownerB := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerA, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, ownerB, phone.MustParse("+251922222222"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))

	recID := testutil.SeedRecipient(s.T(), s.pool, ownerA, targetID, "Abebe")

	_, err := s.svc.GetByID(ctx, ownerB, recID)
	s.ErrorIs(err, domain.ErrRecipientNotFound)

	list, total, err := s.svc.List(ctx, ownerB, repository.RecipientFilter{Limit: 10})
	s.Require().NoError(err)
	s.Equal(0, total)
	s.Empty(list)
}

// --- Search ---

func (s *RecipientServiceSuite) TestSearchByBankAccount() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	testutil.SeedBankRecipient(s.T(), s.pool, ownerID, "CBE 6789", "CBE", "1000123456789")
	testutil.SeedBankRecipient(s.T(), s.pool, ownerID, "CBE 0000", "CBE", "2000000000000")

	results, err := s.svc.SearchByBankAccount(ctx, ownerID, "CBE", "10001234")
	s.Require().NoError(err)
	s.Len(results, 1)
	s.Equal("CBE 6789", results[0].DisplayName)
}

func (s *RecipientServiceSuite) TestSearchByName() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	t1 := uuid.NewString()
	t2 := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, t1, phone.MustParse("+251911111111"))
	testutil.SeedUser(s.T(), s.pool, t2, phone.MustParse("+251922222222"))

	testutil.SeedRecipient(s.T(), s.pool, ownerID, t1, "Abebe Kebede")
	testutil.SeedRecipient(s.T(), s.pool, ownerID, t2, "Almaz Tesfaye")

	results, err := s.svc.SearchByName(ctx, ownerID, "Alm")
	s.Require().NoError(err)
	s.Len(results, 1)
	s.Equal("Almaz Tesfaye", results[0].DisplayName)
}

func TestRecipientServiceSuite(t *testing.T) {
	suite.Run(t, new(RecipientServiceSuite))
}
