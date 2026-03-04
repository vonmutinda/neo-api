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

type AdminStaffSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	staffRepo repository.StaffRepository
	auditRepo repository.AuditRepository
	svc       *admin.StaffService
}

func (s *AdminStaffSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.staffRepo = repository.NewStaffRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.svc = admin.NewStaffService(s.staffRepo, s.auditRepo)
}

func (s *AdminStaffSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminStaffSuite) TestList_ReturnsStaff() {
	testutil.SeedStaff(s.T(), s.pool, uuid.New().String(), "staff1@neo.com", "password123", domain.RoleSuperAdmin)
	testutil.SeedStaff(s.T(), s.pool, uuid.New().String(), "staff2@neo.com", "password123", domain.RoleSuperAdmin)

	result, err := s.svc.List(context.Background(), domain.StaffFilter{Limit: 10})
	s.Require().NoError(err)
	s.GreaterOrEqual(int(result.Pagination.Total), 2)
}

func (s *AdminStaffSuite) TestGetByID_Success() {
	staffID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "staff@neo.com", "password123", domain.RoleSuperAdmin)

	staff, err := s.svc.GetByID(context.Background(), staffID)
	s.Require().NoError(err)
	s.Equal("staff@neo.com", staff.Email)
}

func (s *AdminStaffSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID(context.Background(), uuid.New().String())
	s.Require().Error(err)
}

func (s *AdminStaffSuite) TestUpdate_Success() {
	staffID := uuid.New().String()
	actorID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "staff@neo.com", "password123", domain.RoleSuperAdmin)
	testutil.SeedStaff(s.T(), s.pool, actorID, "actor@neo.com", "password123", domain.RoleSuperAdmin)

	newName := "Updated Name"
	updated, err := s.svc.Update(context.Background(), actorID, staffID, admin.UpdateStaffRequest{
		FullName: &newName,
	})
	s.Require().NoError(err)
	s.Equal("Updated Name", updated.FullName)

	found, err := s.staffRepo.GetByID(context.Background(), staffID)
	s.Require().NoError(err)
	s.Equal("Updated Name", found.FullName)
}

func (s *AdminStaffSuite) TestDeactivate_Success() {
	staffID := uuid.New().String()
	actorID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "staff@neo.com", "password123", domain.RoleSuperAdmin)
	testutil.SeedStaff(s.T(), s.pool, actorID, "actor@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.Deactivate(context.Background(), actorID, staffID)
	s.Require().NoError(err)

	var isActive bool
	err = s.pool.QueryRow(context.Background(),
		`SELECT is_active FROM staff WHERE id = $1`, staffID).Scan(&isActive)
	s.Require().NoError(err)
	s.False(isActive)
}

func TestAdminStaffSuite(t *testing.T) {
	suite.Run(t, new(AdminStaffSuite))
}
