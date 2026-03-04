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

type UserSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.UserRepository
}

func (s *UserSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewUserRepository(s.pool)
}

func (s *UserSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *UserSuite) TestCreate_Success() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440001",
		PhoneNumber:     phone.MustParse("+251912345678"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440001",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	err := s.repo.Create(ctx, user)
	s.Require().NoError(err)
	s.NotEmpty(user.CreatedAt)
	s.NotEmpty(user.UpdatedAt)

	got, err := s.repo.GetByID(ctx, user.ID)
	s.Require().NoError(err)
	s.Equal(user.ID, got.ID)
	s.Equal(user.PhoneNumber, got.PhoneNumber)
	s.Equal(user.LedgerWalletID, got.LedgerWalletID)
	s.Equal(user.KYCLevel, got.KYCLevel)
	s.Equal(user.AccountType, got.AccountType)
}

func (s *UserSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, "550e8400-e29b-41d4-a716-446655440099")
	s.ErrorIs(err, domain.ErrUserNotFound)
}

func (s *UserSuite) TestGetByPhone_Success() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440002",
		PhoneNumber:     phone.MustParse("+251911111111"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440002",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))

	got, err := s.repo.GetByPhone(ctx, phone.MustParse("+251911111111"))
	s.Require().NoError(err)
	s.Equal(user.ID, got.ID)
	s.Equal(user.PhoneNumber, got.PhoneNumber)
}

func (s *UserSuite) TestGetByPhone_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByPhone(ctx, phone.MustParse("+251999999999"))
	s.ErrorIs(err, domain.ErrUserNotFound)
}

func (s *UserSuite) TestUpdateKYCLevel() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440003",
		PhoneNumber:     phone.MustParse("+251922222222"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440003",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))

	err := s.repo.UpdateKYCLevel(ctx, user.ID, domain.KYCEnhanced)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, user.ID)
	s.Require().NoError(err)
	s.Equal(domain.KYCEnhanced, got.KYCLevel)
}

func (s *UserSuite) TestUpdateKYCLevel_NotFound() {
	ctx := context.Background()
	err := s.repo.UpdateKYCLevel(ctx, "550e8400-e29b-41d4-a716-446655440099", domain.KYCEnhanced)
	s.ErrorIs(err, domain.ErrUserNotFound)
}

func (s *UserSuite) TestFreeze_Success() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440004",
		PhoneNumber:     phone.MustParse("+251933333333"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440004",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))

	err := s.repo.Freeze(ctx, user.ID, "AML hold")
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, user.ID)
	s.Require().NoError(err)
	s.True(got.IsFrozen)
	s.Require().NotNil(got.FrozenReason)
	s.Equal("AML hold", *got.FrozenReason)
}

func (s *UserSuite) TestUnfreeze_Success() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440005",
		PhoneNumber:     phone.MustParse("+251944444444"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440005",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))
	s.Require().NoError(s.repo.Freeze(ctx, user.ID, "temp hold"))

	err := s.repo.Unfreeze(ctx, user.ID)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, user.ID)
	s.Require().NoError(err)
	s.False(got.IsFrozen)
	s.Nil(got.FrozenReason)
}

func (s *UserSuite) TestBindTelegram() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440006",
		PhoneNumber:     phone.MustParse("+251955555555"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440006",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))

	err := s.repo.BindTelegram(ctx, user.ID, 12345, "testuser")
	s.Require().NoError(err)

	got, err := s.repo.GetByTelegramID(ctx, 12345)
	s.Require().NoError(err)
	s.Equal(user.ID, got.ID)
	s.Require().NotNil(got.TelegramID)
	s.Equal(int64(12345), *got.TelegramID)
	s.Require().NotNil(got.TelegramUsername)
	s.Equal("testuser", *got.TelegramUsername)
}

func (s *UserSuite) TestUnbindTelegram() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440007",
		PhoneNumber:     phone.MustParse("+251966666666"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440007",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))
	s.Require().NoError(s.repo.BindTelegram(ctx, user.ID, 67890, "bounduser"))

	err := s.repo.UnbindTelegram(ctx, user.ID)
	s.Require().NoError(err)

	_, err = s.repo.GetByTelegramID(ctx, 67890)
	s.ErrorIs(err, domain.ErrUserNotFound)
}

func (s *UserSuite) TestUpdateSpendWaterfall() {
	ctx := context.Background()
	user := &domain.User{
		ID:             "550e8400-e29b-41d4-a716-446655440008",
		PhoneNumber:     phone.MustParse("+251977777777"),
		LedgerWalletID:  "wallet:550e8400-e29b-41d4-a716-446655440008",
		KYCLevel:        domain.KYCBasic,
		AccountType:     domain.AccountTypePersonal,
	}
	s.Require().NoError(s.repo.Create(ctx, user))

	waterfall := domain.SpendWaterfall{"USD", "ETB"}
	err := s.repo.UpdateSpendWaterfall(ctx, user.ID, waterfall)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, user.ID)
	s.Require().NoError(err)
	s.Equal(waterfall, got.SpendWaterfallOrder)
}

func TestUserSuite(t *testing.T) {
	suite.Run(t, new(UserSuite))
}
