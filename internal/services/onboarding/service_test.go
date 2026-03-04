package onboarding_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/onboarding"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type OnboardingSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	userRepo    repository.UserRepository
	kycRepo     repository.KYCRepository
	auditRepo   repository.AuditRepository
	faydaClient *testutil.MockFaydaClient
	svc         *onboarding.Service
}

func (s *OnboardingSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.userRepo = repository.NewUserRepository(s.pool)
	s.kycRepo = repository.NewKYCRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.faydaClient = testutil.NewMockFaydaClient()
	ledgerClient := testutil.NewMockLedgerClient()

	s.svc = onboarding.NewService(
		s.userRepo, s.kycRepo, s.auditRepo,
		s.faydaClient, ledgerClient, nil,
	)
}

func (s *OnboardingSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.faydaClient.ShouldFail = false
}

func TestOnboardingSuite(t *testing.T) {
	suite.Run(t, new(OnboardingSuite))
}

func (s *OnboardingSuite) TestRegisterUser_Success() {
	ctx := context.Background()

	user, err := s.svc.RegisterUser(ctx, &onboarding.RegisterRequest{
		PhoneNumber: phone.MustParse("+251912345678"),
	})
	s.Require().NoError(err)
	s.Equal("+251912345678", user.PhoneNumber.E164())
	s.Equal(domain.KYCBasic, user.KYCLevel)
	s.NotEmpty(user.LedgerWalletID)

	fetched, err := s.userRepo.GetByPhone(ctx, phone.MustParse("+251912345678"))
	s.Require().NoError(err)
	s.Equal(user.ID, fetched.ID)

	var auditCount int
	err = s.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE resource_id = $1 AND action = $2`,
		user.ID, domain.AuditUserCreated,
	).Scan(&auditCount)
	s.Require().NoError(err)
	s.Equal(1, auditCount)
}

func (s *OnboardingSuite) TestRequestOTP_Success() {
	ctx := context.Background()

	user, err := s.svc.RegisterUser(ctx, &onboarding.RegisterRequest{
		PhoneNumber: phone.MustParse("+251912345678"),
	})
	s.Require().NoError(err)

	txID, err := s.svc.RequestOTP(ctx, user.ID, &onboarding.KYCOTPRequest{FaydaFIN: "1234567890AB"})
	s.Require().NoError(err)
	s.NotEmpty(txID)

	var status string
	err = s.pool.QueryRow(ctx,
		`SELECT status FROM kyc_verifications WHERE user_id = $1 AND fayda_transaction_id = $2`,
		user.ID, txID,
	).Scan(&status)
	s.Require().NoError(err)
	s.Equal(string(domain.KYCStatusPending), status)
}

func (s *OnboardingSuite) TestRequestOTP_UserNotFound() {
	ctx := context.Background()

	_, err := s.svc.RequestOTP(ctx, uuid.NewString(), &onboarding.KYCOTPRequest{FaydaFIN: "1234567890AB"})
	s.Error(err)
}

func (s *OnboardingSuite) TestVerifyOTP_Success() {
	ctx := context.Background()

	user, err := s.svc.RegisterUser(ctx, &onboarding.RegisterRequest{
		PhoneNumber: phone.MustParse("+251912345678"),
	})
	s.Require().NoError(err)

	txID, err := s.svc.RequestOTP(ctx, user.ID, &onboarding.KYCOTPRequest{FaydaFIN: "1234567890AB"})
	s.Require().NoError(err)

	err = s.svc.VerifyOTP(ctx, user.ID, &onboarding.KYCVerifyRequest{
		FaydaFIN: "1234567890AB", OTP: "123456", TransactionID: txID,
	})
	s.Require().NoError(err)

	updated, err := s.userRepo.GetByID(ctx, user.ID)
	s.Require().NoError(err)
	s.Equal(domain.KYCVerified, updated.KYCLevel)
}

func (s *OnboardingSuite) TestVerifyOTP_FaydaFailure() {
	ctx := context.Background()

	user, err := s.svc.RegisterUser(ctx, &onboarding.RegisterRequest{
		PhoneNumber: phone.MustParse("+251912345678"),
	})
	s.Require().NoError(err)

	txID, err := s.svc.RequestOTP(ctx, user.ID, &onboarding.KYCOTPRequest{FaydaFIN: "1234567890AB"})
	s.Require().NoError(err)

	s.faydaClient.ShouldFail = true

	err = s.svc.VerifyOTP(ctx, user.ID, &onboarding.KYCVerifyRequest{
		FaydaFIN: "1234567890AB", OTP: "wrong-otp", TransactionID: txID,
	})
	s.Error(err)
}
