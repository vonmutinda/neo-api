package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
)

type RegulatoryRuleSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.RegulatoryRuleRepository
}

func (s *RegulatoryRuleSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewRegulatoryRuleRepository(s.pool)
}

func (s *RegulatoryRuleSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *RegulatoryRuleSuite) TestCreate_Success() {
	ctx := context.Background()
	rule := &domain.RegulatoryRule{
		Key:           "daily_transfer_limit",
		Description:   "Max daily transfer in cents",
		ValueType:     domain.RuleTypeAmountCents,
		Value:         "15000000",
		Scope:         domain.RuleScopeGlobal,
		ScopeValue:    "",
		EffectiveFrom: time.Now().Add(-time.Hour),
	}
	err := s.repo.Create(ctx, rule)
	s.Require().NoError(err)
	s.NotEmpty(rule.ID)
}

func (s *RegulatoryRuleSuite) TestGetByID() {
	ctx := context.Background()
	rule := &domain.RegulatoryRule{
		Key:           "get_by_id_key",
		Description:   "Test rule",
		ValueType:     domain.RuleTypeAmountCents,
		Value:         "1000000",
		Scope:         domain.RuleScopeGlobal,
		ScopeValue:    "",
		EffectiveFrom: time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, rule))

	got, err := s.repo.GetByID(ctx, rule.ID)
	s.Require().NoError(err)
	s.Equal(rule.ID, got.ID)
	s.Equal("get_by_id_key", got.Key)
}

func (s *RegulatoryRuleSuite) TestGetEffective() {
	ctx := context.Background()
	rule := &domain.RegulatoryRule{
		Key:           "effective_key",
		Description:   "Effective rule",
		ValueType:     domain.RuleTypeAmountCents,
		Value:         "20000000",
		Scope:         domain.RuleScopeGlobal,
		ScopeValue:    "",
		EffectiveFrom: time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, rule))

	got, err := s.repo.GetEffective(ctx, "effective_key", "global", "")
	s.Require().NoError(err)
	s.Equal(rule.ID, got.ID)
	s.Equal("20000000", got.Value)
}

func (s *RegulatoryRuleSuite) TestListEffectiveByKey() {
	ctx := context.Background()
	key := "multi_scope_key"
	r1 := &domain.RegulatoryRule{
		Key:           key,
		Description:   "Global",
		ValueType:     domain.RuleTypeAmountCents,
		Value:         "10000000",
		Scope:         domain.RuleScopeGlobal,
		ScopeValue:    "",
		EffectiveFrom: time.Now().Add(-time.Hour),
	}
	r2 := &domain.RegulatoryRule{
		Key:           key,
		Description:   "KYC level",
		ValueType:     domain.RuleTypeAmountCents,
		Value:         "5000000",
		Scope:         domain.RuleScopeKYCLevel,
		ScopeValue:    "kyc_1",
		EffectiveFrom: time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, r1))
	s.Require().NoError(s.repo.Create(ctx, r2))

	rules, err := s.repo.ListEffectiveByKey(ctx, key)
	s.Require().NoError(err)
	s.Len(rules, 2)
}

func (s *RegulatoryRuleSuite) TestUpdate() {
	ctx := context.Background()
	rule := &domain.RegulatoryRule{
		Key:           "update_key",
		Description:   "To update",
		ValueType:     domain.RuleTypeAmountCents,
		Value:         "1000",
		Scope:         domain.RuleScopeGlobal,
		ScopeValue:    "",
		EffectiveFrom: time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.repo.Create(ctx, rule))

	rule.Value = "2000"
	err := s.repo.Update(ctx, rule)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, rule.ID)
	s.Require().NoError(err)
	s.Equal("2000", got.Value)
}

func TestRegulatoryRuleSuite(t *testing.T) {
	suite.Run(t, new(RegulatoryRuleSuite))
}
