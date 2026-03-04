package telegram_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	tgclient "github.com/vonmutinda/neo/internal/gateway/telegram"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/telegram"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type TelegramSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	userRepo   repository.UserRepository
	tokenRepo  repository.TelegramLinkTokenRepository
	mockTG     *testutil.MockTelegramClient
	mockLedger *testutil.MockLedgerClient
	svc        *telegram.Service
}

func (s *TelegramSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.userRepo = repository.NewUserRepository(s.pool)
	s.tokenRepo = repository.NewTelegramLinkTokenRepository(s.pool)
	audit := repository.NewAuditRepository(s.pool)
	s.mockTG = testutil.NewMockTelegramClient()
	s.mockLedger = testutil.NewMockLedgerClient()

	s.svc = telegram.NewService(s.userRepo, s.tokenRepo, audit, s.mockTG, s.mockLedger)
}

func (s *TelegramSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.mockLedger.Balances = make(map[string]int64)
	s.mockTG.Messages = nil
}

func (s *TelegramSuite) TestHandleUpdate_DeepLink_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	token, err := s.tokenRepo.Create(ctx, userID, 3600_000_000_000)
	s.Require().NoError(err)

	update := tgclient.Update{
		Message: &tgclient.Message{
			From: &tgclient.User{ID: 12345, Username: "abebe"},
			Chat: tgclient.Chat{ID: 12345, Type: "private"},
			Text: "/start " + token,
		},
	}

	err = s.svc.HandleUpdate(ctx, update)
	s.Require().NoError(err)

	msg := s.mockTG.LastMessage()
	s.Require().NotNil(msg)
	s.True(strings.Contains(msg.Text, "Successfully linked"))

	user, err := s.userRepo.GetByID(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(user.TelegramID)
	s.Equal(int64(12345), *user.TelegramID)
}

func (s *TelegramSuite) TestHandleUpdate_DeepLink_InvalidToken() {
	ctx := context.Background()

	update := tgclient.Update{
		Message: &tgclient.Message{
			From: &tgclient.User{ID: 12345, Username: "abebe"},
			Chat: tgclient.Chat{ID: 12345, Type: "private"},
			Text: "/start expired-token",
		},
	}

	err := s.svc.HandleUpdate(ctx, update)
	s.Require().NoError(err)

	msg := s.mockTG.LastMessage()
	s.Require().NotNil(msg)
	s.True(strings.Contains(msg.Text, "Invalid or expired"))
}

func (s *TelegramSuite) TestHandleUpdate_Balance() {
	ctx := context.Background()
	userID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	tgID := int64(12345)
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET telegram_id = $2, telegram_username = 'abebe' WHERE id = $1`, userID, tgID)
	s.Require().NoError(err)

	s.mockLedger.Balances[user.LedgerWalletID] = 150075

	update := tgclient.Update{
		Message: &tgclient.Message{
			From: &tgclient.User{ID: 12345},
			Chat: tgclient.Chat{ID: 12345, Type: "private"},
			Text: "/balance",
		},
	}

	err = s.svc.HandleUpdate(ctx, update)
	s.Require().NoError(err)

	msg := s.mockTG.LastMessage()
	s.Require().NotNil(msg)
	s.True(strings.Contains(msg.Text, "1500.75"))
}

func (s *TelegramSuite) TestHandleUpdate_Balance_NotLinked() {
	ctx := context.Background()

	update := tgclient.Update{
		Message: &tgclient.Message{
			From: &tgclient.User{ID: 99999},
			Chat: tgclient.Chat{ID: 99999, Type: "private"},
			Text: "/balance",
		},
	}

	err := s.svc.HandleUpdate(ctx, update)
	s.Require().NoError(err)

	msg := s.mockTG.LastMessage()
	s.Require().NotNil(msg)
	s.True(strings.Contains(msg.Text, "not linked"))
}

func (s *TelegramSuite) TestHandleUpdate_Help() {
	ctx := context.Background()

	update := tgclient.Update{
		Message: &tgclient.Message{
			From: &tgclient.User{ID: 12345},
			Chat: tgclient.Chat{ID: 12345, Type: "private"},
			Text: "/help",
		},
	}

	err := s.svc.HandleUpdate(ctx, update)
	s.Require().NoError(err)

	msg := s.mockTG.LastMessage()
	s.Require().NotNil(msg)
	s.True(strings.Contains(msg.Text, "Available commands"))
}

func TestTelegramSuite(t *testing.T) {
	suite.Run(t, new(TelegramSuite))
}
