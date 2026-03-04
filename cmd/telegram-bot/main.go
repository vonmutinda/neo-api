package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/config"
	tgclient "github.com/vonmutinda/neo/internal/gateway/telegram"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	tgsvc "github.com/vonmutinda/neo/internal/services/telegram"
	"github.com/vonmutinda/neo/internal/transport/webhook"
	nlog "github.com/vonmutinda/neo/pkg/logger"
)

func main() {
	log := nlog.NewLogger(&config.Log{Level: "info"})
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadAPI()
	if err != nil {
		return err
	}

	pool, err := repository.ConnectPool(ctx, cfg.Database.BuildConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	tokenRepo := repository.NewTelegramLinkTokenRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)

	ledgerClient := ledger.NewFormanceClient(cfg.Formance)

	telegramClient := tgclient.NewBotClient(cfg.Telegram.BotToken)
	telegramService := tgsvc.NewService(userRepo, tokenRepo, auditRepo, telegramClient, ledgerClient)

	telegramWebhook := webhook.NewTelegramWebhook(telegramService)

	r := chi.NewRouter()
	r.Post("/webhook/telegram", telegramWebhook.Handle)

	srv := &http.Server{
		Addr:         ":8081",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info("telegram-bot webhook server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("telegram server error", slog.String("error", err.Error()))
		}
	}()

	if cfg.Telegram.WebhookURL != "" {
		if err := telegramClient.SetWebhook(ctx, cfg.Telegram.WebhookURL); err != nil {
			log.Error("failed to set telegram webhook", slog.String("error", err.Error()))
		}
	}

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}
