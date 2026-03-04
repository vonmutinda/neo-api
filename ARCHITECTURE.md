# Ethiopian Neobank -- System Architecture & Directory Layout

## High-Level System Overview

A production-grade Ethiopian Neobank backend engineered for high-volume,
low-friction digital transactions under strict NBE compliance mandates.

**Split-Database Architecture:**
- **PostgreSQL** -- State DB: identity, KYC, idempotency, cards, card authorizations, loans, receipts, reconciliation, audit log, currency balances, account details, pots
- **Formance CE** -- Ledger DB: immutable double-entry accounting for all financial balances

**External Integrations:**
- **EthSwitch (EthioPay-IPS)** -- P2P/W2W transfers via JSON over mTLS; card authorization via ISO 8583 over persistent TCP
- **Fayda** -- Ethiopian National ID (eKYC) via OTP + demographic retrieval
- **NBE** -- National Bank of Ethiopia credit registry (blacklist checks)

---

## Go Monorepo Directory Tree

```text
neo/
├── cmd/
│   ├── api/                                  # Main REST API server (user-facing)
│   │   ├── main.go                           #   Bootstraps chi router, DI, graceful shutdown
│   │   └── handlers.go                       #   Handler aggregation struct + NewHandler factory
│   ├── ethswitch-gateway/                    # Standalone mTLS microservice for EthioPay-IPS
│   │   └── main.go                           #   mTLS listener, ISO 8583 TCP socket, health probes
│   ├── recon-worker/                         # EOD reconciliation cron worker
│   │   └── main.go                           #   robfig/cron scheduler, SFTP ingest, 3-way match
│   └── lending-worker/                       # Lending engine cron worker
│       └── main.go                           #   Trust score recalculation, loan sweep scheduler
│
├── internal/
│   ├── domain/                               # Pure domain models -- zero external dependencies
│   │   ├── user.go                           #   User, KYCLevel, FrozenStatus
│   │   ├── kyc.go                            #   KYCVerification, KYCVerificationStatus
│   │   ├── transaction.go                    #   TransactionReceipt, ReceiptType, ReceiptStatus
│   │   ├── card.go                           #   Card, CardType, CardStatus, SecurityToggles
│   │   ├── card_authorization.go             #   CardAuthorization, AuthStatus (ISO 8583 lifecycle)
│   │   ├── loan.go                           #   Loan, LoanStatus, LoanInstallment
│   │   ├── credit_profile.go                 #   CreditProfile, TrustScore, ApprovedLimit
│   │   ├── currency_balance.go               #   CurrencyBalance, AccountDetails, CurrencyBalanceWithDetails
│   │   ├── pot.go                            #   Pot (savings jar with Formance sub-account)
│   │   ├── convert.go                        #   ConvertRequest, ConvertResponse
│   │   ├── audit.go                          #   AuditEntry, AuditAction (27 actions)
│   │   ├── idempotency.go                    #   IdempotencyRecord, IdempotencyStatus
│   │   ├── reconciliation.go                 #   ReconException, ReconRun, ExceptionType, ExceptionStatus
│   │   ├── credit_profile_test.go            #   Tests for CreditProfile helpers
│   │   ├── telegram.go                       #   TelegramLinkToken (legacy, not extended)
│   │   └── errors.go                         #   Sentinel domain errors + HTTPStatus() mapping
│   │
│   ├── services/                             # Application-layer orchestrators (business rules)
│   │   ├── onboarding/                       #   User registration + Fayda eKYC flow
│   │   │   ├── service.go                    #     RegisterUser, RequestOTP, VerifyOTP
│   │   │   ├── forms.go                      #     Request/response DTOs
│   │   │   └── service_test.go
│   │   ├── payments/                         #   Transfers (Hold → Transmit → Settle/Void)
│   │   │   ├── service.go                    #     ProcessOutboundTransfer, ProcessInboundTransfer
│   │   │   ├── forms.go                      #     Request/response DTOs
│   │   │   └── service_test.go
│   │   ├── card_auth/                        #   ISO 8583 authorization decisioning
│   │   │   └── service.go                    #     AuthorizeTransaction, SettleAuth, ReverseAuth
│   │   ├── lending/                          #   Micro-credit: scoring, disbursement, repayment
│   │   │   ├── scoring.go                    #     CalculateTrustScore (cash-flow velocity model)
│   │   │   ├── disbursement.go               #     DisburseLoan, CheckNBEBlacklist
│   │   │   ├── repayment.go                  #     ProcessRepayment, AutoSweep
│   │   │   ├── query.go                      #     GetEligibility, ListHistory, GetLoanDetail
│   │   │   ├── forms.go                      #     Request/response DTOs
│   │   │   └── query_test.go
│   │   ├── balances/                         #   Multi-currency balance lifecycle
│   │   │   ├── service.go                    #     CreateCurrencyBalance, DeleteCurrencyBalance, ListActive
│   │   │   └── forms.go                      #     CreateBalanceRequest
│   │   ├── pots/                             #   Savings pots (sub-wallets)
│   │   │   ├── service.go                    #     CreatePot, AddToPot, WithdrawFromPot, ArchivePot
│   │   │   └── forms.go                      #     Request/response DTOs
│   │   ├── convert/                          #   FX currency conversion
│   │   │   └── service.go                    #     Convert, GetRate
│   │   └── reconciliation/                   #   EOD 3-way match engine
│   │       └── service.go                    #     RunDailyRecon, IngestClearingFile, MatchEngine
│   │
│   ├── repository/                           # PostgreSQL data access layer (pgx, raw SQL)
│   │   ├── postgres.go                       #   *pgxpool.Pool wrapper, DBTX interface, transaction helper
│   │   ├── users.go                          #   UserRepository interface + pgx implementation
│   │   ├── kyc.go                            #   KYCRepository
│   │   ├── cards.go                          #   CardRepository
│   │   ├── card_authorizations.go            #   CardAuthorizationRepository
│   │   ├── loans.go                          #   LoanRepository (loans + installments + credit_profiles)
│   │   ├── transactions.go                   #   TransactionReceiptRepository
│   │   ├── currency_balances.go              #   CurrencyBalanceRepository (with soft-delete)
│   │   ├── account_details.go               #   AccountDetailsRepository (IBAN, account numbers)
│   │   ├── pots.go                           #   PotRepository
│   │   ├── audit.go                          #   AuditRepository (INSERT-only + ListByResource)
│   │   ├── idempotency.go                    #   IdempotencyRepository (INSERT ... ON CONFLICT)
│   │   ├── reconciliation.go                 #   ReconciliationRepository
│   │   └── telegram_bindings.go              #   TelegramLinkTokenRepository (legacy, not extended)
│   │
│   ├── ledger/                               # Formance Ledger abstraction layer
│   │   ├── client.go                         #   Client interface (wraps Formance SDK)
│   │   ├── formance.go                       #   FormanceClient implementation
│   │   ├── chart.go                          #   Chart of Accounts: {prefix}:wallets:*, {prefix}:transit:*, {prefix}:system:*
│   │   └── scripts.go                        #   Numscript template builders (credit, debit, hold, settle, void)
│   │
│   ├── gateway/                              # External system clients (infrastructure adapters)
│   │   ├── ethswitch/                        #   EthSwitch / EthioPay-IPS integration
│   │   │   ├── client.go                     #     EthSwitchClient interface + mTLS HTTP impl
│   │   │   ├── models.go                     #     TransferRequest, TransferResponse
│   │   │   ├── iso8583/                      #     ISO 8583 card processing subsystem
│   │   │   │   ├── codec.go                  #       Message encode/decode (moov-io/iso8583)
│   │   │   │   ├── router.go                 #       Persistent TCP socket manager + message routing
│   │   │   │   ├── fields.go                 #       DE2 PAN, DE37 RRN, DE39 ResponseCode, etc.
│   │   │   │   └── router_test.go
│   │   │   └── sftp.go                       #     SFTP client for EOD clearing file download
│   │   ├── fayda/                            #   Fayda eKYC National ID integration
│   │   │   ├── client.go                     #     FaydaClient interface + HTTP impl
│   │   │   └── models.go                     #     OTPRequest, AuthRequest, KYCResponse
│   │   └── nbe/                              #   National Bank of Ethiopia credit registry
│   │       └── client.go                     #     NBEClient interface (blacklist check)
│   │
│   ├── transport/                            # HTTP transport layer
│   │   └── http/                             #   REST API (go-chi/chi/v5)
│   │       ├── middleware/                    #     HTTP middleware stack
│   │       │   ├── auth.go                   #       JWT validation + UserIDFromContext
│   │       │   ├── idempotency.go            #       Idempotency-Key enforcement (Postgres-backed)
│   │       │   ├── ratelimit.go              #       Per-user token bucket rate limiting
│   │       │   ├── requestid.go              #       X-Request-ID propagation
│   │       │   ├── recovery.go               #       Panic recovery with structured logging
│   │       │   ├── bodylimit.go              #       1 MB max request body
│   │       │   └── logger.go                 #       Request/response logging (method, path, status, duration)
│   │       └── handlers/                     #     Thin HTTP handlers (decode → service → encode)
│   │           ├── onboarding.go             #       POST /v1/register, POST /v1/kyc/otp, POST /v1/kyc/verify
│   │           ├── transfers.go              #       POST /v1/transfers/outbound, POST /v1/transfers/inbound
│   │           ├── cards.go                  #       GET/PATCH /v1/cards, /v1/cards/{id}/status|limits|toggles
│   │           ├── loans.go                  #       GET /v1/loans/eligibility, POST /v1/loans/apply, GET /v1/loans/{id}
│   │           ├── wallets.go                #       GET /v1/wallets/balance, GET /v1/wallets/summary, GET /v1/wallets/transactions
│   │           ├── balances.go               #       POST/GET/DELETE /v1/balances
│   │           ├── pots.go                   #       CRUD /v1/pots, POST /v1/pots/{id}/add|withdraw
│   │           ├── convert.go               #       POST /v1/convert, GET /v1/convert/rate
│   │           ├── me.go                     #       GET /v1/me
│   │           └── health.go                 #       GET /healthz, GET /readyz
│   │
│   └── config/                               # Configuration loading (ardanlabs/conf, env-based)
│       ├── api.go                            #   Root API config struct + LoadAPI()
│       ├── database.go                       #   PostgreSQL connection settings
│       ├── web.go                            #   HTTP server + JWT + CORS settings
│       ├── formance.go                       #   Formance Ledger client config
│       ├── ethswitch.go                      #   EthSwitch mTLS config
│       ├── fayda.go                          #   Fayda eKYC API config
│       ├── rate_limit.go                     #   Rate limiting config
│       ├── log.go                            #   Log level config
│       ├── telegram.go                       #   Telegram bot config (legacy)
│       └── local.go                          #   .env.local loader
│
├── pkg/                                      # Shared, importable utilities
│   ├── httputil/                             #   Shared HTTP response helpers
│   │   ├── response.go                       #     WriteJSON, WriteError, HandleError, DecodeJSON
│   │   └── pagination.go                     #     ParsePagination helper
│   ├── money/                                #   Currency and monetary helpers
│   │   ├── money.go                          #     Display(), FormatAsset(), LookupCurrency(), SupportedCurrencies
│   │   └── money_test.go
│   ├── logger/                               #   Structured JSON logging (slog)
│   │   └── logger.go                         #     NewLogger, WithContext
│   ├── iban/                                 #   IBAN generation for account details
│   │   ├── generator.go                      #     GenerateIBAN (Ethiopian format)
│   │   └── generator_test.go
│   └── validate/                             #   Input validation helpers
│       └── validate.go
│
├── migrations/                               # Versioned SQL migrations (golang-migrate)
│   ├── 000001_init_users_kyc.up.sql          #   users, kyc_verifications
│   ├── 000001_init_users_kyc.down.sql
│   ├── 000002_idempotency_keys.up.sql        #   idempotency_keys
│   ├── 000002_idempotency_keys.down.sql
│   ├── 000003_transaction_receipts.up.sql    #   transaction_receipts
│   ├── 000003_transaction_receipts.down.sql
│   ├── 000004_cards_authorizations.up.sql    #   cards, card_authorizations
│   ├── 000004_cards_authorizations.down.sql
│   ├── 000005_lending.up.sql                 #   credit_profiles, loans, loan_installments
│   ├── 000005_lending.down.sql
│   ├── 000006_reconciliation.up.sql          #   reconciliation_exceptions, reconciliation_runs
│   ├── 000006_reconciliation.down.sql
│   ├── 000007_telegram_bindings.up.sql       #   telegram_command_log (legacy)
│   ├── 000007_telegram_bindings.down.sql
│   ├── 000008_audit_log.up.sql               #   audit_log
│   ├── 000008_audit_log.down.sql
│   ├── 000009_currency_balances_and_pots.up.sql   #   currency_balances, account_details, pots
│   └── 000009_currency_balances_and_pots.down.sql
│
├── deploy/                                   # Deployment manifests
│   ├── docker-compose.yml                    #   Local dev: Postgres + Formance
│   └── Dockerfile                            #   Multi-stage Go build
│
├── tools/
│   └── insomnia-collection.json              #   API testing collection
│
├── go.mod
├── go.sum
├── Makefile                                  # build, test, lint, migrate, docker, dev targets
└── README.md
```

---

## Dependency Flow (Strict Inward)

```text
transport/  →  services/  →  domain/
    ↓              ↓
handlers     repository/  (PostgreSQL)
middleware   ledger/      (Formance)
             gateway/     (EthSwitch, Fayda, NBE)
```

**Rules:**
1. `domain/` has ZERO imports from any other internal package.
2. `services/` depends on `domain/` and accepts repository/ledger/gateway interfaces via constructor injection.
3. `repository/`, `ledger/`, `gateway/` implement interfaces defined in or consumed by `services/`.
4. `transport/` is a thin layer: decode HTTP → call service → encode response.
5. `cmd/` wires everything together (DI composition root).
6. `pkg/` is importable by any layer.

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| PostgreSQL for state, Formance for money | Prevents financial bugs from raw SQL UPDATEs; Formance guarantees double-entry invariants |
| Two-Phase Commit (Hold → Settle/Void) | External network calls (EthSwitch) can time out; funds are safely parked in transit accounts |
| Idempotency via `INSERT ... ON CONFLICT` | Atomic lock acquisition; prevents double-spending on network retries |
| Separate `cmd/` binaries | EthSwitch gateway needs isolated VPC/mTLS; recon/lending workers are cron-scheduled |
| ISO 8583 over persistent TCP | Card authorization requires sub-100ms latency; SmartVista speaks ISO 8583 natively |
| Numscript template builders | Auditable, version-controlled money movement scripts; no string interpolation in hot paths |
| Interface-driven design + DI | Every external dependency is mockable; enables comprehensive unit testing |
| `go-chi/chi/v5` router | Lightweight, stdlib-compatible, middleware-friendly HTTP router |
| `ardanlabs/conf/v3` config | Env-based configuration with `NEO` prefix, supports `.env.local` files |
| Domain errors with `HTTPStatus()` | Centralized error → HTTP status mapping in `domain/errors.go`; handlers use `pkg/httputil.HandleError` |

---

## Formance Chart of Accounts

All account addresses use a configurable prefix (default: `neo`). Defined in
`internal/ledger/chart.go`.

| Account Pattern | Method | Purpose |
|---|---|---|
| `{prefix}:wallets:{walletID}:main` | `MainAccount()` | User's primary wallet balance |
| `{prefix}:wallets:{walletID}:{balanceName}` | `BalanceAccount()` | Named sub-balance |
| `{prefix}:wallets:{walletID}:pot:{potID}` | `PotAccount()` | Pot (savings jar) sub-wallet |
| `{prefix}:wallets:holds:{holdID}` | `HoldAccount()` | Transit hold account |
| `{prefix}:transit:ethswitch_out` | `TransitEthSwitch()` | Pending outbound EthSwitch transfers |
| `{prefix}:transit:card_auth` | `TransitCardAuth()` | Pending card authorizations |
| `{prefix}:transit:p2p` | `TransitP2P()` | Pending internal P2P transfers |
| `{prefix}:system:loan_capital` | `SystemLoanCapital()` | Loan disbursement pool |
| `{prefix}:system:fees` | `SystemFees()` | Platform fee collection |
| `{prefix}:system:interest` | `SystemInterest()` | Loan interest/facilitation fees |
| `{prefix}:system:fx` | `SystemFX()` | FX conversion pool |
| `world` | `World()` | Formance infinite source (external inflows) |
