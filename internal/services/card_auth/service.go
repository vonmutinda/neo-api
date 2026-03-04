package cardauth

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	iso "github.com/vonmutinda/neo/internal/gateway/ethswitch/iso8583"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	nlog "github.com/vonmutinda/neo/pkg/logger"
	"github.com/vonmutinda/neo/pkg/money"
)

// Response codes per ISO 8583 DE 39.
const (
	RCApproved          = "00"
	RCInsufficientFunds = "51"
	RCCardFrozen        = "62"
	RCExpiredCard       = "54"
	RCDoNotHonor        = "05"
	RCSystemError       = "96"
	RCExceedsLimit      = "61"
)

// Service handles inbound card authorization requests from SmartVista.
type Service struct {
	cards       repository.CardRepository
	auths       repository.CardAuthorizationRepository
	users       repository.UserRepository
	balances    repository.CurrencyBalanceRepository
	audit       repository.AuditRepository
	ledger      ledger.Client
	chart       *ledger.Chart
	codec       *iso.Codec
	rates       convert.RateProvider
	regulatory  *regulatory.Service
	iso4217     ISO4217Resolver
}

func NewService(
	cards repository.CardRepository,
	auths repository.CardAuthorizationRepository,
	users repository.UserRepository,
	balances repository.CurrencyBalanceRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
	chart *ledger.Chart,
	rates convert.RateProvider,
	regulatorySvc *regulatory.Service,
	iso4217 ISO4217Resolver,
) *Service {
	return &Service{
		cards:      cards,
		auths:      auths,
		users:      users,
		balances:   balances,
		audit:      audit,
		ledger:     ledgerClient,
		chart:      chart,
		codec:      iso.NewCodec(),
		rates:      rates,
		regulatory: regulatorySvc,
		iso4217:    iso4217,
	}
}

// HandleMessage is the iso8583.MessageHandler callback registered with the Router.
func (s *Service) HandleMessage(ctx context.Context, data []byte) ([]byte, error) {
	log := nlog.FromContext(ctx)

	req, err := s.codec.UnpackAuthRequest(data)
	if err != nil {
		return nil, fmt.Errorf("unpacking auth request: %w", err)
	}

	log.Info("card auth request received",
		slog.String("rrn", req.RRN),
		slog.String("stan", req.STAN),
		slog.Int64("amount", req.TransactionAmount),
		slog.String("mcc", req.MCC),
	)

	resp := s.decide(ctx, req)

	packed, err := s.codec.PackAuthResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("packing auth response: %w", err)
	}
	return packed, nil
}

// decide runs the authorization decisioning engine with multi-currency spend waterfall.
func (s *Service) decide(ctx context.Context, req *iso.AuthorizationRequest) *iso.AuthorizationResponse {
	resp := &iso.AuthorizationResponse{
		MTI:                    "0110",
		PAN:                    req.PAN,
		ProcessingCode:         req.ProcessingCode,
		TransactionAmount:      req.TransactionAmount,
		TransmissionDateTime:   req.TransmissionDateTime,
		STAN:                   req.STAN,
		LocalTime:              req.LocalTime,
		LocalDate:              req.LocalDate,
		AcquiringInstitutionID: req.AcquiringInstitutionID,
		RRN:                    req.RRN,
		TerminalID:             req.TerminalID,
		MerchantID:             req.MerchantID,
	}

	// Step 1: Look up card by tokenized PAN
	card, err := s.cards.GetByToken(ctx, req.PAN)
	if err != nil {
		resp.ResponseCode = RCDoNotHonor
		s.recordDecline(ctx, req, card, RCDoNotHonor, "card not found", nil)
		return resp
	}

	// Step 2: Card status checks
	if card.Status == domain.CardStatusFrozen {
		resp.ResponseCode = RCCardFrozen
		s.recordDecline(ctx, req, card, RCCardFrozen, "card frozen", nil)
		return resp
	}
	if card.Status == domain.CardStatusExpired || card.Status == domain.CardStatusCancelled {
		resp.ResponseCode = RCExpiredCard
		s.recordDecline(ctx, req, card, RCExpiredCard, "card expired/cancelled", nil)
		return resp
	}
	if card.Status != domain.CardStatusActive {
		resp.ResponseCode = RCDoNotHonor
		s.recordDecline(ctx, req, card, RCDoNotHonor, "card not active", nil)
		return resp
	}

	// Step 3: Channel toggle checks
	isOnline := req.POSEntryMode == "010" || req.POSEntryMode == "810"
	isATM := req.ProcessingCode[:2] == "01"
	isContactless := req.POSEntryMode == "071" || req.POSEntryMode == "910"

	if isOnline && !card.AllowOnline {
		resp.ResponseCode = RCDoNotHonor
		s.recordDecline(ctx, req, card, RCDoNotHonor, "online disabled", nil)
		return resp
	}
	if isATM && !card.AllowATM {
		resp.ResponseCode = RCDoNotHonor
		s.recordDecline(ctx, req, card, RCDoNotHonor, "ATM disabled", nil)
		return resp
	}
	if isContactless && !card.AllowContactless {
		resp.ResponseCode = RCDoNotHonor
		s.recordDecline(ctx, req, card, RCDoNotHonor, "contactless disabled", nil)
		return resp
	}

	// Step 4: Spending limit checks (per-txn, daily, monthly)
	amountCents := req.TransactionAmount
	if amountCents > card.PerTxnLimitCents {
		resp.ResponseCode = RCExceedsLimit
		s.recordDecline(ctx, req, card, RCExceedsLimit, "exceeds per-txn limit", nil)
		return resp
	}

	dailyTotal, err := s.auths.SumApprovedToday(ctx, card.ID)
	if err != nil {
		nlog.FromContext(ctx).Error("daily sum query failed", slog.String("error", err.Error()))
	} else if dailyTotal+amountCents > card.DailyLimitCents {
		resp.ResponseCode = RCExceedsLimit
		s.recordDecline(ctx, req, card, RCExceedsLimit,
			fmt.Sprintf("daily limit exceeded: used %d + requested %d > limit %d", dailyTotal, amountCents, card.DailyLimitCents), nil)
		return resp
	}

	monthlyTotal, err := s.auths.SumApprovedThisMonth(ctx, card.ID)
	if err != nil {
		nlog.FromContext(ctx).Error("monthly sum query failed", slog.String("error", err.Error()))
	} else if monthlyTotal+amountCents > card.MonthlyLimitCents {
		resp.ResponseCode = RCExceedsLimit
		s.recordDecline(ctx, req, card, RCExceedsLimit,
			fmt.Sprintf("monthly limit exceeded: used %d + requested %d > limit %d", monthlyTotal, amountCents, card.MonthlyLimitCents), nil)
		return resp
	}

	// Step 5: Check user is not frozen
	user, err := s.users.GetByID(ctx, card.UserID)
	if err != nil || user.IsFrozen {
		resp.ResponseCode = RCDoNotHonor
		reason := "user not found"
		if user != nil && user.IsFrozen {
			reason = "user frozen"
		}
		s.recordDecline(ctx, req, card, RCDoNotHonor, reason, nil)
		return resp
	}

	// Step 6: Determine merchant currency (from ISO 4217 DE 49)
	merchantCurrency := s.iso4217.Resolve(req.CurrencyCode)

	isInternational := merchantCurrency != money.CurrencyETB

	// Step 7: Regulatory gate for international spend
	if isInternational && s.regulatory != nil {
		enabled, _ := s.regulatory.IsEnabled(ctx, "card_international_spend_enabled", user)
		if !enabled {
			resp.ResponseCode = RCDoNotHonor
			s.recordDecline(ctx, req, card, RCDoNotHonor, "international card spend disabled", &merchantCurrency)
			return resp
		}

		if !card.AllowInternational {
			resp.ResponseCode = RCDoNotHonor
			s.recordDecline(ctx, req, card, RCDoNotHonor, "card international toggle off", &merchantCurrency)
			return resp
		}

		intlCap, err := s.regulatory.GetAmountLimit(ctx, "card_international_monthly_cap", user, "USD")
		if err == nil && intlCap > 0 {
			intlTotal, _ := s.auths.SumInternationalThisMonth(ctx, user.ID)
			var amountInUSD int64
			if merchantCurrency == "USD" {
				amountInUSD = amountCents
			} else {
				rate, rateErr := s.rates.GetRate(ctx, merchantCurrency, "USD")
				if rateErr != nil {
					nlog.FromContext(ctx).Warn("failed to get FX rate for intl cap check, declining",
						slog.String("error", rateErr.Error()))
					resp.ResponseCode = RCSystemError
					s.recordDecline(ctx, req, card, RCSystemError, "fx rate unavailable", &merchantCurrency)
					return resp
				}
				amountInUSD = int64(math.Round(float64(amountCents) * rate.Mid))
			}
			if intlTotal+amountInUSD > intlCap {
				resp.ResponseCode = RCExceedsLimit
				s.recordDecline(ctx, req, card, RCExceedsLimit, "international monthly cap exceeded", &merchantCurrency)
				return resp
			}
		}
	}

	// Step 8: Resolve spend currency via waterfall
	waterfall := user.SpendWaterfallOrder
	if len(waterfall) == 0 {
		waterfall = domain.DefaultSpendWaterfall
	}

	var spendCurrency string
	var holdAmountCents int64
	var fxRate *float64
	var fxFromCurrency *string
	var fxFromAmountCents *int64

	for _, entry := range waterfall {
		candidateCurrency := entry
		if entry == "merchant_currency" {
			candidateCurrency = merchantCurrency
		}

		// Check user has an active balance in this currency
		_, err := s.balances.GetByUserAndCurrency(ctx, user.ID, candidateCurrency)
		if err != nil {
			continue
		}

		if candidateCurrency == merchantCurrency {
			// No conversion needed
			asset := money.FormatAsset(candidateCurrency)
			bal, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, asset)
			if err != nil || bal.Int64() < amountCents {
				continue
			}
			spendCurrency = candidateCurrency
			holdAmountCents = amountCents
			break
		}

		// Auto-conversion needed
		if isInternational && s.regulatory != nil {
			autoEnabled, _ := s.regulatory.IsEnabled(ctx, "card_auto_conversion_enabled", user)
			if !autoEnabled {
				continue
			}
		}

		rate, err := s.rates.GetRate(ctx, merchantCurrency, candidateCurrency)
		if err != nil {
			continue
		}

		convertedCents := int64(math.Ceil(float64(amountCents) * rate.Ask))

		asset := money.FormatAsset(candidateCurrency)
		bal, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, asset)
		if err != nil || bal.Int64() < convertedCents {
			continue
		}

		spendCurrency = candidateCurrency
		holdAmountCents = convertedCents
		rateVal := rate.Ask
		fxRate = &rateVal
		fromCur := candidateCurrency
		fxFromCurrency = &fromCur
		fxFromAmountCents = &convertedCents
		break
	}

	if spendCurrency == "" {
		resp.ResponseCode = RCInsufficientFunds
		s.recordDecline(ctx, req, card, RCInsufficientFunds, "no currency in waterfall has sufficient funds", &merchantCurrency)
		return resp
	}

	// Step 9: Create Formance hold in the resolved spend currency
	asset := money.FormatAsset(spendCurrency)
	holdIK := fmt.Sprintf("card-auth-%s-%s", req.RRN, req.STAN)
	holdID, err := s.ledger.HoldFunds(ctx, holdIK, user.LedgerWalletID, s.chart.TransitCardAuth(), holdAmountCents, asset)
	if err != nil {
		resp.ResponseCode = RCInsufficientFunds
		s.recordDecline(ctx, req, card, RCInsufficientFunds, "insufficient funds or ledger error", &merchantCurrency)
		return resp
	}

	// Step 10: Generate auth code and record approval
	authCode := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	resp.AuthorizationCode = authCode
	resp.ResponseCode = RCApproved

	mcc := req.MCC
	merchantName := req.MerchantNameLocation
	terminalID := req.TerminalID
	merchantID := req.MerchantID
	acqInst := req.AcquiringInstitutionID

	auth := &domain.CardAuthorization{
		CardID:                   card.ID,
		RetrievalReferenceNumber: req.RRN,
		STAN:                     req.STAN,
		AuthCode:                 &authCode,
		MerchantName:             &merchantName,
		MerchantID:               &merchantID,
		MerchantCategoryCode:     &mcc,
		TerminalID:               &terminalID,
		AcquiringInstitution:     &acqInst,
		AuthAmountCents:          holdAmountCents,
		Currency:                 spendCurrency,
		MerchantCurrency:         &merchantCurrency,
		FXRateApplied:            fxRate,
		FXFromCurrency:           fxFromCurrency,
		FXFromAmountCents:        fxFromAmountCents,
		Status:                   domain.AuthApproved,
		ResponseCode:             &resp.ResponseCode,
		LedgerHoldID:             &holdID,
	}

	if err := s.auths.Create(ctx, auth); err != nil {
		nlog.FromContext(ctx).Error("failed to persist auth record", slog.String("error", err.Error()))
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditCardAuthApproved,
		ActorType:    "system",
		ResourceType: "card_authorization",
		ResourceID:   auth.ID,
	})

	return resp
}

func (s *Service) recordDecline(ctx context.Context, req *iso.AuthorizationRequest, card *domain.Card, respCode, reason string, merchantCurrency *string) {
	cardID := ""
	if card != nil {
		cardID = card.ID
	}

	mcc := req.MCC
	merchantName := req.MerchantNameLocation
	terminalID := req.TerminalID
	merchantID := req.MerchantID
	acqInst := req.AcquiringInstitutionID

	auth := &domain.CardAuthorization{
		CardID:                   cardID,
		RetrievalReferenceNumber: req.RRN,
		STAN:                     req.STAN,
		MerchantName:             &merchantName,
		MerchantID:               &merchantID,
		MerchantCategoryCode:     &mcc,
		TerminalID:               &terminalID,
		AcquiringInstitution:     &acqInst,
		AuthAmountCents:          req.TransactionAmount,
		Currency:                 "ETB",
		MerchantCurrency:         merchantCurrency,
		Status:                   domain.AuthDeclined,
		DeclineReason:            &reason,
		ResponseCode:             &respCode,
	}

	if cardID != "" {
		_ = s.auths.Create(ctx, auth)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditCardAuthDeclined,
		ActorType:    "system",
		ResourceType: "card_authorization",
		ResourceID:   req.RRN,
	})
}

// SettleAuthorization is called during clearing. Uses the FX rate locked at auth time.
func (s *Service) SettleAuthorization(ctx context.Context, rrn string, settlementAmountCents int64) error {
	auth, err := s.auths.GetByRRN(ctx, rrn)
	if err != nil {
		return fmt.Errorf("looking up auth by RRN %s: %w", rrn, err)
	}

	if auth.Status != domain.AuthApproved {
		return fmt.Errorf("auth %s is in status %s, cannot settle", auth.ID, auth.Status)
	}

	if auth.LedgerHoldID == nil {
		return fmt.Errorf("auth %s has no ledger hold ID", auth.ID)
	}

	settleIK := fmt.Sprintf("card-settle-%s", rrn)
	if err := s.ledger.SettleHold(ctx, settleIK, *auth.LedgerHoldID); err != nil {
		return fmt.Errorf("settling ledger hold: %w", err)
	}

	if err := s.auths.Settle(ctx, auth.ID, settlementAmountCents); err != nil {
		return fmt.Errorf("marking auth settled in postgres: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditCardAuthSettled,
		ActorType:    "system",
		ResourceType: "card_authorization",
		ResourceID:   auth.ID,
	})

	return nil
}

// ReverseAuthorization voids a previously approved hold.
func (s *Service) ReverseAuthorization(ctx context.Context, rrn string) error {
	auth, err := s.auths.GetByRRN(ctx, rrn)
	if err != nil {
		return fmt.Errorf("looking up auth by RRN %s: %w", rrn, err)
	}

	if auth.Status != domain.AuthApproved {
		return fmt.Errorf("auth %s is in status %s, cannot reverse", auth.ID, auth.Status)
	}

	if auth.LedgerHoldID == nil {
		return fmt.Errorf("auth %s has no ledger hold ID", auth.ID)
	}

	voidIK := fmt.Sprintf("card-void-%s", rrn)
	if err := s.ledger.VoidHold(ctx, voidIK, *auth.LedgerHoldID); err != nil {
		return fmt.Errorf("voiding ledger hold: %w", err)
	}

	if err := s.auths.Reverse(ctx, auth.ID); err != nil {
		return fmt.Errorf("marking auth reversed in postgres: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditCardAuthReversed,
		ActorType:    "system",
		ResourceType: "card_authorization",
		ResourceID:   auth.ID,
	})

	return nil
}

