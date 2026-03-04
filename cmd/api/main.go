package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ardanlabs/conf/v3"

	"github.com/vonmutinda/neo/internal/config"
	"github.com/vonmutinda/neo/internal/repository"
	nlog "github.com/vonmutinda/neo/pkg/logger"
)

var (
	build       = "dev"
	environment = "development"
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

	// --- Load Configuration ---
	cfg, err := config.LoadAPI()
	if err != nil {
		return err
	}

	// Railway and similar platforms set PORT at runtime; prefer it over NEO_WEB_PORT.
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Web.Port = p
		}
	}

	cfg.AppInfo.AppVersion.Build = build
	cfg.AppInfo.Environment = environment

	log = nlog.NewLogger(cfg.Log)
	slog.SetDefault(log)

	log.Info("starting neobank API", slog.String("env", cfg.AppInfo.Environment))
	if cfg.JWT == nil || cfg.JWT.SigningKey == "" {
		log.Warn("WARNING: running with dev-mode auth (no JWT validation). Set NEO_JWT_SIGNING_KEY for production.")
	}

	out, err := conf.String(cfg)
	if err != nil {
		return fmt.Errorf("failed to get API configuration: %w", err)
	}
	if out != "" {
		fmt.Println(out)
	}

	// --- Initialize PostgreSQL ---
	log.Info("connecting to PostgreSQL...")
	pool, err := repository.ConnectPool(ctx, cfg.Database.BuildConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	db := repository.NewDB(pool)
	log.Info("PostgreSQL connected")

	// --- Build Dependencies ---
	deps, err := newDeps(ctx, cfg, db, log)
	if err != nil {
		return err
	}

	// --- Build Router ---
	router := buildRouter(deps, cfg, log)

	// --- Start Server ---
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Web.Port),
		Handler:      router,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("HTTP server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// --- Graceful Shutdown ---
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received, draining connections...")
	case err := <-errCh:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Info("server stopped gracefully")
	return nil
}
