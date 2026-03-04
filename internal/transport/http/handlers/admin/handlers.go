package admin

import (
	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/regulatory"

	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/cache"
)

type Handlers struct {
	Auth         *AuthHandler
	Customers    *CustomerHandler
	Transactions *TransactionHandler
	Loans        *LoanHandler
	Cards        *CardHandler
	Recon        *ReconHandler
	Audit        *AuditHandler
	Analytics    *AnalyticsHandler
	Flags        *FlagHandler
	Config       *ConfigHandler
	Rules        *RulesHandler
	Compliance   *ComplianceHandler
	FXRates      *FXRateHandler
	Fees            *FeeHandler
	SystemAccounts  *SystemAccountsHandler
}

func NewHandlers(
	authSvc *adminsvc.AuthService,
	staffSvc *adminsvc.StaffService,
	customerSvc *adminsvc.CustomerService,
	txnSvc *adminsvc.TransactionService,
	loanSvc *adminsvc.LoanService,
	cardSvc *adminsvc.CardService,
	reconSvc *adminsvc.ReconService,
	analyticsSvc *adminsvc.AnalyticsService,
	flagSvc *adminsvc.FlagService,
	configSvc *adminsvc.ConfigService,
	adminQueryRepo repository.AdminQueryRepository,
	ruleRepo repository.RegulatoryRuleRepository,
	auditRepo repository.AuditRepository,
	regulatorySvc *regulatory.Service,
	fxRateRepo repository.FXRateRepository,
	rateRefreshSvc *convert.RateRefreshService,
	spreadPercent float64,
	feeScheduleRepo repository.FeeScheduleRepository,
	remittanceProviderRepo repository.RemittanceProviderRepository,
	c cache.Cache,
	ledgerClient ledger.Client,
	chart *ledger.Chart,
) *Handlers {
	return &Handlers{
		Auth:         NewAuthHandler(authSvc, staffSvc),
		Customers:    NewCustomerHandler(customerSvc),
		Transactions: NewTransactionHandler(txnSvc),
		Loans:        NewLoanHandler(loanSvc),
		Cards:        NewCardHandler(cardSvc),
		Recon:        NewReconHandler(reconSvc),
		Audit:        NewAuditHandler(adminQueryRepo),
		Analytics:    NewAnalyticsHandler(analyticsSvc),
		Flags:        NewFlagHandler(flagSvc),
		Config:       NewConfigHandler(configSvc),
		Rules:        NewRulesHandler(ruleRepo, auditRepo, regulatorySvc),
		Compliance:   NewComplianceHandler(auditRepo, ruleRepo),
		FXRates:      NewFXRateHandler(fxRateRepo, auditRepo, rateRefreshSvc, spreadPercent, c),
		Fees:            NewFeeHandler(feeScheduleRepo, remittanceProviderRepo, auditRepo, c),
		SystemAccounts:  NewSystemAccountsHandler(ledgerClient, chart),
	}
}
