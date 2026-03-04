package main

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/vonmutinda/neo/internal/config"
	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	nlog "github.com/vonmutinda/neo/pkg/logger"
)

func buildRouter(deps *Deps, cfg *config.API, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	applyGlobalMiddleware(r, deps, cfg, log)

	registerHealthRoutes(r, deps)
	registerPublicRoutes(r, deps)

	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.ClientIP)
		r.Use(middleware.Auth(deps.JWTConfig))

		registerPersonalBankingRoutes(r, deps)
		registerAdminRoutes(r, deps)
		registerBusinessRoutes(r, deps)
	})

	// Staff admin API — separate auth, separate JWT secret
	r.Post("/admin/v1/auth/login", deps.Handler.Admin.Auth.Login)

	// Webhooks (unauthenticated, signature-verified internally)
	r.Post("/webhooks/wise", deps.Handler.Webhooks.HandleEvent)

	adminJWTCfg := middleware.JWTConfig{}
	if cfg.AdminJWT != nil {
		adminJWTCfg = middleware.JWTConfig{
			Secret:   []byte(cfg.AdminJWT.Secret),
			Issuer:   cfg.AdminJWT.Issuer,
			Audience: cfg.AdminJWT.Audience,
		}
	}

	r.Route("/admin/v1", func(r chi.Router) {
		r.Use(middleware.AdminAuth(adminJWTCfg))
		registerAdminV1Routes(r, deps)
	})

	return r
}

func applyGlobalMiddleware(r chi.Router, deps *Deps, cfg *config.API, log *slog.Logger) {
	c := cfg.CorsSettings
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   c.AllowedOrigins,
		AllowedMethods:   c.AllowedMethods,
		AllowedHeaders:   c.AllowedHeaders,
		ExposedHeaders:   c.ExposedHeaders,
		AllowCredentials: true,
	}))
	r.Use(middleware.Recovery)
	r.Use(middleware.RequestID)
	r.Use(middleware.BodyLimit(1 << 20))
	r.Use(contextLogger(log))
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.RateLimit(middleware.RateLimitConfig{
		RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
		Burst:             cfg.RateLimit.Burst,
	}, deps.AppCache))
	r.Use(middleware.Idempotency(deps.IdempotencyRepo))
}

// --- Health & Public ---

func registerHealthRoutes(r chi.Router, deps *Deps) {
	h := deps.Handler
	r.Get("/healthz", h.Health.Healthz)
	r.Get("/readyz", h.Health.Readyz)
}

func registerPublicRoutes(r chi.Router, deps *Deps) {
	h := deps.Handler

	// Auth (public, no JWT required)
	r.Post("/v1/auth/register", h.Personal.Auth.Register)
	r.Post("/v1/auth/login", h.Personal.Auth.Login)
	r.Post("/v1/auth/refresh", h.Personal.Auth.Refresh)

	// Legacy registration endpoint (delegates to onboarding)
	r.Post("/v1/register", h.Personal.Onboarding.Register)

	// Public FX rates (unauthenticated)
	r.Get("/v1/fx/rates", h.FXRates.ListRates)
	r.Get("/v1/fx/rates/{from}/{to}", h.FXRates.GetRate)
	r.Get("/v1/fx/convert", h.FXRates.Convert)
}

// --- Personal Banking (authenticated /v1) ---

func registerPersonalBankingRoutes(r chi.Router, deps *Deps) {
	h := deps.Handler

	// Auth (authenticated)
	r.Post("/auth/logout", h.Personal.Auth.Logout)
	r.Post("/auth/change-password", h.Personal.Auth.ChangePassword)

	// Recipient lookup
	r.Get("/users/resolve", h.Personal.Resolve.Resolve)

	// KYC
	r.Post("/kyc/otp", h.Personal.Onboarding.RequestOTP)
	r.Post("/kyc/verify", h.Personal.Onboarding.VerifyOTP)

	// Wallets
	r.Get("/wallets/balance", h.Personal.Wallets.GetBalance)
	r.Get("/wallets/summary", h.Personal.Wallets.GetSummary)
	r.Get("/wallets/transactions", h.Personal.Wallets.GetTransactions)

	// Transfers
	r.Post("/transfers/outbound", h.Personal.Transfers.Outbound)
	r.Post("/transfers/inbound", h.Personal.Transfers.Inbound)
	r.Post("/transfers/batch", h.Personal.Transfers.Batch)

	// Loans
	r.Get("/loans/eligibility", h.Personal.Loans.GetEligibility)
	r.Get("/loans/credit-score", h.Personal.Loans.GetCreditScore)
	r.Get("/loans/credit-score/history", h.Personal.Loans.GetCreditScoreHistory)
	r.Get("/loans", h.Personal.Loans.ListHistory)
	r.Post("/loans/apply", h.Personal.Loans.Apply)
	r.Post("/loans/{id}/repay", h.Personal.Loans.Repay)
	r.Get("/loans/{id}", h.Personal.Loans.GetLoan)

	// Overdraft
	r.Get("/overdraft", h.Personal.Overdraft.Get)
	r.Post("/overdraft/opt-in", h.Personal.Overdraft.OptIn)
	r.Post("/overdraft/opt-out", h.Personal.Overdraft.OptOut)
	r.Post("/overdraft/repay", h.Personal.Overdraft.Repay)

	// Cards
	r.Post("/cards", h.Personal.Cards.CreateCard)
	r.Get("/cards", h.Personal.Cards.ListCards)
	r.Get("/cards/{id}", h.Personal.Cards.GetCard)
	r.Patch("/cards/{id}/status", h.Personal.Cards.UpdateStatus)
	r.Patch("/cards/{id}/limits", h.Personal.Cards.UpdateLimits)
	r.Patch("/cards/{id}/toggles", h.Personal.Cards.UpdateToggles)

	// Currency Balances
	r.Post("/balances", h.Personal.Balances.Create)
	r.Get("/balances", h.Personal.Balances.List)
	r.Delete("/balances/{code}", h.Personal.Balances.Delete)

	// Pots
	r.Post("/pots", h.Personal.Pots.Create)
	r.Get("/pots", h.Personal.Pots.List)
	r.Get("/pots/{id}", h.Personal.Pots.Get)
	r.Patch("/pots/{id}", h.Personal.Pots.Update)
	r.Delete("/pots/{id}", h.Personal.Pots.Delete)
	r.Post("/pots/{id}/add", h.Personal.Pots.Add)
	r.Post("/pots/{id}/withdraw", h.Personal.Pots.Withdraw)
	r.Get("/pots/{id}/transactions", h.Personal.Pots.GetTransactions)

	// Currency conversion
	r.Post("/convert", h.Personal.Convert.Convert)
	r.Get("/convert/rate", h.Personal.Convert.GetRate)

	// User profile
	r.Get("/me", h.Personal.Me.GetMe)
	r.Get("/me/spend-waterfall", h.Personal.SpendWaterfall.Get)
	r.Put("/me/spend-waterfall", h.Personal.SpendWaterfall.Update)

	// Beneficiaries
	r.Post("/beneficiaries", h.Personal.Beneficiaries.Create)
	r.Get("/beneficiaries", h.Personal.Beneficiaries.List)
	r.Delete("/beneficiaries/{id}", h.Personal.Beneficiaries.Delete)

	// Transaction labeling
	r.Post("/transactions/{txId}/label", h.Personal.Categories.LabelTransaction)
	r.Patch("/transactions/{txId}/label", h.Personal.Categories.UpdateLabel)
	r.Delete("/transactions/{txId}/label", h.Personal.Categories.DeleteLabel)

	// Fee quotes
	r.Get("/fees/quote", h.Personal.Fees.GetQuote)
	r.Get("/fees/quote/international", h.Personal.Fees.GetInternationalQuote)

	// Recipients
	r.Post("/recipients", h.Personal.Recipients.Create)
	r.Get("/recipients", h.Personal.Recipients.List)
	r.Get("/recipients/search/bank", h.Personal.Recipients.SearchByBank)
	r.Get("/recipients/search/name", h.Personal.Recipients.SearchByName)
	r.Get("/recipients/{id}", h.Personal.Recipients.Get)
	r.Patch("/recipients/{id}/favorite", h.Personal.Recipients.SetFavorite)
	r.Delete("/recipients/{id}", h.Personal.Recipients.Archive)

	// Bank directory
	r.Get("/banks", h.Personal.Recipients.ListBanks)

	// Payment Requests
	r.Post("/payment-requests", h.Personal.PaymentRequests.Create)
	r.Post("/payment-requests/batch", h.Personal.PaymentRequests.CreateBatch)
	r.Get("/payment-requests/sent", h.Personal.PaymentRequests.ListSent)
	r.Get("/payment-requests/received", h.Personal.PaymentRequests.ListReceived)
	r.Get("/payment-requests/received/count", h.Personal.PaymentRequests.PendingCount)
	r.Get("/payment-requests/{id}", h.Personal.PaymentRequests.Get)
	r.Post("/payment-requests/{id}/pay", h.Personal.PaymentRequests.Pay)
	r.Post("/payment-requests/{id}/decline", h.Personal.PaymentRequests.Decline)
	r.Delete("/payment-requests/{id}", h.Personal.PaymentRequests.Cancel)
	r.Post("/payment-requests/{id}/remind", h.Personal.PaymentRequests.Remind)
}

// --- Admin (authenticated /v1/admin) ---

func registerAdminRoutes(r chi.Router, deps *Deps) {
	h := deps.Handler

	r.Get("/admin/rules", h.Admin.Rules.List)
	r.Get("/admin/rules/{id}", h.Admin.Rules.Get)
	r.Post("/admin/rules", h.Admin.Rules.Create)
	r.Patch("/admin/rules/{id}", h.Admin.Rules.Update)

	r.Get("/admin/compliance/report", h.Admin.Compliance.Report)
}

// --- Business (authenticated /v1/business) ---

func registerBusinessRoutes(r chi.Router, deps *Deps) {
	h := deps.Handler

	r.Post("/business/register", h.Business.Core.Register)
	r.Get("/business/mine", h.Business.Core.ListMine)

	r.Route("/business/{id}", func(r chi.Router) {
		r.Use(middleware.BusinessContext(deps.BizRepo, deps.BizMemberRepo))

		r.Get("/", h.Business.Core.Get)
		r.With(perm(deps, domain.BPermManageSettings)).Patch("/", h.Business.Core.Update)

		// Members
		r.Get("/members", h.Business.Core.ListMembers)
		r.With(perm(deps, domain.BPermManageMembers)).Post("/members", h.Business.Core.InviteMember)
		r.With(perm(deps, domain.BPermManageMembers)).Patch("/members/{memberId}", h.Business.Core.UpdateMemberRole)
		r.With(perm(deps, domain.BPermManageMembers)).Delete("/members/{memberId}", h.Business.Core.RemoveMember)
		r.Get("/members/me/permissions", h.Business.Core.GetMyPermissions)

		// Roles
		r.Get("/roles", h.Business.Core.ListRoles)
		r.With(perm(deps, domain.BPermManageRoles)).Post("/roles", h.Business.Core.CreateRole)
		r.Get("/roles/{roleId}", h.Business.Core.GetRole)
		r.With(perm(deps, domain.BPermManageRoles)).Patch("/roles/{roleId}", h.Business.Core.UpdateRole)
		r.With(perm(deps, domain.BPermManageRoles)).Delete("/roles/{roleId}", h.Business.Core.DeleteRole)

		// Categories
		r.Get("/categories", h.Business.Categories.ListCategories)
		r.With(perm(deps, domain.BPermLabelTransactions)).Post("/categories", h.Business.Categories.CreateCategory)
		r.With(perm(deps, domain.BPermLabelTransactions)).Patch("/categories/{catId}", h.Business.Categories.UpdateCategory)
		r.With(perm(deps, domain.BPermLabelTransactions)).Delete("/categories/{catId}", h.Business.Categories.DeleteCategory)
		r.Get("/transactions/labeled", h.Business.Categories.ListLabeled)
		r.Get("/transactions/tax-summary", h.Business.Categories.TaxSummary)

		// Tax pots
		r.With(perm(deps, domain.BPermManageTaxPots)).Post("/tax-pots", h.Business.TaxPots.Create)
		r.Get("/tax-pots", h.Business.TaxPots.List)
		r.With(perm(deps, domain.BPermManageTaxPots)).Patch("/tax-pots/{taxPotId}", h.Business.TaxPots.Update)
		r.With(perm(deps, domain.BPermManageTaxPots)).Delete("/tax-pots/{taxPotId}", h.Business.TaxPots.Delete)
		r.Get("/tax-pots/summary", h.Business.TaxPots.Summary)

		// Transfers
		r.With(perm(deps, domain.BPermApproveTransfer)).Get("/transfers/pending", h.Business.PendingTransfers.List)
		r.With(perm(deps, domain.BPermApproveTransfer)).Get("/transfers/pending/{transferId}", h.Business.PendingTransfers.Get)
		r.With(perm(deps, domain.BPermApproveTransfer)).Post("/transfers/pending/{transferId}/approve", h.Business.PendingTransfers.Approve)
		r.With(perm(deps, domain.BPermApproveTransfer)).Post("/transfers/pending/{transferId}/reject", h.Business.PendingTransfers.Reject)
		r.Get("/transfers/pending/count", h.Business.PendingTransfers.Count)

		// Batch payments
		r.With(perm(deps, domain.BPermBatchCreate)).Post("/batch-payments", h.Business.BatchPayments.Create)
		r.Get("/batch-payments", h.Business.BatchPayments.List)
		r.Get("/batch-payments/{batchId}", h.Business.BatchPayments.Get)
		r.With(perm(deps, domain.BPermBatchApprove)).Post("/batch-payments/{batchId}/approve", h.Business.BatchPayments.Approve)
		r.With(perm(deps, domain.BPermBatchExecute)).Post("/batch-payments/{batchId}/process", h.Business.BatchPayments.Process)

		// Invoices
		r.With(perm(deps, domain.BPermManageInvoices)).Post("/invoices", h.Business.Invoices.Create)
		r.Get("/invoices", h.Business.Invoices.List)
		r.Get("/invoices/{invId}", h.Business.Invoices.Get)
		r.With(perm(deps, domain.BPermManageInvoices)).Patch("/invoices/{invId}", h.Business.Invoices.Update)
		r.With(perm(deps, domain.BPermManageInvoices)).Post("/invoices/{invId}/send", h.Business.Invoices.Send)
		r.With(perm(deps, domain.BPermManageInvoices)).Post("/invoices/{invId}/cancel", h.Business.Invoices.Cancel)
		r.With(perm(deps, domain.BPermManageInvoices)).Post("/invoices/{invId}/record-payment", h.Business.Invoices.RecordPayment)
		r.Get("/invoices/summary", h.Business.Invoices.Summary)

		// Documents
		r.Post("/documents/upload-url", h.Business.Documents.GetUploadURL)
		r.With(perm(deps, domain.BPermManageDocuments)).Post("/documents", h.Business.Documents.Create)
		r.Get("/documents", h.Business.Documents.List)
		r.Get("/documents/{docId}", h.Business.Documents.Get)
		r.With(perm(deps, domain.BPermManageDocuments)).Patch("/documents/{docId}", h.Business.Documents.Update)
		r.With(perm(deps, domain.BPermManageDocuments)).Delete("/documents/{docId}", h.Business.Documents.Archive)
		r.Get("/documents/expiring", h.Business.Documents.ListExpiring)

		// Loans
		r.Get("/loans/eligibility", h.Business.Loans.GetEligibility)
		r.Get("/loans", h.Business.Loans.List)
		r.With(perm(deps, domain.BPermApplyLoan)).Post("/loans/apply", h.Business.Loans.Apply)
		r.Get("/loans/{loanId}", h.Business.Loans.Get)

		// Accounting / Export
		r.With(perm(deps, domain.BPermExportTransactions)).Get("/export/transactions", h.Business.Accounting.ExportTransactions)
		r.With(perm(deps, domain.BPermExportTransactions)).Get("/export/profit-loss", h.Business.Accounting.ProfitAndLoss)
		r.With(perm(deps, domain.BPermExportTransactions)).Get("/export/balance-sheet", h.Business.Accounting.BalanceSheet)
		r.With(perm(deps, domain.BPermExportTransactions)).Get("/export/tax-report", h.Business.Accounting.TaxReport)
	})
}

// --- Admin V1 Routes (staff-authenticated) ---

func registerAdminV1Routes(r chi.Router, deps *Deps) {
	h := deps.Handler
	requirePerm := func(p domain.Permission) func(http.Handler) http.Handler {
		return middleware.RequirePermission(p)
	}

	// Staff auth
	r.Post("/auth/change-password", h.Admin.Auth.ChangePassword)
	r.Get("/staff/me", h.Admin.Auth.GetMe)

	// Staff management
	r.With(requirePerm(domain.PermStaffManage)).Get("/staff", h.Admin.Auth.ListStaff)
	r.With(requirePerm(domain.PermStaffManage)).Post("/staff", h.Admin.Auth.CreateStaff)
	r.With(requirePerm(domain.PermStaffManage)).Get("/staff/{id}", h.Admin.Auth.GetStaff)
	r.With(requirePerm(domain.PermStaffManage)).Patch("/staff/{id}", h.Admin.Auth.UpdateStaff)
	r.With(requirePerm(domain.PermStaffManage)).Delete("/staff/{id}", h.Admin.Auth.DeactivateStaff)

	// Customers
	r.With(requirePerm(domain.PermUsersRead)).Get("/customers", h.Admin.Customers.List)
	r.With(requirePerm(domain.PermUsersRead)).Get("/customers/{id}", h.Admin.Customers.GetProfile)
	r.With(requirePerm(domain.PermUsersFreezeUnfreeze)).Post("/customers/{id}/freeze", h.Admin.Customers.Freeze)
	r.With(requirePerm(domain.PermUsersFreezeUnfreeze)).Post("/customers/{id}/unfreeze", h.Admin.Customers.Unfreeze)
	r.With(requirePerm(domain.PermUsersKYCOverride)).Post("/customers/{id}/kyc-override", h.Admin.Customers.KYCOverride)
	r.With(requirePerm(domain.PermUsersRead)).Post("/customers/{id}/note", h.Admin.Customers.AddNote)
	r.With(requirePerm(domain.PermUsersRead)).Get("/customers/{id}/flags", h.Admin.Flags.ListByCustomer)

	// Transactions
	r.With(requirePerm(domain.PermTransactionsRead)).Get("/transactions", h.Admin.Transactions.List)
	r.With(requirePerm(domain.PermTransactionsRead)).Get("/transactions/{id}", h.Admin.Transactions.Get)
	r.With(requirePerm(domain.PermTransactionsRead)).Get("/transactions/{id}/conversion", h.Admin.Transactions.GetConversion)
	r.With(requirePerm(domain.PermTransactionsReverse)).Post("/transactions/{id}/reverse", h.Admin.Transactions.Reverse)

	// Loans
	r.With(requirePerm(domain.PermLoansRead)).Get("/loans", h.Admin.Loans.List)
	r.With(requirePerm(domain.PermLoansRead)).Get("/loans/summary", h.Admin.Loans.Summary)
	r.With(requirePerm(domain.PermLoansRead)).Get("/loans/{id}", h.Admin.Loans.Get)
	r.With(requirePerm(domain.PermLoansWriteOff)).Post("/loans/{id}/write-off", h.Admin.Loans.WriteOff)
	r.With(requirePerm(domain.PermLoansRead)).Get("/credit-profiles", h.Admin.Loans.ListCreditProfiles)
	r.With(requirePerm(domain.PermLoansRead)).Get("/credit-profiles/{userId}", h.Admin.Loans.GetCreditProfile)
	r.With(requirePerm(domain.PermLoansCreditOverride)).Post("/credit-profiles/{userId}/override", h.Admin.Loans.OverrideCredit)

	// Cards
	r.With(requirePerm(domain.PermCardsRead)).Get("/cards", h.Admin.Cards.List)
	r.With(requirePerm(domain.PermCardsRead)).Get("/cards/{id}", h.Admin.Cards.Get)
	r.With(requirePerm(domain.PermCardsRead)).Get("/cards/{id}/authorizations", h.Admin.Cards.ListAuthorizations)
	r.With(requirePerm(domain.PermCardsManage)).Post("/cards/{id}/freeze", h.Admin.Cards.Freeze)
	r.With(requirePerm(domain.PermCardsManage)).Post("/cards/{id}/unfreeze", h.Admin.Cards.Unfreeze)
	r.With(requirePerm(domain.PermCardsManage)).Post("/cards/{id}/cancel", h.Admin.Cards.Cancel)
	r.With(requirePerm(domain.PermCardsManage)).Patch("/cards/{id}/limits", h.Admin.Cards.UpdateLimits)

	// Reconciliation
	r.With(requirePerm(domain.PermReconRead)).Get("/reconciliation/runs", h.Admin.Recon.ListRuns)
	r.With(requirePerm(domain.PermReconRead)).Get("/reconciliation/exceptions", h.Admin.Recon.ListExceptions)
	r.With(requirePerm(domain.PermReconManage)).Post("/reconciliation/exceptions/{id}/assign", h.Admin.Recon.Assign)
	r.With(requirePerm(domain.PermReconManage)).Post("/reconciliation/exceptions/{id}/investigate", h.Admin.Recon.Investigate)
	r.With(requirePerm(domain.PermReconManage)).Post("/reconciliation/exceptions/{id}/resolve", h.Admin.Recon.Resolve)
	r.With(requirePerm(domain.PermReconManage)).Post("/reconciliation/exceptions/{id}/escalate", h.Admin.Recon.Escalate)

	// Audit
	r.With(requirePerm(domain.PermAuditRead)).Get("/audit", h.Admin.Audit.List)
	r.With(requirePerm(domain.PermAuditRead)).Get("/audit/{id}", h.Admin.Audit.Get)

	// Analytics
	r.With(requirePerm(domain.PermAnalyticsRead)).Get("/analytics/overview", h.Admin.Analytics.Overview)
	r.With(requirePerm(domain.PermAnalyticsRead)).Get("/analytics/money-flow-map", h.Admin.Analytics.MoneyFlowMap)

	// Flags
	r.With(requirePerm(domain.PermFlagsManage)).Get("/flags", h.Admin.Flags.List)
	r.With(requirePerm(domain.PermFlagsManage)).Post("/flags", h.Admin.Flags.Create)
	r.With(requirePerm(domain.PermFlagsManage)).Post("/flags/{id}/resolve", h.Admin.Flags.Resolve)

	// Regulatory rules (migrated from /v1/admin/rules)
	r.With(requirePerm(domain.PermConfigManage)).Get("/rules", h.Admin.Rules.List)
	r.With(requirePerm(domain.PermConfigManage)).Get("/rules/{id}", h.Admin.Rules.Get)
	r.With(requirePerm(domain.PermConfigManage)).Post("/rules", h.Admin.Rules.Create)
	r.With(requirePerm(domain.PermConfigManage)).Patch("/rules/{id}", h.Admin.Rules.Update)
	r.With(requirePerm(domain.PermConfigManage)).Get("/compliance/report", h.Admin.Compliance.Report)

	// System config
	r.With(requirePerm(domain.PermConfigManage)).Get("/config", h.Admin.Config.ListAll)
	r.With(requirePerm(domain.PermConfigManage)).Patch("/config", h.Admin.Config.Update)

	// FX Rate Management
	r.With(requirePerm(domain.PermConfigManage)).Get("/fx/rates", h.Admin.FXRates.ListCurrent)
	r.With(requirePerm(domain.PermConfigManage)).Get("/fx/rates/history", h.Admin.FXRates.ListHistory)
	r.With(requirePerm(domain.PermConfigManage)).Post("/fx/rates", h.Admin.FXRates.Override)
	r.With(requirePerm(domain.PermConfigManage)).Post("/fx/rates/refresh", h.Admin.FXRates.TriggerRefresh)

	// Fee Management
	r.With(requirePerm(domain.PermConfigManage)).Get("/fees", h.Admin.Fees.ListSchedules)
	r.With(requirePerm(domain.PermConfigManage)).Post("/fees", h.Admin.Fees.CreateSchedule)
	r.With(requirePerm(domain.PermConfigManage)).Put("/fees/{id}", h.Admin.Fees.UpdateSchedule)
	r.With(requirePerm(domain.PermConfigManage)).Delete("/fees/{id}", h.Admin.Fees.DeactivateSchedule)
	r.With(requirePerm(domain.PermConfigManage)).Get("/fees/providers", h.Admin.Fees.ListProviders)
	r.With(requirePerm(domain.PermConfigManage)).Put("/fees/providers/{id}", h.Admin.Fees.UpdateProvider)

	// System accounts (capital pools)
	r.With(requirePerm(domain.PermSystemAccounts)).Get("/system/accounts", h.Admin.SystemAccounts.Get)
}

// --- Helpers ---

func perm(deps *Deps, p domain.BusinessPermission) func(http.Handler) http.Handler {
	return middleware.RequireBusinessPermission(deps.Handler.PermissionService, p)
}

func contextLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqLog := log.With(
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", middleware.RequestIDFromContext(r.Context())),
			)
			ctx := nlog.WithContext(r.Context(), reqLog)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
