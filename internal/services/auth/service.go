package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/password"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/google/uuid"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour // 30 days
)

// DefaultBalanceCreator creates the default ETB currency balance for a new user.
type DefaultBalanceCreator interface {
	CreateDefaultBalance(ctx context.Context, userID string) (*domain.CurrencyBalance, *domain.AccountDetails, error)
}

type Service struct {
	users          repository.UserRepository
	sessions       repository.SessionRepository
	audit          repository.AuditRepository
	ledger         ledger.Client
	defaultBalance DefaultBalanceCreator
	jwt            *JWTConfig
}

func NewService(
	users repository.UserRepository,
	sessions repository.SessionRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
	defaultBalance DefaultBalanceCreator,
	jwt *JWTConfig,
) *Service {
	return &Service{
		users:          users,
		sessions:       sessions,
		audit:          audit,
		ledger:         ledgerClient,
		defaultBalance: defaultBalance,
		jwt:            jwt,
	}
}

func (s *Service) Register(ctx context.Context, req *RegisterRequest, userAgent, ipAddress string) (*TokenResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if _, err := s.users.GetByPhone(ctx, req.PhoneNumber); err == nil {
		return nil, fmt.Errorf("phone number already registered: %w", domain.ErrConflict)
	}

	if _, err := s.users.GetByUsername(ctx, req.Username); err == nil {
		return nil, domain.ErrUsernameTaken
	}

	hash := password.GeneratePasswordHash(req.Password)

	walletID := uuid.NewString()
	user := &domain.User{
		ID:             uuid.NewString(),
		PhoneNumber:    req.PhoneNumber,
		Username:       &req.Username,
		PasswordHash:   hash,
		KYCLevel:       domain.KYCBasic,
		LedgerWalletID: "wallet:" + walletID,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	if err := s.ledger.CreateWallet(ctx, walletID, req.PhoneNumber.E164()); err != nil {
		return nil, fmt.Errorf("provisioning wallet: %w", err)
	}

	if s.defaultBalance != nil {
		if _, _, err := s.defaultBalance.CreateDefaultBalance(ctx, user.ID); err != nil {
			return nil, fmt.Errorf("creating default balance: %w", err)
		}
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditUserCreated,
		ActorType:    "system",
		ResourceType: "user",
		ResourceID:   user.ID,
	})

	return s.createSession(ctx, user, userAgent, ipAddress)
}

func (s *Service) Login(ctx context.Context, req *LoginRequest, userAgent, ipAddress string) (*TokenResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	user, err := s.resolveUser(ctx, req.Identifier)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if user.PasswordHash == "" {
		return nil, domain.ErrInvalidCredentials
	}

	if err := password.MatchPassword(user.PasswordHash, req.Password); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if user.IsFrozen {
		return nil, domain.ErrUserFrozen
	}

	return s.createSession(ctx, user, userAgent, ipAddress)
}

func (s *Service) Refresh(ctx context.Context, req *RefreshRequest, userAgent, ipAddress string) (*TokenResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	session, err := s.sessions.GetByRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if session.IsRevoked() {
		return nil, domain.ErrSessionRevoked
	}
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}

	_ = s.sessions.Revoke(ctx, session.ID)

	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if user.IsFrozen {
		return nil, domain.ErrUserFrozen
	}

	return s.createSession(ctx, user, userAgent, ipAddress)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.sessions.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil
	}
	return s.sessions.Revoke(ctx, session.ID)
}

func (s *Service) ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.PasswordHash == "" {
		return domain.ErrInvalidCredentials
	}

	if err := password.MatchPassword(user.PasswordHash, req.CurrentPassword); err != nil {
		return domain.ErrInvalidCredentials
	}

	hash := password.GeneratePasswordHash(req.NewPassword)

	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}

	_ = s.sessions.RevokeAllForUser(ctx, userID)

	return nil
}

// resolveUser looks up a user by phone (E.164) or username.
func (s *Service) resolveUser(ctx context.Context, identifier string) (*domain.User, error) {
	if p, err := phone.Parse(identifier); err == nil {
		return s.users.GetByPhone(ctx, p)
	}
	user, err := s.users.GetByUsername(ctx, identifier)
	if err == nil {
		return user, nil
	}
	return nil, domain.ErrUserNotFound
}

func (s *Service) createSession(ctx context.Context, user *domain.User, userAgent, ipAddress string) (*TokenResponse, error) {
	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	var ua, ip *string
	if userAgent != "" {
		ua = &userAgent
	}
	if ipAddress != "" {
		ip = &ipAddress
	}

	session := &domain.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    ua,
		IPAddress:    ip,
		ExpiresAt:    time.Now().Add(refreshTokenTTL),
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	accessToken, err := s.jwt.CreateToken(user.ID, session.ID, accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(accessTokenTTL).Format(time.RFC3339),
		User:         userInfoFromDomain(user),
	}, nil
}

func userInfoFromDomain(u *domain.User) *UserInfo {
	return &UserInfo{
		ID:          u.ID,
		PhoneNumber: u.PhoneNumber,
		Username:    u.Username,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		KYCLevel:    int(u.KYCLevel),
	}
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
