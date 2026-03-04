package balances

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/pkg/iban"
	"github.com/vonmutinda/neo/pkg/money"
)

type Service struct {
	balances       repository.CurrencyBalanceRepository
	accountDetails repository.AccountDetailsRepository
	users          repository.UserRepository
	ledger         ledger.Client
	regulatory     *regulatory.Service
}

func NewService(
	balances repository.CurrencyBalanceRepository,
	accountDetails repository.AccountDetailsRepository,
	users repository.UserRepository,
	ledgerClient ledger.Client,
	regulatorySvc *regulatory.Service,
) *Service {
	return &Service{
		balances:       balances,
		accountDetails: accountDetails,
		users:          users,
		ledger:         ledgerClient,
		regulatory:     regulatorySvc,
	}
}

// CreateCurrencyBalance activates a new currency for the user.
// If a soft-deleted balance exists, it is reactivated instead.
func (s *Service) CreateCurrencyBalance(ctx context.Context, userID string, req *CreateBalanceRequest) (*domain.CurrencyBalanceWithDetails, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	currencyCode := req.CurrencyCode

	cur, err := money.LookupCurrency(currencyCode)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	// For non-ETB currencies, verify user has sufficient KYC level (Clause 4/5)
	if currencyCode != money.CurrencyETB {
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("looking up user: %w", err)
		}
		if user.KYCLevel < domain.KYCVerified {
			return nil, domain.ErrKYCInsufficientForFX
		}
	}

	existing, err := s.balances.GetByUserAndCurrency(ctx, userID, currencyCode)
	if err == nil && existing != nil {
		return nil, domain.ErrBalanceAlreadyActive
	}

	// Check for soft-deleted balance that can be reactivated.
	softDeleted, _ := s.balances.GetSoftDeleted(ctx, userID, currencyCode)
	var bal *domain.CurrencyBalance
	if softDeleted != nil {
		bal, err = s.balances.Reactivate(ctx, userID, currencyCode)
		if err != nil {
			return nil, fmt.Errorf("reactivating balance: %w", err)
		}
	} else {
		bal = &domain.CurrencyBalance{
			UserID:       userID,
			CurrencyCode: currencyCode,
			IsPrimary:    false,
			FXSource:     req.FXSource,
		}
		if err := s.balances.Create(ctx, bal); err != nil {
			return nil, fmt.Errorf("creating currency balance: %w", err)
		}
	}

	// Generate account details for eligible currencies.
	var details *domain.AccountDetails
	if cur.HasAccountDetails {
		details, err = s.generateAccountDetails(ctx, bal.ID, currencyCode)
		if err != nil {
			return nil, fmt.Errorf("generating account details: %w", err)
		}
	}

	return &domain.CurrencyBalanceWithDetails{
		CurrencyBalance: *bal,
		AccountDetails:  details,
		BalanceCents:    0,
		Display:         money.Display(0, currencyCode),
	}, nil
}

// DeleteCurrencyBalance soft-deletes a currency balance if its Formance balance is zero.
func (s *Service) DeleteCurrencyBalance(ctx context.Context, userID, currencyCode string) error {
	bal, err := s.balances.GetByUserAndCurrency(ctx, userID, currencyCode)
	if err != nil {
		return err
	}

	if bal.IsPrimary {
		return domain.ErrCannotDeletePrimary
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("looking up user: %w", err)
	}

	asset := money.FormatAsset(currencyCode)
	formanceBalance, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, asset)
	if err != nil {
		return fmt.Errorf("checking formance balance: %w", err)
	}
	if formanceBalance.Int64() != 0 {
		return domain.ErrBalanceNotEmpty
	}

	return s.balances.SoftDelete(ctx, userID, currencyCode)
}

// ListActiveCurrencyBalances returns all active balances enriched with live
// Formance balances and account details.
func (s *Service) ListActiveCurrencyBalances(ctx context.Context, userID string) ([]domain.CurrencyBalanceWithDetails, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	active, err := s.balances.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing active balances: %w", err)
	}

	// Build asset list and fetch all Formance balances in one call.
	assets := make([]string, len(active))
	for i, b := range active {
		assets[i] = money.FormatAsset(b.CurrencyCode)
	}
	formanceBalances, err := s.ledger.GetMultiCurrencyBalances(ctx, user.LedgerWalletID, assets)
	if err != nil {
		return nil, fmt.Errorf("getting formance balances: %w", err)
	}

	result := make([]domain.CurrencyBalanceWithDetails, 0, len(active))
	for _, b := range active {
		asset := money.FormatAsset(b.CurrencyCode)
		var cents int64
		if bal, ok := formanceBalances[asset]; ok && bal != nil {
			cents = bal.Int64()
		}

		details, _ := s.accountDetails.GetByCurrencyBalanceID(ctx, b.ID)

		result = append(result, domain.CurrencyBalanceWithDetails{
			CurrencyBalance: b,
			AccountDetails:  details,
			BalanceCents:    cents,
			Display:         money.Display(cents, b.CurrencyCode),
		})
	}

	return result, nil
}

// CreateDefaultBalance creates the primary ETB balance for a new user.
// Called during registration.
func (s *Service) CreateDefaultBalance(ctx context.Context, userID string) (*domain.CurrencyBalance, *domain.AccountDetails, error) {
	bal := &domain.CurrencyBalance{
		UserID:       userID,
		CurrencyCode: money.CurrencyETB,
		IsPrimary:    true,
	}
	if err := s.balances.Create(ctx, bal); err != nil {
		return nil, nil, fmt.Errorf("creating default ETB balance: %w", err)
	}

	details, err := s.generateAccountDetails(ctx, bal.ID, money.CurrencyETB)
	if err != nil {
		return nil, nil, fmt.Errorf("generating ETB account details: %w", err)
	}

	return bal, details, nil
}

func (s *Service) generateAccountDetails(ctx context.Context, balanceID, currencyCode string) (*domain.AccountDetails, error) {
	// Check if details already exist (for reactivated balances).
	existing, _ := s.accountDetails.GetByCurrencyBalanceID(ctx, balanceID)
	if existing != nil {
		return existing, nil
	}

	seq, err := s.accountDetails.NextAccountNumber(ctx)
	if err != nil {
		return nil, err
	}

	accountNumber := iban.FormatAccountNumber(seq)
	ibanStr := iban.Generate(accountNumber, currencyCode)

	details := &domain.AccountDetails{
		CurrencyBalanceID: balanceID,
		IBAN:              ibanStr,
		AccountNumber:     accountNumber,
		BankName:          "Neo Bank Ethiopia",
		SwiftCode:         "NEOBETET",
	}

	if currencyCode == money.CurrencyUSD {
		rn := "021000021"
		details.RoutingNumber = &rn
	}

	if err := s.accountDetails.Create(ctx, details); err != nil {
		return nil, fmt.Errorf("inserting account details: %w", err)
	}

	return details, nil
}
