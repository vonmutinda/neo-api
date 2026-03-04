package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vonmutinda/neo/internal/config"
	iso "github.com/vonmutinda/neo/internal/gateway/ethswitch/iso8583"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	cardauth "github.com/vonmutinda/neo/internal/services/card_auth"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/pkg/cache"
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

	cardRepo := repository.NewCardRepository(pool)
	authRepo := repository.NewCardAuthorizationRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	balanceRepo := repository.NewCurrencyBalanceRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	ruleRepo := repository.NewRegulatoryRuleRepository(pool)
	totalsRepo := repository.NewTransferTotalsRepository(pool)

	ledgerClient := ledger.NewFormanceClient(cfg.Formance)
	chart := ledger.NewChart(cfg.Formance.AccountPrefix)

	gwCache := cache.NewMemoryCache()
	fxRateRepo := repository.NewFXRateRepository(pool)
	rateProvider := convert.NewDatabaseRateProvider(fxRateRepo, 1.5, 60*time.Second, gwCache)
	rateFn := regulatory.RateFunc(func(ctx context.Context, from, to string) (float64, error) {
		r, err := rateProvider.GetRate(ctx, from, to)
		if err != nil {
			return 0, err
		}
		return r.Mid, nil
	})
	regulatorySvc := regulatory.NewService(ruleRepo, totalsRepo, rateFn, 60*time.Second, gwCache)

	iso4217Resolver := cardauth.StaticISO4217Resolver(cardauth.DefaultISO4217Map)

	cardAuthService := cardauth.NewService(cardRepo, authRepo, userRepo, balanceRepo, auditRepo, ledgerClient, chart, rateProvider, regulatorySvc, iso4217Resolver)

	listenAddr := envOrDefault("ISO8583_LISTEN_ADDR", ":8583")
	router := iso.NewRouter(listenAddr, cardAuthService.HandleMessage, log)

	log.Info("ethswitch-gateway starting",
		slog.String("iso8583_addr", listenAddr),
	)

	return router.Start(ctx)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
