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

type RecipientRepoSuite struct {
	suite.Suite
	pool *pgxpool.Pool
	repo repository.RecipientRepository
}

func (s *RecipientRepoSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewRecipientRepository(s.pool)
}

func (s *RecipientRepoSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *RecipientRepoSuite) seedOwnerAndTarget() (string, string) {
	ownerID := uuid.NewString()
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))
	return ownerID, targetID
}

// --- Upsert ---

func (s *RecipientRepoSuite) TestUpsert_NeoUser_Insert() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	now := time.Now()
	cc := "251"
	num := "911111111"
	currency := "ETB"

	r := &domain.Recipient{
		OwnerUserID:      ownerID,
		Type:             domain.RecipientNeoUser,
		DisplayName:      "Abebe Kebede",
		NeoUserID:        &targetID,
		CountryCode:      &cc,
		Number:           &num,
		LastUsedAt:       &now,
		LastUsedCurrency: &currency,
	}
	err := s.repo.Upsert(ctx, r)
	s.Require().NoError(err)
	s.NotEmpty(r.ID)
	s.Equal(1, r.TransferCount)
	s.Equal(domain.RecipientActive, r.Status)
}

func (s *RecipientRepoSuite) TestUpsert_NeoUser_UpdateOnConflict() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	now := time.Now()
	cc := "251"
	num := "911111111"
	currency := "ETB"

	r := &domain.Recipient{
		OwnerUserID:      ownerID,
		Type:             domain.RecipientNeoUser,
		DisplayName:      "Abebe Kebede",
		NeoUserID:        &targetID,
		CountryCode:      &cc,
		Number:           &num,
		LastUsedAt:       &now,
		LastUsedCurrency: &currency,
	}
	s.Require().NoError(s.repo.Upsert(ctx, r))
	firstID := r.ID
	s.Equal(1, r.TransferCount)

	r2 := &domain.Recipient{
		OwnerUserID:      ownerID,
		Type:             domain.RecipientNeoUser,
		DisplayName:      "Abebe K.",
		NeoUserID:        &targetID,
		CountryCode:      &cc,
		Number:           &num,
		LastUsedAt:       &now,
		LastUsedCurrency: &currency,
	}
	s.Require().NoError(s.repo.Upsert(ctx, r2))
	s.Equal(firstID, r2.ID, "upsert should return the same ID")
	s.Equal(2, r2.TransferCount, "transfer_count should increment")
	s.Equal("Abebe K.", r2.DisplayName, "display_name should update")
}

func (s *RecipientRepoSuite) TestUpsert_BankAccount_Insert() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	now := time.Now()
	inst := "CBE"
	acct := "1000123456789"
	masked := "****6789"
	bankName := "Commercial Bank of Ethiopia"
	swift := "CBETETAA"
	country := "ET"
	currency := "ETB"

	r := &domain.Recipient{
		OwnerUserID:         ownerID,
		Type:                domain.RecipientBankAccount,
		DisplayName:         "CBE 6789",
		InstitutionCode:     &inst,
		BankName:            &bankName,
		SwiftBIC:            &swift,
		AccountNumber:       &acct,
		AccountNumberMasked: &masked,
		BankCountryCode:     &country,
		LastUsedAt:          &now,
		LastUsedCurrency:    &currency,
	}
	err := s.repo.Upsert(ctx, r)
	s.Require().NoError(err)
	s.NotEmpty(r.ID)
	s.Equal(1, r.TransferCount)
}

// --- GetByID ---

func (s *RecipientRepoSuite) TestGetByID_Success() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	got, err := s.repo.GetByID(ctx, recID, ownerID)
	s.Require().NoError(err)
	s.Equal(recID, got.ID)
	s.Equal("Abebe", got.DisplayName)
}

func (s *RecipientRepoSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	_, err := s.repo.GetByID(ctx, uuid.NewString(), ownerID)
	s.ErrorIs(err, domain.ErrRecipientNotFound)
}

func (s *RecipientRepoSuite) TestGetByID_WrongOwner() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	otherOwner := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, otherOwner, phone.MustParse("+251922222222"))

	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	_, err := s.repo.GetByID(ctx, recID, otherOwner)
	s.ErrorIs(err, domain.ErrRecipientNotFound)
}

// --- ListByOwner ---

func (s *RecipientRepoSuite) TestListByOwner_Pagination() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	phones := []string{"+251910000001", "+251910000002", "+251910000003", "+251910000004", "+251910000005"}
	for i := 0; i < 5; i++ {
		tid := uuid.NewString()
		testutil.SeedUser(s.T(), s.pool, tid, phone.MustParse(phones[i]))
		testutil.SeedRecipient(s.T(), s.pool, ownerID, tid, "User"+string(rune('A'+i)))
	}

	list, total, err := s.repo.ListByOwner(ctx, ownerID, repository.RecipientFilter{Limit: 2, Offset: 0})
	s.Require().NoError(err)
	s.Equal(5, total)
	s.Len(list, 2)

	list2, total2, err := s.repo.ListByOwner(ctx, ownerID, repository.RecipientFilter{Limit: 2, Offset: 2})
	s.Require().NoError(err)
	s.Equal(5, total2)
	s.Len(list2, 2)
}

func (s *RecipientRepoSuite) TestListByOwner_FilterByType() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	tid := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, tid, phone.MustParse("+251911111111"))
	testutil.SeedRecipient(s.T(), s.pool, ownerID, tid, "Neo User")
	testutil.SeedBankRecipient(s.T(), s.pool, ownerID, "CBE Acct", "CBE", "1000123456789")

	neoType := "neo_user"
	list, total, err := s.repo.ListByOwner(ctx, ownerID, repository.RecipientFilter{Type: &neoType, Limit: 10})
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Len(list, 1)
	s.Equal(domain.RecipientNeoUser, list[0].Type)
}

// --- FindNeoUserRecipient ---

func (s *RecipientRepoSuite) TestFindNeoUserRecipient_Success() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	got, err := s.repo.FindNeoUserRecipient(ctx, ownerID, targetID)
	s.Require().NoError(err)
	s.Equal(targetID, *got.NeoUserID)
}

func (s *RecipientRepoSuite) TestFindNeoUserRecipient_NotFound() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	_, err := s.repo.FindNeoUserRecipient(ctx, ownerID, uuid.NewString())
	s.ErrorIs(err, domain.ErrRecipientNotFound)
}

// --- SearchByBankAccount ---

func (s *RecipientRepoSuite) TestSearchByBankAccount() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	testutil.SeedBankRecipient(s.T(), s.pool, ownerID, "CBE 6789", "CBE", "1000123456789")
	testutil.SeedBankRecipient(s.T(), s.pool, ownerID, "CBE 9999", "CBE", "1000199999999")
	testutil.SeedBankRecipient(s.T(), s.pool, ownerID, "Dashen", "DASHEN", "2000123456789")

	results, err := s.repo.SearchByBankAccount(ctx, ownerID, "CBE", "10001234", 5)
	s.Require().NoError(err)
	s.Len(results, 1)
	s.Equal("CBE 6789", results[0].DisplayName)
}

// --- SearchByName ---

func (s *RecipientRepoSuite) TestSearchByName() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	t1 := uuid.NewString()
	t2 := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, t1, phone.MustParse("+251911111111"))
	testutil.SeedUser(s.T(), s.pool, t2, phone.MustParse("+251922222222"))
	testutil.SeedRecipient(s.T(), s.pool, ownerID, t1, "Abebe Kebede")
	testutil.SeedRecipient(s.T(), s.pool, ownerID, t2, "Almaz Tesfaye")

	results, err := s.repo.SearchByName(ctx, ownerID, "Ab", 10)
	s.Require().NoError(err)
	s.Len(results, 1)
	s.Equal("Abebe Kebede", results[0].DisplayName)
}

// --- UpdateFavorite ---

func (s *RecipientRepoSuite) TestUpdateFavorite() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	err := s.repo.UpdateFavorite(ctx, recID, ownerID, true)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, recID, ownerID)
	s.Require().NoError(err)
	s.True(got.IsFavorite)

	err = s.repo.UpdateFavorite(ctx, recID, ownerID, false)
	s.Require().NoError(err)

	got, err = s.repo.GetByID(ctx, recID, ownerID)
	s.Require().NoError(err)
	s.False(got.IsFavorite)
}

func (s *RecipientRepoSuite) TestUpdateFavorite_NotFound() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	err := s.repo.UpdateFavorite(ctx, uuid.NewString(), ownerID, true)
	s.ErrorIs(err, domain.ErrRecipientNotFound)
}

// --- Archive ---

func (s *RecipientRepoSuite) TestArchive_Success() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	err := s.repo.Archive(ctx, recID, ownerID)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, recID, ownerID)
	s.Require().NoError(err)
	s.Equal(domain.RecipientArchived, got.Status)
}

func (s *RecipientRepoSuite) TestArchive_ExcludedFromActiveList() {
	ctx := context.Background()
	ownerID, targetID := s.seedOwnerAndTarget()
	recID := testutil.SeedRecipient(s.T(), s.pool, ownerID, targetID, "Abebe")

	s.Require().NoError(s.repo.Archive(ctx, recID, ownerID))

	list, total, err := s.repo.ListByOwner(ctx, ownerID, repository.RecipientFilter{Limit: 10})
	s.Require().NoError(err)
	s.Equal(0, total)
	s.Empty(list)
}

func (s *RecipientRepoSuite) TestArchive_NotFound() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	err := s.repo.Archive(ctx, uuid.NewString(), ownerID)
	s.ErrorIs(err, domain.ErrRecipientNotFound)
}

func TestRecipientRepoSuite(t *testing.T) {
	suite.Run(t, new(RecipientRepoSuite))
}
