package admin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	admin "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
)

type AdminConfigSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	configRepo repository.SystemConfigRepository
	auditRepo  repository.AuditRepository
	svc        *admin.ConfigService
}

func (s *AdminConfigSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.configRepo = repository.NewSystemConfigRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = admin.NewConfigService(s.configRepo, s.auditRepo)
}

func (s *AdminConfigSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminConfigSuite) TestListAll() {
	testutil.SeedSystemConfig(s.T(), s.pool, "feature_x", []byte(`true`))
	testutil.SeedSystemConfig(s.T(), s.pool, "feature_y", []byte(`false`))

	configs, err := s.svc.ListAll(context.Background())
	s.Require().NoError(err)
	s.GreaterOrEqual(len(configs), 2)
}

func (s *AdminConfigSuite) TestGet_Success() {
	testutil.SeedSystemConfig(s.T(), s.pool, "feature_x", []byte(`true`))

	cfg, err := s.svc.Get(context.Background(), "feature_x")
	s.Require().NoError(err)
	s.Equal("feature_x", cfg.Key)
}

func (s *AdminConfigSuite) TestGet_NotFound() {
	_, err := s.svc.Get(context.Background(), "nonexistent_key_"+uuid.NewString())
	s.Require().Error(err)
}

func (s *AdminConfigSuite) TestIsEnabled_True() {
	testutil.SeedSystemConfig(s.T(), s.pool, "toggle_on", []byte(`true`))

	enabled, err := s.svc.IsEnabled(context.Background(), "toggle_on")
	s.Require().NoError(err)
	s.True(enabled)
}

func (s *AdminConfigSuite) TestIsEnabled_False() {
	testutil.SeedSystemConfig(s.T(), s.pool, "toggle_off", []byte(`false`))

	enabled, err := s.svc.IsEnabled(context.Background(), "toggle_off")
	s.Require().NoError(err)
	s.False(enabled)
}

func (s *AdminConfigSuite) TestUpdate_Success() {
	testutil.SeedSystemConfig(s.T(), s.pool, "limit_key", []byte(`100`))
	staff := testutil.SeedStaff(s.T(), s.pool, uuid.NewString(), "config-admin@neo.com", "pass1234", domain.RoleSuperAdmin)

	newValue := json.RawMessage(`200`)
	err := s.svc.Update(context.Background(), staff.ID, "limit_key", admin.UpdateConfigRequest{
		Value: newValue,
	})
	s.Require().NoError(err)

	var raw []byte
	err = s.pool.QueryRow(context.Background(),
		"SELECT value FROM system_config WHERE key=$1", "limit_key",
	).Scan(&raw)
	s.Require().NoError(err)
	s.JSONEq(`200`, string(raw))
}

func TestAdminConfigSuite(t *testing.T) {
	suite.Run(t, new(AdminConfigSuite))
}
