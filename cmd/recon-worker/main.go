package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vonmutinda/neo/internal/config"
	"github.com/vonmutinda/neo/internal/gateway/ethswitch"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/reconciliation"
	"github.com/vonmutinda/neo/pkg/cache"
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

	receiptsRepo := repository.NewTransactionReceiptRepository(pool)
	reconRepo := repository.NewReconciliationRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)

	ledgerClient := ledger.NewFormanceClient(cfg.Formance)

	sftpClient := ethswitch.NewLocalFileSFTP()

	reconService := reconciliation.NewService(receiptsRepo, reconRepo, auditRepo, ledgerClient)

	c := cron.New()

	// EOD reconciliation: run daily at 01:00 AM EAT (UTC+3)
	_, err = c.AddFunc("0 22 * * *", func() {
		log.Info("starting EOD reconciliation")
		localPath := "/tmp/clearing_" + fmt.Sprintf("%d", os.Getpid()) + ".csv"
		if err := sftpClient.DownloadClearingFile(ctx, cfg.EthSwitch.EthSwitchSFTPPath, localPath); err != nil {
			log.Error("failed to download clearing file", slog.String("error", err.Error()))
			return
		}
		svcCtx := nlog.WithContext(ctx, log)
		if err := reconService.RunDailyReconciliation(svcCtx, localPath); err != nil {
			log.Error("reconciliation failed", slog.String("error", err.Error()))
		}
	})
	if err != nil {
		return fmt.Errorf("scheduling recon cron: %w", err)
	}

	// --- Cache ---
	var appCache cache.Cache
	if cfg.Redis != nil && cfg.Redis.URL != "" {
		rc, err := cache.NewRedisCache(cfg.Redis.URL)
		if err != nil {
			return fmt.Errorf("connecting to Redis: %w", err)
		}
		appCache = rc
		log.Info("Redis cache connected")
	} else {
		appCache = cache.NewMemoryCache()
		log.Info("using in-memory cache")
	}

	// --- FX Rate Refresh ---
	fxRateRepo := repository.NewFXRateRepository(pool)

	oxrAppID := ""
	if cfg.OpenExchangeRates != nil {
		oxrAppID = cfg.OpenExchangeRates.AppID
	}
	primarySource := convert.NewOpenExchangeRateSource(oxrAppID)
	fallbackSource := convert.NewExchangeRateAPISource()
	rateSource := convert.NewChainedRateSource(primarySource, fallbackSource)
	rateRefreshSvc := convert.NewRateRefreshService(rateSource, fxRateRepo, auditRepo, 1.5)

	// Refresh FX rates every hour (top of the hour).
	// Hourly is sufficient for ETB's managed-float regime.
	_, err = c.AddFunc("0 * * * *", func() {
		log.Info("refreshing FX rates")
		if err := rateRefreshSvc.Refresh(ctx); err != nil {
			log.Error("FX rate refresh failed", slog.String("error", err.Error()))
		} else {
			_ = appCache.DeleteByPrefix(ctx, "neo:fx:")
		}
	})
	if err != nil {
		return fmt.Errorf("scheduling fx rate refresh cron: %w", err)
	}

	// Clean up FX rate history older than 90 days -- daily at 03:00 UTC
	_, err = c.AddFunc("0 3 * * *", func() {
		n, err := rateRefreshSvc.Cleanup(ctx)
		if err != nil {
			log.Error("FX rate cleanup failed", slog.String("error", err.Error()))
			return
		}
		if n > 0 {
			log.Info("cleaned up old FX rates", slog.Int64("deleted", n))
		}
	})
	if err != nil {
		return fmt.Errorf("scheduling fx rate cleanup cron: %w", err)
	}

	c.Start()
	log.Info("recon-worker started")

	<-ctx.Done()
	c.Stop()
	log.Info("recon-worker stopped")
	return nil
}
