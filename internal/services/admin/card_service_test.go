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
	"github.com/vonmutinda/neo/pkg/phone"
)

type AdminCardSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	adminRepo repository.AdminQueryRepository
	cardRepo  repository.CardRepository
	auditRepo repository.AuditRepository
	svc       *admin.CardService
}

func (s *AdminCardSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.adminRepo = repository.NewAdminQueryRepository(s.pool)
	s.cardRepo = repository.NewCardRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = admin.NewCardService(s.adminRepo, s.cardRepo, s.auditRepo)
}

func (s *AdminCardSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminCardSuite) TestGetByID_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	cardID := testutil.SeedCard(s.T(), s.pool, user.ID, domain.CardStatusActive)

	card, err := s.svc.GetByID(context.Background(), cardID)
	s.Require().NoError(err)
	s.Equal(cardID, card.ID)
	s.Equal(user.ID, card.UserID)
	s.Equal(domain.CardStatusActive, card.Status)
}

func (s *AdminCardSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID(context.Background(), uuid.NewString())
	s.Require().Error(err)
}

func (s *AdminCardSuite) TestFreeze_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	cardID := testutil.SeedCard(s.T(), s.pool, user.ID, domain.CardStatusActive)
	staffID := uuid.NewString()

	err := s.svc.Freeze(context.Background(), staffID, cardID)
	s.Require().NoError(err)

	var status string
	err = s.pool.QueryRow(context.Background(),
		"SELECT status FROM cards WHERE id=$1", cardID,
	).Scan(&status)
	s.Require().NoError(err)
	s.Equal("frozen", status)
}

func (s *AdminCardSuite) TestUnfreeze_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	cardID := testutil.SeedCard(s.T(), s.pool, user.ID, domain.CardStatusFrozen)
	staffID := uuid.NewString()

	err := s.svc.Unfreeze(context.Background(), staffID, cardID)
	s.Require().NoError(err)

	var status string
	err = s.pool.QueryRow(context.Background(),
		"SELECT status FROM cards WHERE id=$1", cardID,
	).Scan(&status)
	s.Require().NoError(err)
	s.Equal("active", status)
}

func (s *AdminCardSuite) TestCancel_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	cardID := testutil.SeedCard(s.T(), s.pool, user.ID, domain.CardStatusActive)
	staffID := uuid.NewString()

	err := s.svc.Cancel(context.Background(), staffID, cardID, "reported stolen")
	s.Require().NoError(err)

	var status string
	err = s.pool.QueryRow(context.Background(),
		"SELECT status FROM cards WHERE id=$1", cardID,
	).Scan(&status)
	s.Require().NoError(err)
	s.Equal("cancelled", status)
}

func (s *AdminCardSuite) TestUpdateLimits_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	cardID := testutil.SeedCard(s.T(), s.pool, user.ID, domain.CardStatusActive)
	staffID := uuid.NewString()

	err := s.svc.UpdateLimits(context.Background(), staffID, cardID, 200000, 800000, 100000)
	s.Require().NoError(err)

	var daily, monthly, perTxn int64
	err = s.pool.QueryRow(context.Background(),
		"SELECT daily_limit_cents, monthly_limit_cents, per_txn_limit_cents FROM cards WHERE id=$1", cardID,
	).Scan(&daily, &monthly, &perTxn)
	s.Require().NoError(err)
	s.Equal(int64(200000), daily)
	s.Equal(int64(800000), monthly)
	s.Equal(int64(100000), perTxn)
}

func TestAdminCardSuite(t *testing.T) {
	suite.Run(t, new(AdminCardSuite))
}
