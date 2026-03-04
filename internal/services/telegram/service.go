package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	tgclient "github.com/vonmutinda/neo/internal/gateway/telegram"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/money"
)

// Service handles Telegram bot commands and deep-link user binding.
type Service struct {
	users      repository.UserRepository
	tokens     repository.TelegramLinkTokenRepository
	audit      repository.AuditRepository
	telegram   tgclient.Client
	ledger     ledger.Client
}

func NewService(
	users repository.UserRepository,
	tokens repository.TelegramLinkTokenRepository,
	audit repository.AuditRepository,
	telegramClient tgclient.Client,
	ledgerClient ledger.Client,
) *Service {
	return &Service{
		users:    users,
		tokens:   tokens,
		audit:    audit,
		telegram: telegramClient,
		ledger:   ledgerClient,
	}
}

// HandleUpdate processes an incoming Telegram webhook update.
func (s *Service) HandleUpdate(ctx context.Context, update tgclient.Update) error {
	if update.Message == nil || update.Message.From == nil {
		return nil
	}

	msg := update.Message
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	switch {
	case strings.HasPrefix(text, "/start "):
		token := strings.TrimPrefix(text, "/start ")
		return s.handleDeepLink(ctx, chatID, msg.From.ID, msg.From.Username, token)

	case text == "/balance":
		return s.handleBalance(ctx, chatID, msg.From.ID)

	case text == "/help":
		return s.telegram.SendMessage(ctx, chatID,
			"Available commands:\n/balance - Check your wallet balance\n/help - Show this help message")

	default:
		return s.telegram.SendMessage(ctx, chatID, "Unknown command. Type /help for available commands.")
	}
}

// handleDeepLink validates the cryptographic token against the
// telegram_link_tokens table, atomically consumes it, and binds the
// Telegram user to their neobank account.
func (s *Service) handleDeepLink(ctx context.Context, chatID, telegramID int64, username, token string) error {
	// Atomically consume the token and get the associated user ID.
	// This prevents replay attacks -- each token can only be used once.
	userID, err := s.tokens.Consume(ctx, token)
	if err != nil {
		return s.telegram.SendMessage(ctx, chatID,
			"Invalid or expired link token. Please generate a new one from the app.")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return s.telegram.SendMessage(ctx, chatID,
			"Account not found. Please contact support.")
	}

	if err := s.users.BindTelegram(ctx, user.ID, telegramID, username); err != nil {
		return fmt.Errorf("binding telegram to user %s: %w", user.ID, err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditTelegramBound,
		ActorType:    "user",
		ActorID:      &user.ID,
		ResourceType: "user",
		ResourceID:   user.ID,
	})

	return s.telegram.SendMessage(ctx, chatID,
		fmt.Sprintf("Successfully linked! Welcome, %s.", user.FullName()))
}

func (s *Service) handleBalance(ctx context.Context, chatID, telegramID int64) error {
	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return s.telegram.SendMessage(ctx, chatID,
			"Your Telegram account is not linked yet. Open the app and go to Settings > Link Telegram.")
	}

	assets := money.AllAssets()
	balances, err := s.ledger.GetMultiCurrencyBalances(ctx, user.LedgerWalletID, assets)
	if err != nil {
		return s.telegram.SendMessage(ctx, chatID, "Could not fetch your balances. Please try again later.")
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Hi %s! Your balances:", user.FullName()))
	for _, cur := range money.SupportedCurrencies {
		asset := money.FormatAsset(cur.Code)
		bal := balances[asset]
		cents := bal.Int64()
		if cents == 0 {
			lines = append(lines, fmt.Sprintf("  %s %s: %s 0.00", cur.Flag, cur.Code, cur.Symbol))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s: %s", cur.Flag, cur.Code, money.Display(cents, cur.Code)))
		}
	}
	return s.telegram.SendMessage(ctx, chatID, strings.Join(lines, "\n"))
}
