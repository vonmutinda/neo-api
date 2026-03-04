package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AuditSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.AuditRepository
}

func (s *AuditSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewAuditRepository(s.pool)
}

func (s *AuditSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AuditSuite) TestLog_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440001"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	entry := &domain.AuditEntry{
		Action:       domain.AuditUserCreated,
		ActorType:    "system",
		ResourceType: "user",
		ResourceID:   userID,
	}
	err := s.repo.Log(ctx, entry)
	s.Require().NoError(err)
	s.NotEmpty(entry.ID)
	s.NotEmpty(entry.CreatedAt)
}

func (s *AuditSuite) TestListByResource() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440002"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	for i := 0; i < 3; i++ {
		entry := &domain.AuditEntry{
			Action:       domain.AuditUserCreated,
			ActorType:    "system",
			ResourceType: "user",
			ResourceID:   userID,
		}
		s.Require().NoError(s.repo.Log(ctx, entry))
	}

	list, err := s.repo.ListByResource(ctx, "user", userID, 2)
	s.Require().NoError(err)
	s.Len(list, 2)
	s.Equal("user", list[0].ResourceType)
	s.Equal(userID, list[0].ResourceID)
	s.Equal("user", list[1].ResourceType)
	s.Equal(userID, list[1].ResourceID)
	// Most recent first - list[0] should have CreatedAt >= list[1]
	s.True(list[0].CreatedAt.After(list[1].CreatedAt) || list[0].CreatedAt.Equal(list[1].CreatedAt))
}

func TestAuditSuite(t *testing.T) {
	suite.Run(t, new(AuditSuite))
}
