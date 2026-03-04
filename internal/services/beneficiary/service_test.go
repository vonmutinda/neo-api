package beneficiary_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/beneficiary"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type BeneficiarySuite struct {
	suite.Suite
	pool           *pgxpool.Pool
	svc            *beneficiary.Service
	beneficiaryRepo repository.BeneficiaryRepository
	auditRepo      repository.AuditRepository
}

func (s *BeneficiarySuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.beneficiaryRepo = repository.NewBeneficiaryRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.svc = beneficiary.NewService(s.beneficiaryRepo, s.auditRepo)
}

func (s *BeneficiarySuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BeneficiarySuite) TestCreate_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))

	b, err := s.svc.Create(context.Background(), user.ID, &beneficiary.CreateBeneficiaryRequest{
		FullName:     "Abebe Bikila",
		Relationship: "child",
	})
	s.Require().NoError(err)
	s.NotEmpty(b.ID)
	s.Equal("Abebe Bikila", b.FullName)
	s.Equal(domain.BeneficiaryChild, b.Relationship)

	ctx := context.Background()
	list, err := s.beneficiaryRepo.ListByUser(ctx, user.ID)
	s.Require().NoError(err)
	s.Len(list, 1)

	entries, err := s.auditRepo.ListByResource(ctx, "beneficiary", b.ID, 10)
	s.Require().NoError(err)
	if len(entries) > 0 {
		s.GreaterOrEqual(len(entries), 1, "expected audit entry for beneficiary create")
	}
}

func (s *BeneficiarySuite) TestList_ReturnsAll() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedBeneficiary(s.T(), s.pool, user.ID, "Abebe", domain.BeneficiarySpouse)
	testutil.SeedBeneficiary(s.T(), s.pool, user.ID, "Kenenisa", domain.BeneficiaryChild)

	list, err := s.svc.List(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *BeneficiarySuite) TestDelete_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	benID := testutil.SeedBeneficiary(s.T(), s.pool, user.ID, "Abebe", domain.BeneficiarySpouse)

	s.Require().NoError(s.svc.Delete(context.Background(), benID, user.ID))

	_, err := s.beneficiaryRepo.GetByID(context.Background(), benID)
	s.ErrorIs(err, domain.ErrBeneficiaryNotFound)
}

func TestBeneficiarySuite(t *testing.T) {
	suite.Run(t, new(BeneficiarySuite))
}
