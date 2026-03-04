package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vonmutinda/neo/internal/config"
	"github.com/vonmutinda/neo/internal/gateway/nbe"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/lending"
	nlog "github.com/vonmutinda/neo/pkg/logger"
	"github.com/robfig/cron/v3"
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

	loanRepo := repository.NewLoanRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	receiptRepo := repository.NewTransactionReceiptRepository(pool)

	ledgerClient := ledger.NewFormanceClient(cfg.Formance)
	chart := ledger.NewChart(cfg.Formance.AccountPrefix)
	nbeClient := nbe.NewStubClient()

	scoringService := lending.NewScoringService(loanRepo, userRepo, auditRepo, ledgerClient, nbeClient, chart)
	repaymentService := lending.NewRepaymentService(loanRepo, userRepo, auditRepo, ledgerClient, receiptRepo)

	c := cron.New()

	// Auto-sweep: daily at 06:00 AM EAT (03:00 UTC)
	if _, err := c.AddFunc("0 3 * * *", func() {
		log.Info("starting auto-sweep repayment")
		svcCtx := nlog.WithContext(ctx, log)
		if err := repaymentService.AutoSweep(svcCtx); err != nil {
			log.Error("auto-sweep failed", slog.String("error", err.Error()))
		}
	}); err != nil {
		return err
	}

	// Trust score recalculation: weekly on Sunday at 02:00 AM EAT
	if _, err := c.AddFunc("0 23 * * 6", func() {
		log.Info("starting trust score recalculation")
		svcCtx := nlog.WithContext(ctx, log)
		_ = scoringService.CalculateTrustScore(svcCtx, "all-users-placeholder")
	}); err != nil {
		return err
	}

	c.Start()
	log.Info("lending-worker started")

	<-ctx.Done()
	c.Stop()
	log.Info("lending-worker stopped")
	return nil
}
