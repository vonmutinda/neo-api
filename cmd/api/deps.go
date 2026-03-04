package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/vonmutinda/neo/internal/config"
	"github.com/vonmutinda/neo/internal/gateway/ethswitch"

	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/vonmutinda/neo/internal/gateway/fayda"
	"github.com/vonmutinda/neo/internal/gateway/nbe"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/admin"
	authsvc "github.com/vonmutinda/neo/internal/services/auth"
	"github.com/vonmutinda/neo/internal/services/balances"
	"github.com/vonmutinda/neo/internal/services/beneficiary"
	"github.com/vonmutinda/neo/internal/services/business"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/lending"
	"github.com/vonmutinda/neo/internal/services/onboarding"
	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/services/payment_requests"
	"github.com/vonmutinda/neo/internal/services/payments"
	"github.com/vonmutinda/neo/internal/services/permissions"
	"github.com/vonmutinda/neo/internal/services/pots"
	recipientsvc "github.com/vonmutinda/neo/internal/services/recipient"
	"github.com/vonmutinda/neo/internal/services/pricing"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/internal/services/wallet"
	"github.com/vonmutinda/neo/internal/services/remittance"
	"github.com/vonmutinda/neo/internal/services/remittance/wise"
	"github.com/vonmutinda/neo/internal/transport/http/handlers"
	adminh "github.com/vonmutinda/neo/internal/transport/http/handlers/admin"
	bizh "github.com/vonmutinda/neo/internal/transport/http/handlers/business"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	webhookh "github.com/vonmutinda/neo/internal/transport/http/handlers/webhooks"
	"github.com/vonmutinda/neo/pkg/geo"
)

// Deps holds all initialized dependencies needed to build the HTTP router.
// This is the application's composition root -- every repository, service,
// and handler is constructed here and passed to the router.
type Deps struct {
	Handler         *HandlerList
	JWTConfig       *authsvc.JWTConfig
	IdempotencyRepo repository.IdempotencyRepository
	BizRepo         repository.BusinessRepository
	BizMemberRepo   repository.BusinessMemberRepository
	AppCache        cache.Cache
}

func newDeps(ctx context.Context, cfg *config.API, db *repository.DB, log *slog.Logger) (*Deps, error) {
	pool := db.Pool

	// --- Repositories ---
	idempotencyRepo := repository.NewIdempotencyRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	kycRepo := repository.NewKYCRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	receiptRepo := repository.NewTransactionReceiptRepository(pool)
	cardRepo := repository.NewCardRepository(pool)
	loanRepo := repository.NewLoanRepository(pool)
	overdraftRepo := repository.NewOverdraftRepository(pool)
	currencyBalanceRepo := repository.NewCurrencyBalanceRepository(pool)
	accountDetailsRepo := repository.NewAccountDetailsRepository(pool)
	potRepo := repository.NewPotRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)

	// Business repositories
	bizRepo := repository.NewBusinessRepository(pool)
	bizMemberRepo := repository.NewBusinessMemberRepository(pool)
	bizRoleRepo := repository.NewBusinessRoleRepository(pool)
	ptRepo := repository.NewPendingTransferRepository(pool)
	catRepo := repository.NewTransactionCategoryRepository(pool)
	labelRepo := repository.NewTransactionLabelRepository(pool)
	taxPotRepo := repository.NewTaxPotRepository(pool)
	batchRepo := repository.NewBatchPaymentRepository(pool)
	invoiceRepo := repository.NewInvoiceRepository(pool)
	docRepo := repository.NewBusinessDocumentRepository(pool)
	bizLoanRepo := repository.NewBusinessLoanRepository(pool)

	// FX compliance repositories
	ruleRepo := repository.NewRegulatoryRuleRepository(pool)
	totalsRepo := repository.NewTransferTotalsRepository(pool)
	beneficiaryRepo := repository.NewBeneficiaryRepository(pool)
	recipientRepo := repository.NewRecipientRepository(pool)
	paymentRequestRepo := repository.NewPaymentRequestRepository(pool)

	// Pricing repositories
	feeScheduleRepo := repository.NewFeeScheduleRepository(pool)
	remittanceProviderRepo := repository.NewRemittanceProviderRepository(pool)
	remittanceTransferRepo := repository.NewRemittanceTransferRepository(pool)

	// --- Cache ---
	var appCache cache.Cache
	if cfg.Redis != nil && cfg.Redis.URL != "" {
		rc, err := cache.NewRedisCache(cfg.Redis.URL)
		if err != nil {
			log.Warn("Redis unavailable, falling back to in-memory cache", slog.String("url", cfg.Redis.URL), slog.String("error", err.Error()))
			appCache = cache.NewMemoryCache()
		} else {
			appCache = rc
			log.Info("Redis cache connected", slog.String("url", cfg.Redis.URL))
		}
	} else {
		appCache = cache.NewMemoryCache()
		log.Info("using in-memory cache (no Redis URL configured)")
	}

	// --- Formance Ledger ---
	log.Info("initializing Formance ledger client...")
	ledgerClient := ledger.NewFormanceClient(cfg.Formance)
	if err := ledgerClient.EnsureLedgerExists(ctx); err != nil {
		return nil, err
	}
	log.Info("Formance ledger ready", slog.String("ledger", cfg.Formance.LedgerName))

	chart := ledger.NewChart(cfg.Formance.AccountPrefix)

	// --- Gateway Clients ---
	ethswitchClient, err := ethswitch.NewMTLSClient(cfg.EthSwitch)
	if err != nil {
		return nil, err
	}
	log.Info("EthSwitch mTLS client initialized")

	faydaClient := fayda.NewClient(cfg.Fayda)
	nbeClient := nbe.NewStubClient()

	// --- FX Rates ---
	fxRateRepo := repository.NewFXRateRepository(pool)

	oxrAppID := ""
	if cfg.OpenExchangeRates != nil {
		oxrAppID = cfg.OpenExchangeRates.AppID
	}
	primarySource := convert.NewOpenExchangeRateSource(oxrAppID)
	fallbackSource := convert.NewExchangeRateAPISource()
	rateSource := convert.NewChainedRateSource(primarySource, fallbackSource)

	rateRefreshSvc := convert.NewRateRefreshService(rateSource, fxRateRepo, auditRepo, 1.5)
	rateProvider := convert.NewDatabaseRateProvider(fxRateRepo, 1.5, 60*time.Second, appCache)

	// --- Pricing ---
	var remittanceProviders []remittance.RemittanceProvider
	if cfg.Wise != nil && cfg.Wise.APIToken != "" {
		wiseProvider := wise.NewProvider(cfg.Wise.APIToken, cfg.Wise.ProfileID, cfg.Wise.BaseURL)
		remittanceProviders = append(remittanceProviders, wiseProvider)
	}
	providerRouter := remittance.NewProviderRouter(remittanceProviders, 30*time.Second, appCache)
	pricingSvc := pricing.NewService(feeScheduleRepo, providerRouter, appCache)

	// --- Services ---
	rateFn := regulatory.RateFunc(func(ctx context.Context, from, to string) (float64, error) {
		r, err := rateProvider.GetRate(ctx, from, to)
		if err != nil {
			return 0, err
		}
		return r.Mid, nil
	})
	regulatorySvc := regulatory.NewService(ruleRepo, totalsRepo, rateFn, 60*time.Second, appCache)

	balancesSvc := balances.NewService(currencyBalanceRepo, accountDetailsRepo, userRepo, ledgerClient, regulatorySvc)
	onboardingSvc := onboarding.NewService(userRepo, kycRepo, auditRepo, faydaClient, ledgerClient, balancesSvc)

	var signingKey string
	if cfg.JWT != nil {
		signingKey = cfg.JWT.SigningKey
	}
	jwtConfig := authsvc.NewJWTConfig(signingKey)
	userAuthSvc := authsvc.NewService(userRepo, sessionRepo, auditRepo, ledgerClient, balancesSvc, jwtConfig)
	beneficiarySvc := beneficiary.NewService(beneficiaryRepo, auditRepo)
	recipientSvc := recipientsvc.NewService(recipientRepo, userRepo, auditRepo)
	disbursementSvc := lending.NewDisbursementService(loanRepo, userRepo, auditRepo, ledgerClient, nbeClient, receiptRepo)
	loanQuerySvc := lending.NewQueryService(loanRepo, userRepo)
	overdraftSvc := overdraft.NewService(overdraftRepo, loanRepo, userRepo, ledgerClient, auditRepo)
	paymentsSvc := payments.NewService(userRepo, receiptRepo, auditRepo, ledgerClient, ethswitchClient, chart, regulatorySvc, totalsRepo, pricingSvc, recipientSvc, overdraftSvc)
	potsSvc := pots.NewService(potRepo, currencyBalanceRepo, userRepo, ledgerClient, chart, receiptRepo)
	convertSvc := convert.NewService(userRepo, ledgerClient, chart, rateProvider, regulatorySvc, receiptRepo, overdraftSvc)
	walletSvc := wallet.NewService(userRepo, receiptRepo, currencyBalanceRepo, ledgerClient, chart, rateProvider)
	paymentRequestSvc := payment_requests.NewService(paymentRequestRepo, userRepo, paymentsSvc, auditRepo)

	permSvc := permissions.NewService(bizRoleRepo, bizMemberRepo, 5*time.Minute, appCache)
	businessSvc := business.NewService(bizRepo, bizMemberRepo, bizRoleRepo, userRepo, auditRepo, ledgerClient, permSvc)

	// --- Admin Services ---
	adminJWTSecret := ""
	adminJWTIssuer := "neobank-admin"
	adminJWTAud := "neobank-admin-api"
	if cfg.AdminJWT != nil {
		adminJWTSecret = cfg.AdminJWT.Secret
		adminJWTIssuer = cfg.AdminJWT.Issuer
		adminJWTAud = cfg.AdminJWT.Audience
	}

	staffRepo := repository.NewStaffRepository(pool)
	flagRepo := repository.NewFlagRepository(pool)
	configRepo := repository.NewSystemConfigRepository(pool)
	adminQueryRepo := repository.NewAdminQueryRepository(pool)

	authSvc := admin.NewAuthService(staffRepo, auditRepo, adminJWTSecret, adminJWTIssuer, adminJWTAud)
	staffSvc := admin.NewStaffService(staffRepo, auditRepo)
	customerSvc := admin.NewCustomerService(adminQueryRepo, userRepo, kycRepo, flagRepo, auditRepo, loanRepo)
	txnSvc := admin.NewTransactionService(adminQueryRepo, receiptRepo, auditRepo)
	adminLoanSvc := admin.NewLoanService(adminQueryRepo, loanRepo, auditRepo)
	adminCardSvc := admin.NewCardService(adminQueryRepo, cardRepo, auditRepo)
	reconSvc := admin.NewReconService(adminQueryRepo, auditRepo)
	geoPath := os.Getenv("GEOIP_DB_PATH")
	var geoReader *geo.Reader
	if geoPath != "" {
		var err error
		geoReader, err = geo.NewReader(geoPath)
		if err != nil {
			return nil, fmt.Errorf("GEOIP_DB_PATH: %w", err)
		}
		if geoReader != nil {
			log.Info("GeoIP database loaded for money-flow map", slog.String("path", geoPath))
		}
	}
	// When geoPath is empty, geoReader is nil; NewAnalyticsService uses NoopLookuper in that case.
	analyticsSvc := admin.NewAnalyticsService(adminQueryRepo, flagRepo, geoReader)
	flagSvc := admin.NewFlagService(flagRepo, auditRepo)
	configSvc := admin.NewConfigService(configRepo, auditRepo)

	// --- Build Handler Facades ---
	handler := &HandlerList{
		Health:  handlers.NewHealthHandler(db.Pool, appCache),
		FXRates: handlers.NewFXRateHandler(rateProvider),

		Personal: persh.NewHandlers(
			userAuthSvc,
			onboardingSvc, paymentsSvc, disbursementSvc, loanQuerySvc, overdraftSvc,
			userRepo, cardRepo, currencyBalanceRepo,
			balancesSvc, walletSvc, potsSvc, convertSvc, beneficiarySvc,
			recipientSvc,
			paymentRequestSvc,
			labelRepo, pricingSvc,
		),

		Business: bizh.NewHandlers(
			businessSvc, catRepo, labelRepo, taxPotRepo, potRepo,
			ptRepo, auditRepo, batchRepo, invoiceRepo, docRepo, bizLoanRepo,
		),

		Admin: adminh.NewHandlers(
			authSvc, staffSvc, customerSvc, txnSvc, adminLoanSvc,
			adminCardSvc, reconSvc, analyticsSvc, flagSvc, configSvc,
			adminQueryRepo, ruleRepo, auditRepo, regulatorySvc,
			fxRateRepo, rateRefreshSvc, 1.5,
			feeScheduleRepo, remittanceProviderRepo,
			appCache,
			ledgerClient, chart,
		),

		Webhooks: webhookh.NewWiseWebhookHandler(remittanceTransferRepo, ""),

		PermissionService: permSvc,
		RegulatoryService: regulatorySvc,
	}

	return &Deps{
		Handler:         handler,
		JWTConfig:       jwtConfig,
		IdempotencyRepo: idempotencyRepo,
		BizRepo:         bizRepo,
		BizMemberRepo:   bizMemberRepo,
		AppCache:        appCache,
	}, nil
}
