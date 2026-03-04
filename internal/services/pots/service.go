package pots

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/google/uuid"
)

type Service struct {
	pots     repository.PotRepository
	balances repository.CurrencyBalanceRepository
	users    repository.UserRepository
	ledger   ledger.Client
	chart    *ledger.Chart
	receipts repository.TransactionReceiptRepository
}

func NewService(
	pots repository.PotRepository,
	balances repository.CurrencyBalanceRepository,
	users repository.UserRepository,
	ledgerClient ledger.Client,
	chart *ledger.Chart,
	receipts repository.TransactionReceiptRepository,
) *Service {
	return &Service{
		pots:     pots,
		balances: balances,
		users:    users,
		ledger:   ledgerClient,
		chart:    chart,
		receipts: receipts,
	}
}

func (s *Service) CreatePot(ctx context.Context, userID string, req *CreatePotRequest) (*domain.Pot, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	req.Name = strings.TrimSpace(req.Name)

	if _, err := s.balances.GetByUserAndCurrency(ctx, userID, req.CurrencyCode); err != nil {
		return nil, fmt.Errorf("currency not active: %w", domain.ErrCurrencyNotActive)
	}

	pot := &domain.Pot{
		UserID:       userID,
		Name:         req.Name,
		CurrencyCode: req.CurrencyCode,
		TargetCents:  req.TargetCents,
		Emoji:        req.Emoji,
	}

	if err := s.pots.Create(ctx, pot); err != nil {
		return nil, fmt.Errorf("creating pot: %w", err)
	}

	pot.BalanceCents = 0
	pot.Display = money.Display(0, req.CurrencyCode)
	return pot, nil
}

func (s *Service) UpdatePot(ctx context.Context, userID, potID string, req *UpdatePotRequest) (*domain.Pot, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	pot, err := s.getPotForUser(ctx, userID, potID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		pot.Name = strings.TrimSpace(*req.Name)
	}
	if req.TargetCents != nil {
		pot.TargetCents = req.TargetCents
	}
	if req.Emoji != nil {
		pot.Emoji = req.Emoji
	}

	if err := s.pots.Update(ctx, pot); err != nil {
		return nil, err
	}

	return s.enrichPot(ctx, pot)
}

// ArchivePot archives a pot. If the pot has remaining funds, they are
// automatically transferred back to the user's main balance first.
func (s *Service) ArchivePot(ctx context.Context, userID, potID string) (returnedCents int64, currency string, err error) {
	pot, err := s.getPotForUser(ctx, userID, potID)
	if err != nil {
		return 0, "", err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return 0, "", fmt.Errorf("looking up user: %w", err)
	}

	asset := money.FormatAsset(pot.CurrencyCode)
	bal, err := s.ledger.GetPotBalance(ctx, user.LedgerWalletID, pot.ID, asset)
	if err != nil {
		return 0, "", fmt.Errorf("checking pot balance: %w", err)
	}

	cents := bal.Int64()
	if cents > 0 {
		ik := uuid.NewString()
		if err := s.ledger.TransferFromPot(ctx, ik, user.LedgerWalletID, pot.ID, cents, asset); err != nil {
			return 0, "", fmt.Errorf("auto-withdrawing pot funds: %w", err)
		}
	}

	if err := s.pots.Archive(ctx, potID, userID); err != nil {
		return 0, "", err
	}

	return cents, pot.CurrencyCode, nil
}

func (s *Service) AddToPot(ctx context.Context, userID, potID string, req *PotTransferRequest) (*domain.Pot, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	pot, err := s.getPotForUser(ctx, userID, potID)
	if err != nil {
		return nil, err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	asset := money.FormatAsset(pot.CurrencyCode)
	mainBalance, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, asset)
	if err != nil {
		return nil, fmt.Errorf("checking main balance: %w", err)
	}
	if mainBalance.Int64() < req.AmountCents {
		return nil, domain.ErrInsufficientFunds
	}

	ik := uuid.NewString()
	if err := s.ledger.TransferToPot(ctx, ik, user.LedgerWalletID, pot.ID, req.AmountCents, asset); err != nil {
		return nil, fmt.Errorf("depositing to pot: %w", err)
	}

	if s.receipts != nil {
		meta := domain.PotTransferMetadata{PotID: pot.ID, PotName: pot.Name}
		metaBytes, _ := json.Marshal(meta)
		rawMeta := json.RawMessage(metaBytes)
		_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
			UserID:              userID,
			LedgerTransactionID: ik,
			IdempotencyKey:     &ik,
			Type:               domain.ReceiptPotDeposit,
			Status:             domain.ReceiptCompleted,
			AmountCents:        req.AmountCents,
			Currency:           pot.CurrencyCode,
			Narration:          strPtr("Added to " + pot.Name),
			Metadata:           &rawMeta,
		})
	}

	return s.enrichPot(ctx, pot)
}

func (s *Service) WithdrawFromPot(ctx context.Context, userID, potID string, req *PotTransferRequest) (*domain.Pot, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	pot, err := s.getPotForUser(ctx, userID, potID)
	if err != nil {
		return nil, err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	asset := money.FormatAsset(pot.CurrencyCode)
	potBalance, err := s.ledger.GetPotBalance(ctx, user.LedgerWalletID, pot.ID, asset)
	if err != nil {
		return nil, fmt.Errorf("checking pot balance: %w", err)
	}
	if potBalance.Int64() < req.AmountCents {
		return nil, domain.ErrInsufficientFunds
	}

	ik := uuid.NewString()
	if err := s.ledger.TransferFromPot(ctx, ik, user.LedgerWalletID, pot.ID, req.AmountCents, asset); err != nil {
		return nil, fmt.Errorf("withdrawing from pot: %w", err)
	}

	if s.receipts != nil {
		meta := domain.PotTransferMetadata{PotID: pot.ID, PotName: pot.Name}
		metaBytes, _ := json.Marshal(meta)
		rawMeta := json.RawMessage(metaBytes)
		_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
			UserID:              userID,
			LedgerTransactionID: ik,
			IdempotencyKey:     &ik,
			Type:               domain.ReceiptPotWithdraw,
			Status:             domain.ReceiptCompleted,
			AmountCents:        req.AmountCents,
			Currency:           pot.CurrencyCode,
			Narration:          strPtr("Withdrawn from " + pot.Name),
			Metadata:           &rawMeta,
		})
	}

	return s.enrichPot(ctx, pot)
}

func strPtr(s string) *string { return &s }

func (s *Service) ListPots(ctx context.Context, userID string) ([]domain.Pot, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	list, err := s.pots.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := range list {
		asset := money.FormatAsset(list[i].CurrencyCode)
		bal, err := s.ledger.GetPotBalance(ctx, user.LedgerWalletID, list[i].ID, asset)
		if err != nil {
			return nil, fmt.Errorf("getting pot balance: %w", err)
		}
		list[i].BalanceCents = bal.Int64()
		list[i].Display = money.Display(list[i].BalanceCents, list[i].CurrencyCode)
		if list[i].TargetCents != nil && *list[i].TargetCents > 0 {
			list[i].ProgressPercent = float64(list[i].BalanceCents) / float64(*list[i].TargetCents) * 100
		}
	}

	return list, nil
}

func (s *Service) GetPot(ctx context.Context, userID, potID string) (*domain.Pot, error) {
	pot, err := s.getPotForUser(ctx, userID, potID)
	if err != nil {
		return nil, err
	}
	return s.enrichPot(ctx, pot)
}

func (s *Service) GetPotTransactions(ctx context.Context, userID, potID string, limit int) ([]ledger.Transaction, error) {
	pot, err := s.getPotForUser(ctx, userID, potID)
	if err != nil {
		return nil, err
	}

	user, err := s.users.GetByID(ctx, pot.UserID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	potAccount := s.chart.PotAccount(user.LedgerWalletID, pot.ID)
	txs, err := s.ledger.GetAccountHistory(ctx, potAccount, limit)
	if err != nil {
		return nil, fmt.Errorf("getting pot transactions: %w", err)
	}

	return txs, nil
}

func (s *Service) getPotForUser(ctx context.Context, userID, potID string) (*domain.Pot, error) {
	pot, err := s.pots.GetByID(ctx, potID)
	if err != nil {
		return nil, err
	}
	if pot.UserID != userID {
		return nil, domain.ErrPotNotFound
	}
	if pot.IsArchived {
		return nil, domain.ErrPotArchived
	}
	return pot, nil
}

func (s *Service) enrichPot(ctx context.Context, pot *domain.Pot) (*domain.Pot, error) {
	user, err := s.users.GetByID(ctx, pot.UserID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	asset := money.FormatAsset(pot.CurrencyCode)
	bal, err := s.ledger.GetPotBalance(ctx, user.LedgerWalletID, pot.ID, asset)
	if err != nil {
		return nil, fmt.Errorf("getting pot balance: %w", err)
	}
	pot.BalanceCents = bal.Int64()
	pot.Display = money.Display(pot.BalanceCents, pot.CurrencyCode)
	if pot.TargetCents != nil && *pot.TargetCents > 0 {
		pot.ProgressPercent = float64(pot.BalanceCents) / float64(*pot.TargetCents) * 100
	}
	return pot, nil
}
