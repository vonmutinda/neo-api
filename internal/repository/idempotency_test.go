package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type IdempotencySuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.IdempotencyRepository
}

func (s *IdempotencySuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewIdempotencyRepository(s.pool)
}

func (s *IdempotencySuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *IdempotencySuite) TestAcquireLock_NewKey() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	key := uuid.NewString()
	rec, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", []byte(`{"amount":100}`))
	s.Require().NoError(err)
	s.Require().NotNil(rec)
	s.Equal(key, rec.Key)
	s.Equal(userID, rec.UserID)
	s.Equal("POST /v1/transfers/outbound", rec.Endpoint)
	s.Equal(domain.IdempotencyStarted, rec.Status)
}

func (s *IdempotencySuite) TestAcquireLock_ExistingCompleted() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345679"))

	key := uuid.NewString()
	payload := []byte(`{"amount":100}`)
	rec1, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", payload)
	s.Require().NoError(err)
	s.Equal(domain.IdempotencyStarted, rec1.Status)

	err = s.repo.MarkCompleted(ctx, key, 200, json.RawMessage(`{"ok":true}`))
	s.Require().NoError(err)

	rec2, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", payload)
	s.Require().NoError(err)
	s.Equal(domain.IdempotencyCompleted, rec2.Status)
	s.Require().NotNil(rec2.ResponseCode)
	s.Equal(200, *rec2.ResponseCode)
	s.Require().NotNil(rec2.ResponseBody)
	var body map[string]any
	s.Require().NoError(json.Unmarshal(rec2.ResponseBody, &body))
	s.True(body["ok"].(bool))
}

func (s *IdempotencySuite) TestAcquireLock_PayloadMismatch() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345680"))

	key := uuid.NewString()
	_, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", []byte(`{"amount":100}`))
	s.Require().NoError(err)

	_, err = s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", []byte(`{"amount":500}`))
	s.ErrorIs(err, domain.ErrIdempotencyPayloadMismatch)
}

func (s *IdempotencySuite) TestMarkCompleted() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345681"))

	key := uuid.NewString()
	payload := []byte(`{"amount":100}`)
	_, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", payload)
	s.Require().NoError(err)

	err = s.repo.MarkCompleted(ctx, key, 200, json.RawMessage(`{"ok":true}`))
	s.Require().NoError(err)

	rec, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", payload)
	s.Require().NoError(err)
	s.True(rec.IsCompleted())
	s.Require().NotNil(rec.ResponseCode)
	s.Equal(200, *rec.ResponseCode)
	s.Require().NotNil(rec.ResponseBody)
	var body map[string]any
	s.Require().NoError(json.Unmarshal(rec.ResponseBody, &body))
	s.True(body["ok"].(bool))
}

func (s *IdempotencySuite) TestMarkFailed() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345682"))

	key := uuid.NewString()
	payload := []byte(`{"amount":100}`)
	_, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", payload)
	s.Require().NoError(err)

	err = s.repo.MarkFailed(ctx, key)
	s.Require().NoError(err)

	rec, err := s.repo.AcquireLock(ctx, key, userID, "POST /v1/transfers/outbound", payload)
	s.Require().NoError(err)
	s.True(rec.IsFailed())
	s.Equal(domain.IdempotencyFailed, rec.Status)
}

func (s *IdempotencySuite) TestPurgeExpired() {
	ctx := context.Background()

	_, err := s.repo.PurgeExpired(ctx, 48*time.Hour)
	s.Require().NoError(err)
}

func TestIdempotencySuite(t *testing.T) {
	suite.Run(t, new(IdempotencySuite))
}
