package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/gateway/ethswitch"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	recipientsvc "github.com/vonmutinda/neo/internal/services/recipient"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	nlog "github.com/vonmutinda/neo/pkg/logger"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/google/uuid"
)

// PricingCalculator is the subset of pricing.Service needed by payments.
// Defined as an interface to avoid circular imports.
type PricingCalculator interface {
	CalculateFee(ctx context.Context, txType domain.TransactionType, amountCents int64, currency string, channel *string) (*domain.FeeBreakdown, error)
}

// RecipientSaver is the subset of recipient.Service needed by payments.
type RecipientSaver interface {
	SaveFromTransferFireAndForget(ctx context.Context, ownerUserID string, cp recipientsvc.TransferCounterparty, currency string)
}

// OverdraftCover is the subset of overdraft.Service needed to cover ETB shortfalls and repay on inflow.
// Users must explicitly opt-in via POST /v1/overdraft/opt-in before overdraft can cover a shortfall.
type OverdraftCover interface {
	UseCover(ctx context.Context, userID, walletID, idempotencyKey string, shortfallCents int64, asset string) error
	AutoRepayOnInflow(ctx context.Context, userID, walletID, idempotencyKey string, creditAmountCents int64) (repayCents int64, err error)
}

// Service orchestrates transfer flows:
//   - Outbound: Hold → Transmit via EthSwitch → Settle/Void (Two-Phase Commit)
//   - Inbound P2P: Direct wallet-to-wallet within the neobank (same or cross-currency)
type Service struct {
	users      repository.UserRepository
	receipts   repository.TransactionReceiptRepository
	audit      repository.AuditRepository
	ledger     ledger.Client
	ethswitch  ethswitch.Client
	chart      *ledger.Chart
	regulatory *regulatory.Service
	totals      repository.TransferTotalsRepository
	pricing     PricingCalculator
	recipients  RecipientSaver
	overdraft   OverdraftCover
}

func NewService(
	users repository.UserRepository,
	receipts repository.TransactionReceiptRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
	ethswitchClient ethswitch.Client,
	chart *ledger.Chart,
	regulatorySvc *regulatory.Service,
	totals repository.TransferTotalsRepository,
	pricing PricingCalculator,
	recipients RecipientSaver,
	overdraft OverdraftCover,
) *Service {
	return &Service{
		users:      users,
		receipts:   receipts,
		audit:      audit,
		ledger:     ledgerClient,
		ethswitch:  ethswitchClient,
		chart:      chart,
		regulatory: regulatorySvc,
		totals:     totals,
		pricing:    pricing,
		recipients: recipients,
		overdraft:  overdraft,
	}
}

// overdraftErrOrInsufficientFunds returns the error unchanged if it is a known overdraft sentinel (so API returns 422 with specific message), else ErrInsufficientFunds.
func overdraftErrOrInsufficientFunds(err error) error {
	if errors.Is(err, domain.ErrOverdraftNotActive) ||
		errors.Is(err, domain.ErrOverdraftNotEligible) ||
		errors.Is(err, domain.ErrOverdraftLimitExceeded) ||
		errors.Is(err, domain.ErrOverdraftETBOnly) {
		return err
	}
	return domain.ErrInsufficientFunds
}

func setAuditIP(ctx context.Context, e *domain.AuditEntry) {
	if e.ActorType != "user" {
		return
	}
	if ipStr := middleware.GetClientIP(ctx); ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil {
			e.IPAddress = ip
		}
	}
}

// ProcessOutboundTransfer executes a full Hold → Transmit → Settle/Void cycle
// for external transfers via EthSwitch. Supports multi-currency.
func (s *Service) ProcessOutboundTransfer(
	ctx context.Context,
	userID string,
	req *OutboundTransferRequest,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("looking up user: %w", err)
	}
	if user.IsFrozen {
		return domain.ErrUserFrozen
	}

	// Regulatory checks
	if s.regulatory != nil {
		if err := s.regulatory.CheckTransferAllowed(ctx, &regulatory.TransferCheckRequest{
			UserID:      userID,
			User:        user,
			Direction:   "outbound",
			AmountCents: req.AmountCents,
			Currency:    req.Currency,
			Purpose:     req.Purpose,
			Destination: req.Destination,
		}); err != nil {
			_ = s.audit.Log(ctx, &domain.AuditEntry{
				Action:       domain.AuditTransferBlocked,
				ActorType:    "system",
				ActorID:      &userID,
				ResourceType: "transfer",
				ResourceID:   "outbound",
			})
			return err
		}
	}

	idempotencyKey := uuid.NewString()
	asset := money.FormatAsset(req.Currency)

	// Phase 1: Hold funds in Formance (wallet → transit).
	holdID, err := s.ledger.HoldFunds(ctx, idempotencyKey+"-hold", user.LedgerWalletID, s.chart.TransitEthSwitch(), req.AmountCents, asset)
	if err != nil {
		return fmt.Errorf("holding funds: %w", err)
	}

	initEntry := &domain.AuditEntry{
		Action:       domain.AuditTransferInitiated,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "transfer",
		ResourceID:   holdID,
	}
	setAuditIP(ctx, initEntry)
	_ = s.audit.Log(ctx, initEntry)

	// Phase 2: Transmit to EthSwitch.
	ethswitchReq := ethswitch.TransferRequest{
		IdempotencyKey:     idempotencyKey,
		SourceInstitution:  "NEOBANK",
		DestInstitution:    req.DestInstitution,
		SourceAccount:      user.LedgerWalletID,
		DestinationAccount: req.DestPhone.E164(),
		AmountCents:        req.AmountCents,
		Currency:           req.Currency,
		Narration:          req.Narration,
	}

	resp, err := s.ethswitch.InitiateTransfer(ctx, ethswitchReq)

	// Phase 3: Settle, Void, or queue for background check.
	if err != nil {
		_ = s.audit.Log(ctx, &domain.AuditEntry{
			Action:       domain.AuditTransferFailed,
			ActorType:    "system",
			ResourceType: "transfer",
			ResourceID:   holdID,
		})
		return fmt.Errorf("transfer pending verification (hold %s): %w", holdID, domain.ErrEthSwitchTimeout)
	}

	switch resp.Status {
	case "SUCCESS":
		if err := retryLedgerOp(ctx, func() error {
			return s.ledger.SettleHold(ctx, idempotencyKey+"-settle", holdID)
		}); err != nil {
			return fmt.Errorf("settling hold after EthSwitch success: %w", err)
		}

		var purpose *domain.TransferPurpose
		if req.Purpose != "" {
			p := domain.TransferPurpose(req.Purpose)
			purpose = &p
		}
		var beneficiaryID *string
		if req.BeneficiaryID != "" {
			beneficiaryID = &req.BeneficiaryID
		}

		_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
			UserID:              userID,
			LedgerTransactionID: holdID,
			EthSwitchReference:  &resp.EthSwitchReference,
			IdempotencyKey:      &idempotencyKey,
			Type:                domain.ReceiptEthSwitchOut,
			Status:              domain.ReceiptCompleted,
			AmountCents:         req.AmountCents,
			Currency:            req.Currency,
			CounterpartyPhone:   &req.DestPhone,
			Narration:           &req.Narration,
			Purpose:             purpose,
			BeneficiaryID:       beneficiaryID,
		})
		_ = s.audit.Log(ctx, &domain.AuditEntry{
			Action:       domain.AuditTransferSettled,
			ActorType:    "system",
			ResourceType: "transfer",
			ResourceID:   holdID,
		})

		// Update daily totals on success
		if s.totals != nil {
			_ = s.totals.Increment(ctx, userID, req.Currency, "outbound", req.AmountCents)
		}

		// Auto-save recipient (fire-and-forget)
		if s.recipients != nil {
			s.recipients.SaveFromTransferFireAndForget(ctx, userID, recipientsvc.TransferCounterparty{
				Type:            domain.RecipientBankAccount,
				DisplayName:     req.DestPhone.E164(),
				InstitutionCode: req.DestInstitution,
				AccountNumber:   req.DestPhone.E164(),
			}, req.Currency)
		}

		return nil

	default:
		if err := retryLedgerOp(ctx, func() error {
			return s.ledger.VoidHold(ctx, idempotencyKey+"-void", holdID)
		}); err != nil {
			return fmt.Errorf("voiding hold after EthSwitch failure: %w", err)
		}
		_ = s.audit.Log(ctx, &domain.AuditEntry{
			Action:       domain.AuditTransferVoided,
			ActorType:    "system",
			ResourceType: "transfer",
			ResourceID:   holdID,
		})
		return fmt.Errorf("transfer failed: %s", resp.ErrorMessage)
	}
}

// ProcessInboundTransfer handles a wallet-to-wallet transfer between two users
// of the neobank. This is an atomic operation within Formance:
//   - Debits the sender's wallet
//   - Credits the recipient's wallet
//
// Both wallets must exist. The transfer is in a single currency.
func (s *Service) ProcessInboundTransfer(
	ctx context.Context,
	senderID string,
	req *InboundTransferRequest,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	sender, err := s.users.GetByID(ctx, senderID)
	if err != nil {
		return fmt.Errorf("looking up sender: %w", err)
	}
	if sender.IsFrozen {
		return fmt.Errorf("sender: %w", domain.ErrUserFrozen)
	}

	// Regulatory checks
	if s.regulatory != nil {
		if err := s.regulatory.CheckTransferAllowed(ctx, &regulatory.TransferCheckRequest{
			UserID:      senderID,
			User:        sender,
			Direction:   "p2p",
			AmountCents: req.AmountCents,
			Currency:    req.Currency,
			Purpose:     req.Purpose,
			Destination: "domestic",
		}); err != nil {
			_ = s.audit.Log(ctx, &domain.AuditEntry{
				Action:       domain.AuditTransferBlocked,
				ActorType:    "system",
				ActorID:      &senderID,
				ResourceType: "transfer",
				ResourceID:   "p2p",
			})
			return err
		}
	}

	recipient, err := s.resolveRecipient(ctx, req)
	if err != nil {
		return fmt.Errorf("recipient not found: %w", domain.ErrUserNotFound)
	}
	if recipient.IsFrozen {
		return fmt.Errorf("recipient: %w", domain.ErrUserFrozen)
	}

	if sender.ID == recipient.ID {
		return fmt.Errorf("cannot transfer to yourself: %w", domain.ErrInvalidInput)
	}

	asset := money.FormatAsset(req.Currency)
	idempotencyKey := uuid.NewString()
	balance, err := s.ledger.GetWalletBalance(ctx, sender.LedgerWalletID, asset)
	if err != nil {
		return fmt.Errorf("checking sender balance: %w", err)
	}
	if balance.Int64() < req.AmountCents {
		if req.Currency == money.CurrencyETB && s.overdraft != nil {
			shortfall := req.AmountCents - balance.Int64()
			if err := s.overdraft.UseCover(ctx, senderID, sender.LedgerWalletID, idempotencyKey+"-od", shortfall, asset); err != nil {
				return overdraftErrOrInsufficientFunds(err)
			}
		} else {
			return domain.ErrInsufficientFunds
		}
	}

	_, err = s.ledger.DebitWallet(
		ctx,
		idempotencyKey+"-debit",
		sender.LedgerWalletID,
		req.AmountCents,
		asset,
		s.chart.MainAccount(recipient.LedgerWalletID),
		false,
	)
	if err != nil {
		return fmt.Errorf("debiting sender wallet: %w", err)
	}

	recipientPhone := recipient.PhoneNumber
	_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
		UserID:              senderID,
		LedgerTransactionID: idempotencyKey,
		IdempotencyKey:      &idempotencyKey,
		Type:                domain.ReceiptP2PSend,
		Status:              domain.ReceiptCompleted,
		AmountCents:         req.AmountCents,
		Currency:            req.Currency,
		CounterpartyPhone:   &recipientPhone,
		Narration:           &req.Narration,
	})

	senderPhone := sender.PhoneNumber
	var recipientReceiptMeta *json.RawMessage
	if req.Currency == money.CurrencyETB && s.overdraft != nil {
		repayCents, _ := s.overdraft.AutoRepayOnInflow(ctx, recipient.ID, recipient.LedgerWalletID, idempotencyKey+"-od-repay", req.AmountCents)
		if repayCents > 0 {
			meta := domain.InflowOverdraftMetadata{
				TotalInflowCents:          req.AmountCents,
				OverdraftRepaymentCents: repayCents,
				NetInflowCents:           req.AmountCents - repayCents,
			}
			if b, err := json.Marshal(meta); err == nil {
				raw := json.RawMessage(b)
				recipientReceiptMeta = &raw
			}
		}
	}
	rec := &domain.TransactionReceipt{
		UserID:              recipient.ID,
		LedgerTransactionID: idempotencyKey + "-in",
		IdempotencyKey:      &idempotencyKey,
		Type:                domain.ReceiptP2PReceive,
		Status:              domain.ReceiptCompleted,
		AmountCents:         req.AmountCents,
		Currency:            req.Currency,
		CounterpartyPhone:   &senderPhone,
		Narration:           &req.Narration,
		Metadata:            recipientReceiptMeta,
	}
	_ = s.receipts.Create(ctx, rec)

	p2pEntry := &domain.AuditEntry{
		Action:       domain.AuditP2PTransfer,
		ActorType:    "user",
		ActorID:      &senderID,
		ResourceType: "transfer",
		ResourceID:   idempotencyKey,
	}
	setAuditIP(ctx, p2pEntry)
	_ = s.audit.Log(ctx, p2pEntry)

	// Update daily totals on success
	if s.totals != nil {
		_ = s.totals.Increment(ctx, senderID, req.Currency, "p2p", req.AmountCents)
	}

	// Auto-save recipient (fire-and-forget)
	if s.recipients != nil {
		var username string
		if recipient.Username != nil {
			username = *recipient.Username
		}
		s.recipients.SaveFromTransferFireAndForget(ctx, senderID, recipientsvc.TransferCounterparty{
			Type:        domain.RecipientNeoUser,
			DisplayName: recipient.FullName(),
			NeoUserID:   recipient.ID,
			CountryCode: recipient.PhoneNumber.CountryCode,
			Number:      recipient.PhoneNumber.Number,
			Username:    username,
		}, req.Currency)
	}

	return nil
}

// resolveRecipient looks up the transfer recipient by the new Recipient field
// (phone or username) or falls back to the legacy RecipientPhone field.
func (s *Service) resolveRecipient(ctx context.Context, req *InboundTransferRequest) (*domain.User, error) {
	if req.Recipient != "" {
		if p, err := phone.Parse(req.Recipient); err == nil {
			return s.users.GetByPhone(ctx, p)
		}
		return s.users.GetByUsername(ctx, req.Recipient)
	}
	if !req.RecipientPhone.IsZero() {
		return s.users.GetByPhone(ctx, req.RecipientPhone)
	}
	return nil, domain.ErrUserNotFound
}

// resolveBatchRecipient resolves a single recipient identifier which may be a
// user UUID, phone number (E.164), or username.
func (s *Service) resolveBatchRecipient(ctx context.Context, identifier string) (*domain.User, error) {
	if _, err := uuid.Parse(identifier); err == nil {
		return s.users.GetByID(ctx, identifier)
	}
	if p, err := phone.Parse(identifier); err == nil {
		return s.users.GetByPhone(ctx, p)
	}
	return s.users.GetByUsername(ctx, identifier)
}

// ProcessBatchTransfer sends funds from one sender to multiple recipients
// in a single request. Each transfer is an individual ledger debit.
func (s *Service) ProcessBatchTransfer(
	ctx context.Context,
	senderID string,
	req *BatchTransferRequest,
) (*BatchTransferResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	sender, err := s.users.GetByID(ctx, senderID)
	if err != nil {
		return nil, fmt.Errorf("looking up sender: %w", err)
	}
	if sender.IsFrozen {
		return nil, fmt.Errorf("sender: %w", domain.ErrUserFrozen)
	}

	type resolved struct {
		user *domain.User
		item BatchTransferItem
	}
	recipients := make([]resolved, 0, len(req.Items))
	for _, item := range req.Items {
		user, err := s.resolveBatchRecipient(ctx, item.Recipient)
		if err != nil {
			return nil, fmt.Errorf("recipient %q not found: %w", item.Recipient, domain.ErrUserNotFound)
		}
		if user.IsFrozen {
			return nil, fmt.Errorf("recipient %q: %w", item.Recipient, domain.ErrUserFrozen)
		}
		if user.ID == senderID {
			return nil, fmt.Errorf("cannot transfer to yourself: %w", domain.ErrInvalidInput)
		}
		recipients = append(recipients, resolved{user: user, item: item})
	}

	totalCents := req.TotalCents()
	asset := money.FormatAsset(req.Currency)
	batchKey := uuid.NewString()

	balance, err := s.ledger.GetWalletBalance(ctx, sender.LedgerWalletID, asset)
	if err != nil {
		return nil, fmt.Errorf("checking sender balance: %w", err)
	}
	if balance.Int64() < totalCents {
		if req.Currency == money.CurrencyETB && s.overdraft != nil {
			shortfall := totalCents - balance.Int64()
			if err := s.overdraft.UseCover(ctx, senderID, sender.LedgerWalletID, batchKey+"-od", shortfall, asset); err != nil {
				return nil, overdraftErrOrInsufficientFunds(err)
			}
		} else {
			return nil, domain.ErrInsufficientFunds
		}
	}
	metaRecipients := make([]domain.BatchSendRecipient, 0, len(recipients))

	for i, r := range recipients {
		ik := fmt.Sprintf("%s-%d", batchKey, i)
		_, err := s.ledger.DebitWallet(
			ctx,
			ik+"-debit",
			sender.LedgerWalletID,
			r.item.AmountCents,
			asset,
			s.chart.MainAccount(r.user.LedgerWalletID),
			false,
		)
		if err != nil {
			return nil, fmt.Errorf("debiting for recipient %q: %w", r.item.Recipient, err)
		}

		var recMeta *json.RawMessage
		if req.Currency == money.CurrencyETB && s.overdraft != nil {
			repayCents, _ := s.overdraft.AutoRepayOnInflow(ctx, r.user.ID, r.user.LedgerWalletID, ik+"-od-repay", r.item.AmountCents)
			if repayCents > 0 {
				meta := domain.InflowOverdraftMetadata{
					TotalInflowCents:          r.item.AmountCents,
					OverdraftRepaymentCents: repayCents,
					NetInflowCents:           r.item.AmountCents - repayCents,
				}
				if b, err := json.Marshal(meta); err == nil {
					raw := json.RawMessage(b)
					recMeta = &raw
				}
			}
		}
		senderPhone := sender.PhoneNumber
		narration := r.item.Narration
		// idempotency_key column is UUID; ik is "batchKey-N" which is not a valid UUID, so leave nil
		_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
			UserID:              r.user.ID,
			LedgerTransactionID: ik + "-in",
			Type:                domain.ReceiptP2PReceive,
			Status:              domain.ReceiptCompleted,
			AmountCents:         r.item.AmountCents,
			Currency:            req.Currency,
			CounterpartyPhone:   &senderPhone,
			Narration:           &narration,
			Metadata:            recMeta,
		})

		metaRecipients = append(metaRecipients, domain.BatchSendRecipient{
			Name:        r.user.FullName(),
			Phone:       r.user.PhoneNumber.E164(),
			UserID:      r.user.ID,
			AmountCents: r.item.AmountCents,
			Narration:   r.item.Narration,
		})

		if s.recipients != nil {
			var username string
			if r.user.Username != nil {
				username = *r.user.Username
			}
			s.recipients.SaveFromTransferFireAndForget(ctx, senderID, recipientsvc.TransferCounterparty{
				Type:        domain.RecipientNeoUser,
				DisplayName: r.user.FullName(),
				NeoUserID:   r.user.ID,
				CountryCode: r.user.PhoneNumber.CountryCode,
				Number:      r.user.PhoneNumber.Number,
				Username:    username,
			}, req.Currency)
		}
	}

	metaBytes, err := json.Marshal(domain.BatchSendMetadata{Recipients: metaRecipients})
	if err != nil {
		return nil, fmt.Errorf("marshalling batch metadata: %w", err)
	}
	rawMeta := json.RawMessage(metaBytes)

	senderReceipt := &domain.TransactionReceipt{
		UserID:              senderID,
		LedgerTransactionID: batchKey,
		IdempotencyKey:      &batchKey,
		Type:                domain.ReceiptBatchSend,
		Status:              domain.ReceiptCompleted,
		AmountCents:         totalCents,
		Currency:            req.Currency,
		Metadata:            &rawMeta,
	}
	_ = s.receipts.Create(ctx, senderReceipt)

	batchEntry := &domain.AuditEntry{
		Action:       domain.AuditP2PTransfer,
		ActorType:    "user",
		ActorID:      &senderID,
		ResourceType: "transfer",
		ResourceID:   batchKey,
	}
	setAuditIP(ctx, batchEntry)
	_ = s.audit.Log(ctx, batchEntry)

	if s.totals != nil {
		_ = s.totals.Increment(ctx, senderID, req.Currency, "p2p", totalCents)
	}

	return &BatchTransferResponse{
		Status:         "completed",
		ReceiptID:      senderReceipt.ID,
		RecipientCount: len(recipients),
		TotalCents:     totalCents,
		Currency:       req.Currency,
	}, nil
}

// retryLedgerOp retries a ledger operation up to 3 times with exponential
// backoff (1s, 2s, 4s). Used for settle/void which must not be silently lost.
func retryLedgerOp(ctx context.Context, op func() error) error {
	backoff := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error
	for attempt, delay := range backoff {
		if err := op(); err != nil {
			lastErr = err
			nlog.FromContext(ctx).Warn("ledger operation failed, retrying",
				slog.Int("attempt", attempt+1),
				slog.String("error", err.Error()),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return nil
	}
	return lastErr
}
