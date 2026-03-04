package onboarding

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/gateway/fayda"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/google/uuid"
)

// DefaultBalanceCreator creates the default ETB currency balance for a new user.
// Satisfied by balances.Service.CreateDefaultBalance.
type DefaultBalanceCreator interface {
	CreateDefaultBalance(ctx context.Context, userID string) (*domain.CurrencyBalance, *domain.AccountDetails, error)
}

// Service orchestrates user registration and Fayda eKYC verification.
type Service struct {
	users          repository.UserRepository
	kyc            repository.KYCRepository
	audit          repository.AuditRepository
	faydaClient    fayda.Client
	ledger         ledger.Client
	defaultBalance DefaultBalanceCreator
}

// NewService creates an onboarding service with all dependencies injected.
func NewService(
	users repository.UserRepository,
	kyc repository.KYCRepository,
	audit repository.AuditRepository,
	faydaClient fayda.Client,
	ledgerClient ledger.Client,
	defaultBalance DefaultBalanceCreator,
) *Service {
	return &Service{
		users:          users,
		kyc:            kyc,
		audit:          audit,
		faydaClient:    faydaClient,
		ledger:         ledgerClient,
		defaultBalance: defaultBalance,
	}
}

// RegisterUser creates a new user with a phone number and provisions their
// Formance wallet. The user starts at KYC Level 1 (Basic).
// If the phone number already exists, the existing user is returned (phone-first
// auth treats register and login as the same operation).
func (s *Service) RegisterUser(ctx context.Context, req *RegisterRequest) (*domain.User, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	existing, err := s.users.GetByPhone(ctx, req.PhoneNumber)
	if err == nil {
		return existing, nil
	}
	if err != nil && err != domain.ErrUserNotFound {
		return nil, fmt.Errorf("looking up existing user: %w", err)
	}

	walletID := uuid.NewString()
	user := &domain.User{
		ID:             uuid.NewString(),
		PhoneNumber:    req.PhoneNumber,
		KYCLevel:       domain.KYCBasic,
		LedgerWalletID: "wallet:" + walletID,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	if err := s.ledger.CreateWallet(ctx, walletID, req.PhoneNumber.E164()); err != nil {
		return nil, fmt.Errorf("provisioning formance wallet: %w", err)
	}

	// Create the default ETB currency balance with IBAN/account details.
	if s.defaultBalance != nil {
		if _, _, err := s.defaultBalance.CreateDefaultBalance(ctx, user.ID); err != nil {
			return nil, fmt.Errorf("creating default ETB balance: %w", err)
		}
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditUserCreated,
		ActorType:    "system",
		ResourceType: "user",
		ResourceID:   user.ID,
	})

	return user, nil
}

// RequestOTP triggers Fayda to send an OTP to the user's registered phone.
func (s *Service) RequestOTP(ctx context.Context, userID string, req *KYCOTPRequest) (string, error) {
	if err := req.Validate(); err != nil {
		return "", err
	}

	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return "", fmt.Errorf("looking up user: %w", err)
	}

	txID := uuid.NewString()
	if err := s.faydaClient.RequestOTP(ctx, req.FaydaFIN, txID); err != nil {
		return "", fmt.Errorf("requesting fayda OTP: %w", err)
	}

	verification := &domain.KYCVerification{
		UserID:             userID,
		FaydaFIN:           req.FaydaFIN,
		FaydaTransactionID: txID,
		Status:             domain.KYCStatusPending,
	}
	if err := s.kyc.Create(ctx, verification); err != nil {
		return "", fmt.Errorf("recording kyc verification: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditKYCOTPRequested,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "user",
		ResourceID:   userID,
	})

	return txID, nil
}

// VerifyOTP submits the OTP to Fayda, fetches demographic data, and upgrades KYC.
func (s *Service) VerifyOTP(ctx context.Context, userID string, req *KYCVerifyRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	kycResp, err := s.faydaClient.VerifyAndFetchKYC(ctx, req.FaydaFIN, req.OTP, req.TransactionID)
	if err != nil {
		_ = s.kyc.UpdateStatus(ctx, req.TransactionID, domain.KYCStatusFailed)
		_ = s.audit.Log(ctx, &domain.AuditEntry{
			Action:       domain.AuditKYCFailed,
			ActorType:    "user",
			ActorID:      &userID,
			ResourceType: "user",
			ResourceID:   userID,
		})
		return fmt.Errorf("fayda verification failed: %w", err)
	}

	fullName := kycResp.Identity.FullName
	dob := kycResp.Identity.DOB
	gender := kycResp.Identity.Gender

	if err := s.users.UpdateDemographics(ctx, userID, &fullName, nil, nil, &dob, &gender, nil); err != nil {
		return fmt.Errorf("updating demographics: %w", err)
	}

	if err := s.users.UpdateKYCLevel(ctx, userID, domain.KYCVerified); err != nil {
		return fmt.Errorf("upgrading kyc level: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditKYCVerified,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "user",
		ResourceID:   userID,
	})

	return nil
}
