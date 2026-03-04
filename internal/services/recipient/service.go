package recipient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	nlog "github.com/vonmutinda/neo/pkg/logger"
	"github.com/vonmutinda/neo/pkg/phone"
)

// Service implements recipient management business logic.
type Service struct {
	recipients repository.RecipientRepository
	users      repository.UserRepository
	audit      repository.AuditRepository
}

func NewService(
	recipients repository.RecipientRepository,
	users repository.UserRepository,
	audit repository.AuditRepository,
) *Service {
	return &Service{
		recipients: recipients,
		users:      users,
		audit:      audit,
	}
}

// CreateRequest carries the parameters for explicitly adding a recipient.
type CreateRequest struct {
	Type            domain.RecipientType
	Identifier      string
	InstitutionCode string
	AccountNumber   string
	DisplayName     string
}

// Create adds a new recipient for the given user. For neo_user recipients the
// identifier is resolved as a phone number or username; for bank_account
// recipients the institution is validated against the static bank directory.
func (s *Service) Create(ctx context.Context, ownerUserID string, req CreateRequest) (*domain.Recipient, error) {
	r := &domain.Recipient{
		OwnerUserID: ownerUserID,
		Type:        req.Type,
		DisplayName: req.DisplayName,
	}

	switch req.Type {
	case domain.RecipientNeoUser:
		user, err := resolveUser(ctx, s.users, req.Identifier)
		if err != nil {
			return nil, err
		}
		if user.ID == ownerUserID {
			return nil, domain.ErrSelfRecipient
		}
		r.NeoUserID = &user.ID
		cc := user.PhoneNumber.CountryCode
		num := user.PhoneNumber.Number
		r.CountryCode = &cc
		r.Number = &num
		if user.Username != nil {
			r.Username = user.Username
		}
		if r.DisplayName == "" {
			r.DisplayName = user.FullName()
		}

	case domain.RecipientBankAccount:
		bank := domain.LookupBank(req.InstitutionCode)
		if bank == nil {
			return nil, fmt.Errorf("%w: %s", domain.ErrUnknownInstitution, req.InstitutionCode)
		}
		r.InstitutionCode = &req.InstitutionCode
		r.BankName = &bank.Name
		if bank.SwiftBIC != "" {
			r.SwiftBIC = &bank.SwiftBIC
		}
		r.BankCountryCode = &bank.CountryCode
		if req.AccountNumber != "" {
			r.AccountNumber = &req.AccountNumber
			masked := maskAccountNumber(req.AccountNumber)
			r.AccountNumberMasked = &masked
		}
		if r.DisplayName == "" {
			r.DisplayName = bank.Name + " " + req.AccountNumber
		}

	default:
		return nil, fmt.Errorf("%w: unsupported recipient type %q", domain.ErrInvalidInput, req.Type)
	}

	if err := s.recipients.Upsert(ctx, r); err != nil {
		return nil, fmt.Errorf("upserting recipient: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditRecipientCreated,
		ActorType:    "user",
		ActorID:      &ownerUserID,
		ResourceType: "recipient",
		ResourceID:   r.ID,
	})

	return r, nil
}

func resolveUser(ctx context.Context, users repository.UserRepository, identifier string) (*domain.User, error) {
	if p, err := phone.Parse(identifier); err == nil {
		if user, userErr := users.GetByPhone(ctx, p); userErr == nil {
			return user, nil
		}
	}
	return users.GetByUsername(ctx, identifier)
}

// SaveFromTransfer is called after a successful transfer to upsert the
// counterparty as a recipient. For neo_user recipients the counterparty is
// looked up by neoUserID; for bank_account recipients the bank metadata is
// resolved from the static EthiopianBanks map.
//
// This method is fire-and-forget: callers should ignore the returned error.
func (s *Service) SaveFromTransfer(ctx context.Context, ownerUserID string, counterparty TransferCounterparty, currency string) error {
	now := time.Now()

	r := &domain.Recipient{
		OwnerUserID:      ownerUserID,
		Type:             counterparty.Type,
		DisplayName:      counterparty.DisplayName,
		LastUsedAt:       &now,
		LastUsedCurrency: &currency,
	}

	switch counterparty.Type {
	case domain.RecipientNeoUser:
		r.NeoUserID = &counterparty.NeoUserID
		if counterparty.CountryCode != "" {
			r.CountryCode = &counterparty.CountryCode
		}
		if counterparty.Number != "" {
			r.Number = &counterparty.Number
		}
		if counterparty.Username != "" {
			r.Username = &counterparty.Username
		}

	case domain.RecipientBankAccount:
		bank := domain.LookupBank(counterparty.InstitutionCode)
		if bank == nil {
			return fmt.Errorf("%w: %s", domain.ErrUnknownInstitution, counterparty.InstitutionCode)
		}
		r.InstitutionCode = &counterparty.InstitutionCode
		r.BankName = &bank.Name
		if bank.SwiftBIC != "" {
			r.SwiftBIC = &bank.SwiftBIC
		}
		r.BankCountryCode = &bank.CountryCode
		if counterparty.AccountNumber != "" {
			r.AccountNumber = &counterparty.AccountNumber
			masked := maskAccountNumber(counterparty.AccountNumber)
			r.AccountNumberMasked = &masked
		}
	}

	if err := s.recipients.Upsert(ctx, r); err != nil {
		return fmt.Errorf("upserting recipient: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditRecipientSaved,
		ActorType:    "user",
		ActorID:      &ownerUserID,
		ResourceType: "recipient",
		ResourceID:   r.ID,
	})

	return nil
}

// List returns paginated recipients for a user.
func (s *Service) List(ctx context.Context, ownerUserID string, filter repository.RecipientFilter) ([]domain.Recipient, int, error) {
	return s.recipients.ListByOwner(ctx, ownerUserID, filter)
}

// GetByID returns a single recipient, enforcing owner scoping.
func (s *Service) GetByID(ctx context.Context, ownerUserID, recipientID string) (*domain.Recipient, error) {
	r, err := s.recipients.GetByID(ctx, recipientID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if r.Status == domain.RecipientArchived {
		return nil, domain.ErrRecipientArchived
	}
	return r, nil
}

// SearchByBankAccount returns recipients matching an institution + account prefix.
func (s *Service) SearchByBankAccount(ctx context.Context, ownerUserID, institutionCode, accountPrefix string) ([]domain.Recipient, error) {
	return s.recipients.SearchByBankAccount(ctx, ownerUserID, institutionCode, accountPrefix, 5)
}

// SearchByName returns recipients matching a display name prefix.
func (s *Service) SearchByName(ctx context.Context, ownerUserID, query string) ([]domain.Recipient, error) {
	return s.recipients.SearchByName(ctx, ownerUserID, query, 10)
}

// SetFavorite toggles the favorite flag on a recipient.
func (s *Service) SetFavorite(ctx context.Context, ownerUserID, recipientID string, isFavorite bool) error {
	if err := s.recipients.UpdateFavorite(ctx, recipientID, ownerUserID, isFavorite); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditRecipientFavorited,
		ActorType:    "user",
		ActorID:      &ownerUserID,
		ResourceType: "recipient",
		ResourceID:   recipientID,
	})
	return nil
}

// Archive soft-deletes a recipient.
func (s *Service) Archive(ctx context.Context, ownerUserID, recipientID string) error {
	if err := s.recipients.Archive(ctx, recipientID, ownerUserID); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditRecipientArchived,
		ActorType:    "user",
		ActorID:      &ownerUserID,
		ResourceType: "recipient",
		ResourceID:   recipientID,
	})
	return nil
}

// SaveFromTransferFireAndForget wraps SaveFromTransfer, logging errors instead
// of returning them. Use this in transfer flows where recipient save must not
// fail the parent operation.
func (s *Service) SaveFromTransferFireAndForget(ctx context.Context, ownerUserID string, cp TransferCounterparty, currency string) {
	if err := s.SaveFromTransfer(ctx, ownerUserID, cp, currency); err != nil {
		nlog.FromContext(ctx).Warn("failed to save recipient from transfer",
			slog.String("ownerUserID", ownerUserID),
			slog.String("error", err.Error()),
		)
	}
}

// maskAccountNumber returns a masked version like "****5678".
func maskAccountNumber(acct string) string {
	if len(acct) <= 4 {
		return acct
	}
	return "****" + acct[len(acct)-4:]
}
