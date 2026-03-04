package personal

import (
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/auth"
	"github.com/vonmutinda/neo/internal/services/balances"
	"github.com/vonmutinda/neo/internal/services/beneficiary"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/lending"
	"github.com/vonmutinda/neo/internal/services/onboarding"
	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/services/payment_requests"
	"github.com/vonmutinda/neo/internal/services/payments"
	"github.com/vonmutinda/neo/internal/services/pots"
	"github.com/vonmutinda/neo/internal/services/pricing"
	"github.com/vonmutinda/neo/internal/services/recipient"
	"github.com/vonmutinda/neo/internal/services/wallet"
)

type Handlers struct {
	Auth           *AuthHandler
	Onboarding     *OnboardingHandler
	Wallets        *WalletHandler
	Transfers      *TransferHandler
	Loans          *LoanHandler
	Overdraft      *OverdraftHandler
	Cards          *CardHandler
	Balances       *BalanceHandler
	Pots           *PotHandler
	Convert        *ConvertHandler
	Me             *MeHandler
	Beneficiaries  *BeneficiaryHandler
	Recipients      *RecipientHandler
	PaymentRequests *PaymentRequestHandler
	SpendWaterfall  *SpendWaterfallHandler
	Categories     *CategoryHandler
	Fees           *FeeHandler
	Resolve        *ResolveHandler
}

func NewHandlers(
	authSvc *auth.Service,
	onboardingSvc *onboarding.Service,
	paymentsSvc *payments.Service,
	disbursementSvc *lending.DisbursementService,
	loanQuerySvc *lending.QueryService,
	overdraftSvc *overdraft.Service,
	userRepo repository.UserRepository,
	cardRepo repository.CardRepository,
	currencyBalanceRepo repository.CurrencyBalanceRepository,
	balancesSvc *balances.Service,
	walletSvc *wallet.Service,
	potsSvc *pots.Service,
	convertSvc *convert.Service,
	beneficiarySvc *beneficiary.Service,
	recipientSvc *recipient.Service,
	paymentRequestSvc *payment_requests.Service,
	labelRepo repository.TransactionLabelRepository,
	pricingSvc *pricing.Service,
) *Handlers {
	return &Handlers{
		Auth:           NewAuthHandler(authSvc),
		Onboarding:     NewOnboardingHandler(onboardingSvc),
		Wallets:        NewWalletHandler(walletSvc),
		Transfers:      NewTransferHandler(paymentsSvc),
		Loans:          NewLoanHandler(disbursementSvc, loanQuerySvc),
		Overdraft:      NewOverdraftHandler(overdraftSvc),
		Cards:          NewCardHandler(cardRepo),
		Balances:       NewBalanceHandler(balancesSvc),
		Pots:           NewPotHandler(potsSvc),
		Convert:        NewConvertHandler(convertSvc),
		Me:             NewMeHandler(userRepo),
		Beneficiaries:  NewBeneficiaryHandler(beneficiarySvc),
		Recipients:      NewRecipientHandler(recipientSvc),
		PaymentRequests: NewPaymentRequestHandler(paymentRequestSvc),
		SpendWaterfall:  NewSpendWaterfallHandler(userRepo, currencyBalanceRepo),
		Categories:     NewCategoryHandler(labelRepo),
		Fees:           NewFeeHandler(pricingSvc),
		Resolve:        NewResolveHandler(userRepo),
	}
}
