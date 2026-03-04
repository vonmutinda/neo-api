package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
	adminh "github.com/vonmutinda/neo/internal/transport/http/handlers/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AdminHandlerSuite struct {
	suite.Suite
	pool   *pgxpool.Pool
	server *httptest.Server
}

func TestAdminHandlers(t *testing.T) {
	suite.Run(t, new(AdminHandlerSuite))
}

func (s *AdminHandlerSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	staffRepo := repository.NewStaffRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	adminQueryRepo := repository.NewAdminQueryRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)
	kycRepo := repository.NewKYCRepository(s.pool)
	flagRepo := repository.NewFlagRepository(s.pool)
	loanRepo := repository.NewLoanRepository(s.pool)

	authSvc := adminsvc.NewAuthService(staffRepo, auditRepo, "test-secret", "neo", "neo-admin")
	staffSvc := adminsvc.NewStaffService(staffRepo, auditRepo)
	customerSvc := adminsvc.NewCustomerService(adminQueryRepo, userRepo, kycRepo, flagRepo, auditRepo, loanRepo)

	authHandler := adminh.NewAuthHandler(authSvc, staffSvc)
	customerHandler := adminh.NewCustomerHandler(customerSvc)

	r := chi.NewRouter()
	r.Post("/auth/login", authHandler.Login)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AdminAuth(testutil.TestAdminJWTConfig()))
		r.Get("/customers", customerHandler.List)
		r.Get("/customers/{id}", customerHandler.GetProfile)
		r.Post("/customers/{id}/freeze", customerHandler.Freeze)
		r.Post("/customers/{id}/unfreeze", customerHandler.Unfreeze)
		r.Post("/auth/change-password", authHandler.ChangePassword)
		r.Get("/staff", authHandler.ListStaff)
		r.Post("/staff", authHandler.CreateStaff)
	})

	s.server = httptest.NewServer(r)
	s.T().Cleanup(s.server.Close)
}

func (s *AdminHandlerSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

const devStaffID = "33333333-3333-3333-3333-333333333333"

func (s *AdminHandlerSuite) authReq(method, path string, body any) *http.Request {
	return testutil.NewAuthRequest(s.T(), method, s.server.URL+path, body, testutil.MustCreateAdminToken(s.T(), devStaffID, domain.RoleSuperAdmin))
}

// ---------------------------------------------------------------------------
// Auth: Login
// ---------------------------------------------------------------------------

func (s *AdminHandlerSuite) TestLogin_Success() {
	staffID := uuid.NewString()
	testutil.SeedStaff(s.T(), s.pool, staffID, "admin@neo.test", "Str0ngP@ss!", domain.RoleSuperAdmin)

	body, _ := json.Marshal(adminsvc.LoginRequest{
		Email:    "admin@neo.test",
		Password: "Str0ngP@ss!",
	})
	resp, err := http.Post(s.server.URL+"/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	var login adminsvc.LoginResponse
	testutil.MustDecodeJSON(s.T(), resp, &login)
	require.NotEmpty(s.T(), login.Token)
	require.Equal(s.T(), staffID, login.Staff.ID)
}

func (s *AdminHandlerSuite) TestLogin_WrongPassword() {
	staffID := uuid.NewString()
	testutil.SeedStaff(s.T(), s.pool, staffID, "admin2@neo.test", "Str0ngP@ss!", domain.RoleSuperAdmin)

	body, _ := json.Marshal(adminsvc.LoginRequest{
		Email:    "admin2@neo.test",
		Password: "wr0ng-password",
	})
	resp, err := http.Post(s.server.URL+"/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Customers: List
// ---------------------------------------------------------------------------

func (s *AdminHandlerSuite) TestListCustomers() {
	testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911000001"))
	testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911000002"))

	req := s.authReq(http.MethodGet, "/customers", nil)
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	var result domain.PaginatedResult[domain.User]
	testutil.MustDecodeJSON(s.T(), resp, &result)
	require.GreaterOrEqual(s.T(), len(result.Data), 2)
}

// ---------------------------------------------------------------------------
// Customers: GetProfile
// ---------------------------------------------------------------------------

func (s *AdminHandlerSuite) TestGetCustomerProfile() {
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000003"))

	req := s.authReq(http.MethodGet, "/customers/"+userID, nil)
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	var profile adminsvc.CustomerProfile
	testutil.MustDecodeJSON(s.T(), resp, &profile)
	require.Equal(s.T(), userID, profile.User.ID)
}

// ---------------------------------------------------------------------------
// Customers: Freeze
// ---------------------------------------------------------------------------

func (s *AdminHandlerSuite) TestFreezeCustomer() {
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000004"))

	req := s.authReq(http.MethodPost, "/customers/"+userID+"/freeze", map[string]string{
		"reason": "suspicious activity",
	})
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	var frozen bool
	err := s.pool.QueryRow(context.Background(),
		`SELECT is_frozen FROM users WHERE id = $1`, userID,
	).Scan(&frozen)
	require.NoError(s.T(), err)
	require.True(s.T(), frozen)
}

// ---------------------------------------------------------------------------
// Customers: Unfreeze
// ---------------------------------------------------------------------------

func (s *AdminHandlerSuite) TestUnfreezeCustomer() {
	userID := uuid.NewString()
	testutil.SeedFrozenUser(s.T(), s.pool, userID, phone.MustParse("+251911000005"), "compliance hold")

	req := s.authReq(http.MethodPost, "/customers/"+userID+"/unfreeze", nil)
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusOK, resp.StatusCode)

	var frozen bool
	err := s.pool.QueryRow(context.Background(),
		`SELECT is_frozen FROM users WHERE id = $1`, userID,
	).Scan(&frozen)
	require.NoError(s.T(), err)
	require.False(s.T(), frozen)
}

// ---------------------------------------------------------------------------
// Staff: Create
// ---------------------------------------------------------------------------

func (s *AdminHandlerSuite) TestCreateStaff() {
	testutil.SeedStaff(s.T(), s.pool, devStaffID, "dev-admin@neo.com", "pass1234", domain.RoleSuperAdmin)

	req := s.authReq(http.MethodPost, "/staff", adminsvc.CreateStaffRequest{
		Email:      "newstaff@neo.test",
		FullName:   "Jane Doe",
		Role:       domain.RoleCustomerSupport,
		Department: "support",
		Password:   "Secur3P@ss!",
	})
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	require.Equal(s.T(), http.StatusCreated, resp.StatusCode)

	var staff domain.Staff
	testutil.MustDecodeJSON(s.T(), resp, &staff)
	require.Equal(s.T(), "newstaff@neo.test", staff.Email)
	require.Equal(s.T(), domain.RoleCustomerSupport, staff.Role)
	require.True(s.T(), staff.IsActive)
}
