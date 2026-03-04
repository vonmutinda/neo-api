package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/beneficiary"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const (
	beneficiaryUserID = "550e8400-e29b-41d4-a716-446655440001"
)

type BeneficiarySuite struct {
	suite.Suite
	pool            *pgxpool.Pool
	server          *httptest.Server
	beneficiaryRepo repository.BeneficiaryRepository
}

func (s *BeneficiarySuite) SetupSuite() {
	t := s.T()
	s.pool = testutil.MustStartPostgres(t)

	userRepo := repository.NewUserRepository(s.pool)
	s.beneficiaryRepo = repository.NewBeneficiaryRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)

	beneficiarySvc := beneficiary.NewService(s.beneficiaryRepo, auditRepo)

	handler := persh.NewHandlers(
		nil,
		nil, nil, nil, nil, nil,
		userRepo, nil, nil,
		nil, nil, nil, nil, beneficiarySvc,
		nil,
		nil,
		nil, nil,
	)

	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(testutil.TestJWTConfig()))
		r.Post("/beneficiaries", handler.Beneficiaries.Create)
		r.Get("/beneficiaries", handler.Beneficiaries.List)
		r.Delete("/beneficiaries/{id}", handler.Beneficiaries.Delete)
	})

	s.server = httptest.NewServer(r)
	t.Cleanup(s.server.Close)
}

func (s *BeneficiarySuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BeneficiarySuite) TestCreate_Success() {
	_ = testutil.SeedUser(s.T(), s.pool, beneficiaryUserID, phone.MustParse("+251912345678"))

	body := map[string]string{
		"fullName":     "Tigist",
		"relationship": "spouse",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/beneficiaries", body, testutil.MustCreateToken(s.T(), beneficiaryUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusCreated, resp.StatusCode)

	list, err := s.beneficiaryRepo.ListByUser(context.Background(), beneficiaryUserID)
	s.Require().NoError(err)
	s.Len(list, 1)
}

func (s *BeneficiarySuite) TestList_ReturnsAll() {
	_ = testutil.SeedUser(s.T(), s.pool, beneficiaryUserID, phone.MustParse("+251912345678"))
	_ = testutil.SeedBeneficiary(s.T(), s.pool, beneficiaryUserID, "Tigist", domain.BeneficiarySpouse)
	_ = testutil.SeedBeneficiary(s.T(), s.pool, beneficiaryUserID, "Abebe", domain.BeneficiaryChild)

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/beneficiaries", nil, testutil.MustCreateToken(s.T(), beneficiaryUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var list []map[string]interface{}
	testutil.MustDecodeJSON(s.T(), resp, &list)
	s.Require().Len(list, 2)
}

func (s *BeneficiarySuite) TestDelete_Success() {
	_ = testutil.SeedUser(s.T(), s.pool, beneficiaryUserID, phone.MustParse("+251912345678"))
	benID := testutil.SeedBeneficiary(s.T(), s.pool, beneficiaryUserID, "Tigist", domain.BeneficiarySpouse)

	req := testutil.NewAuthRequest(s.T(), "DELETE", s.server.URL+"/v1/beneficiaries/"+benID, nil, testutil.MustCreateToken(s.T(), beneficiaryUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	_, err := s.beneficiaryRepo.GetByID(context.Background(), benID)
	s.ErrorIs(err, domain.ErrBeneficiaryNotFound)
}

func TestBeneficiarySuite(t *testing.T) {
	suite.Run(t, new(BeneficiarySuite))
}
