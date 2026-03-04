package payment_requests

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/payments"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/google/uuid"
)

type Service struct {
	requests repository.PaymentRequestRepository
	users    repository.UserRepository
	payments *payments.Service
	audit    repository.AuditRepository
}

func NewService(
	requests repository.PaymentRequestRepository,
	users repository.UserRepository,
	paymentsSvc *payments.Service,
	audit repository.AuditRepository,
) *Service {
	return &Service{
		requests: requests,
		users:    users,
		payments: paymentsSvc,
		audit:    audit,
	}
}

func (s *Service) Create(ctx context.Context, requesterID string, form *CreatePaymentRequestForm) (*domain.PaymentRequest, error) {
	if err := form.Validate(); err != nil {
		return nil, err
	}

	requester, err := s.users.GetByID(ctx, requesterID)
	if err != nil {
		return nil, fmt.Errorf("looking up requester: %w", err)
	}
	if requester.IsFrozen {
		return nil, domain.ErrUserFrozen
	}

	var payerID *string
	var payerPhone phone.PhoneNumber

	if p, parseErr := phone.Parse(form.Recipient); parseErr == nil {
		payerPhone = p
		payer, lookupErr := s.users.GetByPhone(ctx, p)
		if lookupErr == nil {
			payerID = &payer.ID
			if payer.IsFrozen {
				return nil, fmt.Errorf("payer: %w", domain.ErrUserFrozen)
			}
		}
	} else {
		payer, lookupErr := s.users.GetByUsername(ctx, form.Recipient)
		if lookupErr == nil {
			payerID = &payer.ID
			payerPhone = payer.PhoneNumber
			if payer.IsFrozen {
				return nil, fmt.Errorf("payer: %w", domain.ErrUserFrozen)
			}
		} else {
			return nil, fmt.Errorf("payer not found: %w", domain.ErrUserNotFound)
		}
	}

	if payerID != nil && *payerID == requesterID {
		return nil, domain.ErrSelfRequest
	}

	pr := &domain.PaymentRequest{
		RequesterID:  requesterID,
		PayerID:      payerID,
		PayerPhone:   payerPhone,
		AmountCents:  form.AmountCents,
		CurrencyCode: form.CurrencyCode,
		Narration:    form.Narration,
		ExpiresAt:    time.Now().AddDate(0, 0, 30),
	}

	if err := s.requests.Create(ctx, pr); err != nil {
		return nil, fmt.Errorf("creating payment request: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditPaymentRequestCreated,
		ActorType:    "user",
		ActorID:      &requesterID,
		ResourceType: "payment_request",
		ResourceID:   pr.ID,
	})

	return pr, nil
}

func (s *Service) Pay(ctx context.Context, payerID, requestID string) error {
	pr, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	if pr.PayerID == nil || *pr.PayerID != payerID {
		return domain.ErrNotPayer
	}
	if pr.Status != domain.PaymentRequestPending {
		return domain.ErrPaymentRequestNotPending
	}

	requester, err := s.users.GetByID(ctx, pr.RequesterID)
	if err != nil {
		return fmt.Errorf("looking up requester: %w", err)
	}

	inboundReq := &payments.InboundTransferRequest{
		Recipient:   requester.PhoneNumber.E164(),
		AmountCents: pr.AmountCents,
		Currency:    pr.CurrencyCode,
		Narration:   pr.Narration,
	}

	if err := s.payments.ProcessInboundTransfer(ctx, payerID, inboundReq); err != nil {
		return err
	}

	txID := requestID
	if err := s.requests.Pay(ctx, requestID, txID); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditPaymentRequestPaid,
		ActorType:    "user",
		ActorID:      &payerID,
		ResourceType: "payment_request",
		ResourceID:   requestID,
	})

	return nil
}

func (s *Service) Decline(ctx context.Context, payerID, requestID string, form *DeclineForm) error {
	pr, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if pr.PayerID == nil || *pr.PayerID != payerID {
		return domain.ErrNotPayer
	}
	if pr.Status != domain.PaymentRequestPending {
		return domain.ErrPaymentRequestNotPending
	}

	reason := ""
	if form != nil {
		reason = form.Reason
	}
	if err := s.requests.Decline(ctx, requestID, reason); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditPaymentRequestDeclined,
		ActorType:    "user",
		ActorID:      &payerID,
		ResourceType: "payment_request",
		ResourceID:   requestID,
	})

	return nil
}

func (s *Service) Cancel(ctx context.Context, requesterID, requestID string) error {
	pr, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if pr.RequesterID != requesterID {
		return domain.ErrNotRequester
	}
	if pr.Status != domain.PaymentRequestPending {
		return domain.ErrPaymentRequestNotPending
	}

	if err := s.requests.Cancel(ctx, requestID); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditPaymentRequestCancelled,
		ActorType:    "user",
		ActorID:      &requesterID,
		ResourceType: "payment_request",
		ResourceID:   requestID,
	})

	return nil
}

func (s *Service) Remind(ctx context.Context, requesterID, requestID string) error {
	pr, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if pr.RequesterID != requesterID {
		return domain.ErrNotRequester
	}
	if pr.Status != domain.PaymentRequestPending {
		return domain.ErrPaymentRequestNotPending
	}

	if err := s.requests.IncrementReminder(ctx, requestID); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditPaymentRequestReminded,
		ActorType:    "user",
		ActorID:      &requesterID,
		ResourceType: "payment_request",
		ResourceID:   requestID,
	})

	return nil
}

func (s *Service) ListSent(ctx context.Context, requesterID string, limit, offset int) ([]domain.PaymentRequest, error) {
	return s.requests.ListByRequester(ctx, requesterID, limit, offset)
}

func (s *Service) ListReceived(ctx context.Context, payerID string, limit, offset int) ([]domain.PaymentRequest, error) {
	return s.requests.ListByPayer(ctx, payerID, limit, offset)
}

func (s *Service) Get(ctx context.Context, callerID, requestID string) (*domain.PaymentRequest, error) {
	pr, err := s.requests.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if pr.RequesterID != callerID && (pr.PayerID == nil || *pr.PayerID != callerID) {
		return nil, domain.ErrPaymentRequestNotFound
	}
	return pr, nil
}

func (s *Service) PendingCount(ctx context.Context, payerID string) (int, error) {
	return s.requests.CountPendingByPayer(ctx, payerID)
}

type BatchPaymentRequestResponse struct {
	Requests         []domain.PaymentRequest `json:"requests"`
	TotalAmountCents int64                   `json:"totalAmountCents"`
	RecipientCount   int                     `json:"recipientCount"`
}

func (s *Service) CreateBatch(ctx context.Context, requesterID string, form *BatchPaymentRequestForm) (*BatchPaymentRequestResponse, error) {
	if err := form.Validate(); err != nil {
		return nil, err
	}

	requester, err := s.users.GetByID(ctx, requesterID)
	if err != nil {
		return nil, fmt.Errorf("looking up requester: %w", err)
	}
	if requester.IsFrozen {
		return nil, domain.ErrUserFrozen
	}

	type resolved struct {
		userID    string
		userPhone phone.PhoneNumber
	}
	recipients := make([]resolved, 0, len(form.Recipients))

	for _, recipient := range form.Recipients {
		var payer *domain.User
		var lookupErr error

		if _, parseErr := uuid.Parse(recipient); parseErr == nil {
			payer, lookupErr = s.users.GetByID(ctx, recipient)
		} else if p, parseErr := phone.Parse(recipient); parseErr == nil {
			payer, lookupErr = s.users.GetByPhone(ctx, p)
		} else {
			payer, lookupErr = s.users.GetByUsername(ctx, recipient)
		}

		if lookupErr != nil {
			return nil, fmt.Errorf("recipient %q not found: %w", recipient, domain.ErrUserNotFound)
		}
		if payer.IsFrozen {
			return nil, fmt.Errorf("recipient %q: %w", recipient, domain.ErrUserFrozen)
		}
		if payer.ID == requesterID {
			return nil, domain.ErrSelfRequest
		}
		recipients = append(recipients, resolved{userID: payer.ID, userPhone: payer.PhoneNumber})
	}

	n := int64(len(recipients))
	perRecipient := form.TotalAmountCents / n
	remainder := form.TotalAmountCents % n

	created := make([]domain.PaymentRequest, 0, len(recipients))
	for i, r := range recipients {
		var amount int64
		if form.IsCustomSplit() {
			amount = form.CustomAmounts[form.Recipients[i]]
		} else {
			amount = perRecipient
			if i == 0 {
				amount += remainder
			}
		}
		payerID := r.userID
		pr := &domain.PaymentRequest{
			RequesterID:  requesterID,
			PayerID:      &payerID,
			PayerPhone:   r.userPhone,
			AmountCents:  amount,
			CurrencyCode: form.CurrencyCode,
			Narration:    form.Narration,
			ExpiresAt:    time.Now().AddDate(0, 0, 30),
		}
		if err := s.requests.Create(ctx, pr); err != nil {
			return nil, fmt.Errorf("creating payment request for recipient %d: %w", i, err)
		}
		created = append(created, *pr)
	}

	return &BatchPaymentRequestResponse{
		Requests:         created,
		TotalAmountCents: form.TotalAmountCents,
		RecipientCount:   len(recipients),
	}, nil
}

func (s *Service) ExpireStale(ctx context.Context) (int64, error) {
	n, err := s.requests.ExpirePending(ctx)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		_ = s.audit.Log(ctx, &domain.AuditEntry{
			Action:       domain.AuditPaymentRequestExpired,
			ActorType:    "system",
			ResourceType: "payment_request",
			ResourceID:   "batch",
			Metadata:     json.RawMessage(fmt.Sprintf(`{"count":%d}`, n)),
		})
	}
	return n, nil
}
