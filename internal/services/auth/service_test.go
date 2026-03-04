package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/auth"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/password"
	"github.com/vonmutinda/neo/pkg/phone"
)

var testJWT = auth.NewJWTConfig("test-secret-key-for-unit-tests")

type AuthServiceSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	users      repository.UserRepository
	sessions   repository.SessionRepository
	mockLedger *testutil.MockLedgerClient
	svc        *auth.Service
}

func (s *AuthServiceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.users = repository.NewUserRepository(s.pool)
	s.sessions = repository.NewSessionRepository(s.pool)
	audit := repository.NewAuditRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()

	s.svc = auth.NewService(s.users, s.sessions, audit, s.mockLedger, nil, testJWT)
}

func (s *AuthServiceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.mockLedger.Balances = make(map[string]int64)
}

// seedUserWithPassword inserts a user via SeedUser then sets the password hash.
func (s *AuthServiceSuite) seedUserWithPassword(id string, p phone.PhoneNumber, pw string) {
	s.T().Helper()
	testutil.SeedUser(s.T(), s.pool, id, p)
	hash := password.GeneratePasswordHash(pw)
	_, err := s.pool.Exec(context.Background(),
		`UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
	s.Require().NoError(err)
}

// seedUserWithUsernameAndPassword inserts a user, sets username and password hash.
func (s *AuthServiceSuite) seedUserWithUsernameAndPassword(id string, p phone.PhoneNumber, username, pw string) {
	s.T().Helper()
	testutil.SeedUser(s.T(), s.pool, id, p)
	hash := password.GeneratePasswordHash(pw)
	_, err := s.pool.Exec(context.Background(),
		`UPDATE users SET username = $2, password_hash = $3 WHERE id = $1`, id, username, hash)
	s.Require().NoError(err)
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func (s *AuthServiceSuite) TestRegister_Success() {
	ctx := context.Background()

	resp, err := s.svc.Register(ctx, &auth.RegisterRequest{
		PhoneNumber: phone.MustParse("+251911223344"),
		Username:    "testuser",
		Password:    "securepassword123",
	}, "TestAgent", "127.0.0.1")

	s.Require().NoError(err)
	s.NotEmpty(resp.AccessToken, "expected non-empty access token")
	s.NotEmpty(resp.RefreshToken, "expected non-empty refresh token")
	s.Require().NotNil(resp.User, "expected user info in response")
	s.Require().NotNil(resp.User.Username)
	s.Equal("testuser", *resp.User.Username)

	var userCount int
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&userCount)
	s.Require().NoError(err)
	s.Equal(1, userCount, "expected 1 user")

	var sessionCount int
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessionCount)
	s.Require().NoError(err)
	s.Equal(1, sessionCount, "expected 1 session")
}

func (s *AuthServiceSuite) TestRegister_DuplicatePhone() {
	ctx := context.Background()

	testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911223344"))

	_, err := s.svc.Register(ctx, &auth.RegisterRequest{
		PhoneNumber: phone.MustParse("+251911223344"),
		Username:    "newuser",
		Password:    "securepassword123",
	}, "", "")

	s.Error(err, "expected error for duplicate phone")
}

func (s *AuthServiceSuite) TestRegister_DuplicateUsername() {
	ctx := context.Background()

	id := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, id, phone.MustParse("+251922334455"))
	_, err := s.pool.Exec(ctx, `UPDATE users SET username = $2 WHERE id = $1`, id, "taken")
	s.Require().NoError(err)

	_, err = s.svc.Register(ctx, &auth.RegisterRequest{
		PhoneNumber: phone.MustParse("+251911223344"),
		Username:    "taken",
		Password:    "securepassword123",
	}, "", "")

	s.Error(err, "expected error for duplicate username")
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func (s *AuthServiceSuite) TestLogin_ByPhone() {
	ctx := context.Background()

	s.seedUserWithUsernameAndPassword(uuid.NewString(), phone.MustParse("+251911223344"), "loginuser", "mypassword")

	resp, err := s.svc.Login(ctx, &auth.LoginRequest{
		Identifier: "+251911223344",
		Password:   "mypassword",
	}, "TestAgent", "127.0.0.1")

	s.Require().NoError(err)
	s.NotEmpty(resp.AccessToken, "expected non-empty access token")
}

func (s *AuthServiceSuite) TestLogin_ByUsername() {
	ctx := context.Background()

	userID := uuid.NewString()
	s.seedUserWithUsernameAndPassword(userID, phone.MustParse("+251911223344"), "loginuser", "mypassword")

	resp, err := s.svc.Login(ctx, &auth.LoginRequest{
		Identifier: "loginuser",
		Password:   "mypassword",
	}, "", "")

	s.Require().NoError(err)
	s.Equal(userID, resp.User.ID)
}

func (s *AuthServiceSuite) TestLogin_WrongPassword() {
	ctx := context.Background()

	s.seedUserWithPassword(uuid.NewString(), phone.MustParse("+251911223344"), "correctpassword")

	_, err := s.svc.Login(ctx, &auth.LoginRequest{
		Identifier: "+251911223344",
		Password:   "wrongpassword",
	}, "", "")

	s.Error(err, "expected error for wrong password")
}

func (s *AuthServiceSuite) TestLogin_FrozenUser() {
	ctx := context.Background()

	id := uuid.NewString()
	s.seedUserWithPassword(id, phone.MustParse("+251911223344"), "mypassword")
	_, err := s.pool.Exec(ctx, `UPDATE users SET is_frozen = true, frozen_reason = 'test' WHERE id = $1`, id)
	s.Require().NoError(err)

	_, err = s.svc.Login(ctx, &auth.LoginRequest{
		Identifier: "+251911223344",
		Password:   "mypassword",
	}, "", "")

	s.Error(err, "expected error for frozen user")
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

func (s *AuthServiceSuite) TestRefresh_Success() {
	ctx := context.Background()

	resp, err := s.svc.Register(ctx, &auth.RegisterRequest{
		PhoneNumber: phone.MustParse("+251911223344"),
		Username:    "refreshuser",
		Password:    "securepassword123",
	}, "", "")
	s.Require().NoError(err)

	newResp, err := s.svc.Refresh(ctx, &auth.RefreshRequest{
		RefreshToken: resp.RefreshToken,
	}, "NewAgent", "192.168.1.1")

	s.Require().NoError(err)
	s.NotEmpty(newResp.AccessToken, "expected new access token")
	s.NotEqual(resp.RefreshToken, newResp.RefreshToken, "expected rotated refresh token")
}

func (s *AuthServiceSuite) TestRefresh_InvalidToken() {
	ctx := context.Background()

	_, err := s.svc.Refresh(ctx, &auth.RefreshRequest{
		RefreshToken: "nonexistent-token",
	}, "", "")

	s.Error(err, "expected error for invalid refresh token")
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

func (s *AuthServiceSuite) TestLogout_Success() {
	ctx := context.Background()

	resp, err := s.svc.Register(ctx, &auth.RegisterRequest{
		PhoneNumber: phone.MustParse("+251911223344"),
		Username:    "logoutuser",
		Password:    "securepassword123",
	}, "", "")
	s.Require().NoError(err)

	err = s.svc.Logout(ctx, resp.RefreshToken)
	s.Require().NoError(err)

	var revokedAt *string
	err = s.pool.QueryRow(ctx,
		`SELECT revoked_at::text FROM sessions WHERE refresh_token = $1`, resp.RefreshToken,
	).Scan(&revokedAt)
	s.Require().NoError(err)
	s.NotNil(revokedAt, "expected session to be revoked")
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

func (s *AuthServiceSuite) TestChangePassword_Success() {
	ctx := context.Background()

	userID := uuid.NewString()
	s.seedUserWithPassword(userID, phone.MustParse("+251911223344"), "oldpassword")

	err := s.svc.ChangePassword(ctx, userID, &auth.ChangePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword123",
	})
	s.Require().NoError(err)

	var hash string
	err = s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	s.Require().NoError(err)
	s.NoError(password.MatchPassword(hash, "newpassword123"), "new password should be valid")
}

func (s *AuthServiceSuite) TestChangePassword_WrongCurrent() {
	ctx := context.Background()

	userID := uuid.NewString()
	s.seedUserWithPassword(userID, phone.MustParse("+251911223344"), "correctpassword")

	err := s.svc.ChangePassword(ctx, userID, &auth.ChangePasswordRequest{
		CurrentPassword: "wrongpassword",
		NewPassword:     "newpassword123",
	})

	s.Error(err, "expected error for wrong current password")
}

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

func TestAuthServiceSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceSuite))
}
