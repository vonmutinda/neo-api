package admin_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	admin "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
)

const testStaffID = "22222222-2222-2222-2222-222222222222"

type AdminAuthSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	staffRepo repository.StaffRepository
	auditRepo repository.AuditRepository
	svc       *admin.AuthService
}

func (s *AdminAuthSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.staffRepo = repository.NewStaffRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.svc = admin.NewAuthService(s.staffRepo, s.auditRepo, "test-secret", "neo", "neo-admin")
}

func (s *AdminAuthSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminAuthSuite) TestLogin_Success() {
	testutil.SeedStaff(s.T(), s.pool, testStaffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	resp, err := s.svc.Login(context.Background(), admin.LoginRequest{
		Email:    "admin@neo.com",
		Password: "password123",
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Token)
	s.Equal("admin@neo.com", resp.Staff.Email)
}

func (s *AdminAuthSuite) TestLogin_WrongPassword() {
	testutil.SeedStaff(s.T(), s.pool, testStaffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	_, err := s.svc.Login(context.Background(), admin.LoginRequest{
		Email:    "admin@neo.com",
		Password: "wrong-password",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidCredentials)
}

func (s *AdminAuthSuite) TestLogin_StaffNotFound() {
	_, err := s.svc.Login(context.Background(), admin.LoginRequest{
		Email:    "nobody@neo.com",
		Password: "password123",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidCredentials)
}

func (s *AdminAuthSuite) TestLogin_DeactivatedStaff() {
	testutil.SeedStaff(s.T(), s.pool, testStaffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	_, err := s.pool.Exec(context.Background(),
		`UPDATE staff SET is_active = false WHERE id = $1`, testStaffID)
	s.Require().NoError(err)

	_, err = s.svc.Login(context.Background(), admin.LoginRequest{
		Email:    "admin@neo.com",
		Password: "password123",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrStaffDeactivated)
}

func (s *AdminAuthSuite) TestCreateStaff_Success() {
	creatorID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, creatorID, "creator@neo.com", "password123", domain.RoleSuperAdmin)

	staff, err := s.svc.CreateStaff(context.Background(), creatorID, admin.CreateStaffRequest{
		Email:      "newstaff@neo.com",
		FullName:   "New Staff",
		Role:       domain.RoleSuperAdmin,
		Department: "engineering",
		Password:   "securepass123",
	})
	s.Require().NoError(err)
	s.NotEmpty(staff.ID)

	found, err := s.staffRepo.GetByEmail(context.Background(), "newstaff@neo.com")
	s.Require().NoError(err)
	s.Equal("New Staff", found.FullName)
	s.Equal("engineering", found.Department)
}

func (s *AdminAuthSuite) TestChangePassword_Success() {
	testutil.SeedStaff(s.T(), s.pool, testStaffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.ChangePassword(context.Background(), testStaffID, admin.ChangePasswordRequest{
		CurrentPassword: "password123",
		NewPassword:     "newpassword456",
	})
	s.Require().NoError(err)

	resp, err := s.svc.Login(context.Background(), admin.LoginRequest{
		Email:    "admin@neo.com",
		Password: "newpassword456",
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Token)
}

func (s *AdminAuthSuite) TestChangePassword_WrongCurrent() {
	testutil.SeedStaff(s.T(), s.pool, testStaffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.ChangePassword(context.Background(), testStaffID, admin.ChangePasswordRequest{
		CurrentPassword: "wrong-current",
		NewPassword:     "newpassword456",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidCredentials)
}

func TestAdminAuthSuite(t *testing.T) {
	suite.Run(t, new(AdminAuthSuite))
}
