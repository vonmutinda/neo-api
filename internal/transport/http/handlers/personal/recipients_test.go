package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/recipient"
	"github.com/vonmutinda/neo/internal/testutil"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/phone"
)

type RecipientHandlerSuite struct {
	suite.Suite
	pool          *pgxpool.Pool
	server        *httptest.Server
	recipientRepo repository.RecipientRepository
	ownerID       string
}

func (s *RecipientHandlerSuite) SetupSuite() {
	t := s.T()
	s.pool = testutil.MustStartPostgres(t)

	userRepo := repository.NewUserRepository(s.pool)
	s.recipientRepo = repository.NewRecipientRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)

	recipientSvc := recipient.NewService(s.recipientRepo, userRepo, auditRepo)

	handler := persh.NewHandlers(
		nil,
		nil, nil, nil, nil, nil,
		userRepo, nil, nil,
		nil, nil, nil, nil, nil,
		recipientSvc,
		nil,
		nil, nil,
	)

	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(testutil.TestJWTConfig()))
		r.Get("/recipients", handler.Recipients.List)
		r.Get("/recipients/search/bank", handler.Recipients.SearchByBank)
		r.Get("/recipients/search/name", handler.Recipients.SearchByName)
		r.Get("/recipients/{id}", handler.Recipients.Get)
		r.Patch("/recipients/{id}/favorite", handler.Recipients.SetFavorite)
		r.Delete("/recipients/{id}", handler.Recipients.Archive)
		r.Get("/banks", handler.Recipients.ListBanks)
	})

	s.server = httptest.NewServer(r)
	t.Cleanup(s.server.Close)
}

func (s *RecipientHandlerSuite) SetupTest() {
	s.ownerID = uuid.NewString()
}

func (s *RecipientHandlerSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *RecipientHandlerSuite) token() string {
	return testutil.MustCreateToken(s.T(), s.ownerID)
}

// --- GET /v1/recipients ---

func (s *RecipientHandlerSuite) TestList_Empty() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var result struct {
		Recipients []domain.Recipient `json:"recipients"`
		Total      int                `json:"total"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &result)
	s.Empty(result.Recipients)
	s.Equal(0, result.Total)
}

func (s *RecipientHandlerSuite) TestList_WithRecipients() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))
	testutil.SeedRecipient(s.T(), s.pool, s.ownerID, targetID, "Abebe Kebede")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var result struct {
		Recipients []domain.Recipient `json:"recipients"`
		Total      int                `json:"total"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &result)
	s.Len(result.Recipients, 1)
	s.Equal(1, result.Total)
	s.Equal("Abebe Kebede", result.Recipients[0].DisplayName)
}

// --- GET /v1/recipients/{id} ---

func (s *RecipientHandlerSuite) TestGet_Success() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))
	recID := testutil.SeedRecipient(s.T(), s.pool, s.ownerID, targetID, "Abebe")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients/"+recID, nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var got domain.Recipient
	testutil.MustDecodeJSON(s.T(), resp, &got)
	s.Equal("Abebe", got.DisplayName)
}

func (s *RecipientHandlerSuite) TestGet_NotFound() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients/"+uuid.NewString(), nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNotFound, resp.StatusCode)
}

// --- PATCH /v1/recipients/{id}/favorite ---

func (s *RecipientHandlerSuite) TestSetFavorite_Success() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))
	recID := testutil.SeedRecipient(s.T(), s.pool, s.ownerID, targetID, "Abebe")

	body := map[string]bool{"isFavorite": true}
	req := testutil.NewAuthRequest(s.T(), "PATCH", s.server.URL+"/v1/recipients/"+recID+"/favorite", body, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	got, err := s.recipientRepo.GetByID(context.Background(), recID, s.ownerID)
	s.Require().NoError(err)
	s.True(got.IsFavorite)
}

// --- DELETE /v1/recipients/{id} ---

func (s *RecipientHandlerSuite) TestArchive_Success() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))
	recID := testutil.SeedRecipient(s.T(), s.pool, s.ownerID, targetID, "Abebe")

	req := testutil.NewAuthRequest(s.T(), "DELETE", s.server.URL+"/v1/recipients/"+recID, nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	got, err := s.recipientRepo.GetByID(context.Background(), recID, s.ownerID)
	s.Require().NoError(err)
	s.Equal(domain.RecipientArchived, got.Status)
}

func (s *RecipientHandlerSuite) TestArchive_NotFound() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))

	req := testutil.NewAuthRequest(s.T(), "DELETE", s.server.URL+"/v1/recipients/"+uuid.NewString(), nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNotFound, resp.StatusCode)
}

// --- GET /v1/recipients/search/bank ---

func (s *RecipientHandlerSuite) TestSearchByBank_Success() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))
	testutil.SeedBankRecipient(s.T(), s.pool, s.ownerID, "CBE 6789", "CBE", "1000123456789")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients/search/bank?institution=CBE&account=10001234", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var results []domain.Recipient
	testutil.MustDecodeJSON(s.T(), resp, &results)
	s.Len(results, 1)
}

func (s *RecipientHandlerSuite) TestSearchByBank_MissingParams() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients/search/bank?institution=CBE&account=12", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// --- GET /v1/recipients/search/name ---

func (s *RecipientHandlerSuite) TestSearchByName_Success() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))
	targetID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, targetID, phone.MustParse("+251911111111"))
	testutil.SeedRecipient(s.T(), s.pool, s.ownerID, targetID, "Abebe Kebede")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients/search/name?q=Abe", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var results []domain.Recipient
	testutil.MustDecodeJSON(s.T(), resp, &results)
	s.Len(results, 1)
	s.Equal("Abebe Kebede", results[0].DisplayName)
}

func (s *RecipientHandlerSuite) TestSearchByName_MissingQuery() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/recipients/search/name", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// --- GET /v1/banks ---

func (s *RecipientHandlerSuite) TestListBanks() {
	testutil.SeedUser(s.T(), s.pool, s.ownerID, phone.MustParse("+251912345678"))

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/banks", nil, s.token())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var banks []domain.BankInfo
	testutil.MustDecodeJSON(s.T(), resp, &banks)
	s.Equal(len(domain.EthiopianBanks), len(banks))

	for i := 1; i < len(banks); i++ {
		s.LessOrEqual(banks[i-1].Name, banks[i].Name, "banks should be sorted by name")
	}
}

func TestRecipientHandlerSuite(t *testing.T) {
	suite.Run(t, new(RecipientHandlerSuite))
}
