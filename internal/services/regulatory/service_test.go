package regulatory_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type RegulatorySuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	svc        *regulatory.Service
	ruleRepo   repository.RegulatoryRuleRepository
	totalsRepo repository.TransferTotalsRepository
	mockCache  *testutil.MockCache
}

func (s *RegulatorySuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.mockCache = testutil.NewMockCache()
	s.ruleRepo = repository.NewRegulatoryRuleRepository(s.pool)
	s.totalsRepo = repository.NewTransferTotalsRepository(s.pool)

	rateFn := func(_ context.Context, from, to string) (float64, error) {
		if from == "ETB" && to == "USD" {
			return 0.019, nil
		}
		return 1.0, nil
	}

	s.svc = regulatory.NewService(s.ruleRepo, s.totalsRepo, rateFn, 60*time.Second, s.mockCache)
}

func (s *RegulatorySuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.svc.InvalidateCache()
}

func (s *RegulatorySuite) seedRule(key string, scope domain.RuleScope, scopeValue, value, valueType string) {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO regulatory_rules (key, scope, scope_value, value, value_type, description, effective_from)
		 VALUES ($1, $2, $3, $4, $5, 'test rule', NOW() - INTERVAL '1 hour')`,
		key, scope, scopeValue, value, valueType,
	)
	s.Require().NoError(err)
}

func (s *RegulatorySuite) TestIsEnabled_GlobalRule() {
	s.seedRule("fx_enabled", domain.RuleScopeGlobal, "", "true", "boolean")

	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	enabled, err := s.svc.IsEnabled(context.Background(), "fx_enabled", user)
	s.Require().NoError(err)
	s.True(enabled)
}

func (s *RegulatorySuite) TestIsEnabled_KYCLevelOverride() {
	s.seedRule("fx_enabled", domain.RuleScopeGlobal, "", "true", "boolean")
	s.seedRule("fx_enabled", domain.RuleScopeKYCLevel, "2", "false", "boolean")

	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))

	enabled, err := s.svc.IsEnabled(context.Background(), "fx_enabled", user)
	s.Require().NoError(err)
	s.False(enabled, "kyc_level scope should override global")
}

func (s *RegulatorySuite) TestGetAmountLimit_Success() {
	s.seedRule("daily_transfer_limit", domain.RuleScopeGlobal, "", "500000", "amount_cents")

	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	limit, err := s.svc.GetAmountLimit(context.Background(), "daily_transfer_limit", user, "ETB")
	s.Require().NoError(err)
	s.Equal(int64(500000), limit)
}

func (s *RegulatorySuite) TestCheckTransferAllowed_Allowed() {
	s.seedRule("daily_transfer_limit", domain.RuleScopeGlobal, "", "10000000", "amount_cents")

	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))

	err := s.svc.CheckTransferAllowed(context.Background(), &regulatory.TransferCheckRequest{
		UserID:      user.ID,
		User:        user,
		Direction:   "p2p",
		AmountCents: 50000,
		Currency:    "ETB",
	})
	s.Require().NoError(err)
}

func (s *RegulatorySuite) TestCheckTransferAllowed_DailyLimitExceeded() {
	userID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	s.seedRule("daily_transfer_limit", domain.RuleScopeGlobal, "", "100000", "amount_cents")

	err := s.totalsRepo.Increment(context.Background(), userID, "ETB", "p2p", 90000)
	s.Require().NoError(err)

	err = s.svc.CheckTransferAllowed(context.Background(), &regulatory.TransferCheckRequest{
		UserID:      user.ID,
		User:        user,
		Direction:   "p2p",
		AmountCents: 20000,
		Currency:    "ETB",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrDailyLimitExceeded)
	s.Contains(err.Error(), "daily")
}

func TestRegulatorySuite(t *testing.T) {
	suite.Run(t, new(RegulatorySuite))
}
