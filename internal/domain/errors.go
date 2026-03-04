package domain

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/validate"
)

// AppError carries an HTTP status code and a client-safe message alongside
// the underlying error. Transport layers use this to avoid leaking internals.
type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// ErrToCode maps well-known sentinel errors to HTTP status codes.
// Returns 500 for unknown errors.
func ErrToCode(err error) (int, string) {
	switch {
	// 400
	case errors.Is(err, ErrInvalidInput),
		errors.Is(err, ErrNegativeAmount),
		errors.Is(err, ErrZeroAmount),
		errors.Is(err, ErrInvalidCurrency),
		errors.Is(err, money.ErrInvalidCurrency),
		errors.Is(err, validate.ErrValidation):
		return http.StatusBadRequest, err.Error()

	// 401
	case errors.Is(err, ErrUnauthorized),
		errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrSessionExpired),
		errors.Is(err, ErrSessionRevoked):
		return http.StatusUnauthorized, err.Error()

	// 403
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrForbidden),
		errors.Is(err, ErrUserFrozen),
		errors.Is(err, ErrCardFrozen),
		errors.Is(err, ErrBusinessFrozen),
		errors.Is(err, ErrTransferExceedsLimit),
		errors.Is(err, ErrSelfApproval),
		errors.Is(err, ErrTransferBlocked),
		errors.Is(err, ErrRemittanceCapExceeded),
		errors.Is(err, ErrInvestmentNotEnabled),
		errors.Is(err, ErrIntlCardSpendDisabled),
		errors.Is(err, ErrIntlCardCapExceeded),
		errors.Is(err, ErrAutoConversionDisabled),
		errors.Is(err, ErrFamilyPaymentDisabled),
		errors.Is(err, ErrSelfRequest),
		errors.Is(err, ErrSelfRecipient),
		errors.Is(err, ErrNotRequester),
		errors.Is(err, ErrNotPayer):
		return http.StatusForbidden, err.Error()

	// 404
	case errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrWalletNotFound),
		errors.Is(err, ErrBalanceNotFound),
		errors.Is(err, ErrBalanceNotActive),
		errors.Is(err, ErrPotNotFound),
		errors.Is(err, ErrHoldNotFound),
		errors.Is(err, ErrCardNotFound),
		errors.Is(err, ErrLoanNotFound),
		errors.Is(err, ErrNotFound),
		errors.Is(err, ErrNoEligibleProfile),
		errors.Is(err, ErrBusinessNotFound),
		errors.Is(err, ErrMemberNotFound),
		errors.Is(err, ErrRoleNotFound),
		errors.Is(err, ErrPendingTransferNotFound),
		errors.Is(err, ErrInvoiceNotFound),
		errors.Is(err, ErrDocumentNotFound),
		errors.Is(err, ErrCategoryNotFound),
		errors.Is(err, ErrTaxPotNotFound),
		errors.Is(err, ErrBatchNotFound),
		errors.Is(err, ErrRegulatoryRuleNotFound),
		errors.Is(err, ErrBeneficiaryNotFound),
		errors.Is(err, ErrStaffNotFound),
		errors.Is(err, ErrFlagNotFound),
		errors.Is(err, ErrConfigNotFound),
		errors.Is(err, ErrFeeScheduleNotFound),
		errors.Is(err, ErrRemittanceNotFound),
		errors.Is(err, ErrRecipientNotFound),
		errors.Is(err, ErrPaymentRequestNotFound):
		return http.StatusNotFound, err.Error()

	// 410
	case errors.Is(err, ErrRecipientArchived):
		return http.StatusGone, err.Error()

	// 409
	case errors.Is(err, ErrPaymentRequestNotPending):
		return http.StatusConflict, err.Error()

	case errors.Is(err, ErrConflict),
		errors.Is(err, ErrUsernameTaken),
		errors.Is(err, ErrBalanceAlreadyExists),
		errors.Is(err, ErrBalanceAlreadyActive),
		errors.Is(err, ErrHoldClosed),
		errors.Is(err, ErrPotArchived),
		errors.Is(err, ErrAlreadyMember),
		errors.Is(err, ErrRoleInUse),
		errors.Is(err, ErrSystemRoleImmutable):
		return http.StatusConflict, err.Error()

	// 422
	case errors.Is(err, ErrInsufficientFunds),
		errors.Is(err, ErrBalanceNotEmpty),
		errors.Is(err, ErrCannotDeletePrimary),
		errors.Is(err, ErrLoanLimitExceeded),
		errors.Is(err, ErrNBEBlacklisted),
		errors.Is(err, ErrOverdraftNotEligible),
		errors.Is(err, ErrOverdraftAlreadyActive),
		errors.Is(err, ErrOverdraftNotActive),
		errors.Is(err, ErrOverdraftInUse),
		errors.Is(err, ErrOverdraftLimitExceeded),
		errors.Is(err, ErrOverdraftETBOnly),
		errors.Is(err, ErrCardExpired),
		errors.Is(err, ErrCurrencyNotActive),
		errors.Is(err, ErrBusinessNotActive),
		errors.Is(err, ErrMemberNotActive),
		errors.Is(err, ErrCannotRemoveOwner),
		errors.Is(err, ErrDailyLimitExceeded),
		errors.Is(err, ErrBatchNotDraft),
		errors.Is(err, ErrKYCInsufficientForFX),
		errors.Is(err, ErrDocumentRequired),
		errors.Is(err, ErrPurposeRequired),
		errors.Is(err, ErrBeneficiaryNotVerified),
		errors.Is(err, ErrRateStaleness),
		errors.Is(err, ErrFXConversionDisabled),
		errors.Is(err, ErrWaterfallExhausted),
		errors.Is(err, ErrStaffDeactivated),
		errors.Is(err, ErrNoProviderForCorridor),
		errors.Is(err, ErrQuoteExpired),
		errors.Is(err, ErrReminderLimitReached),
		errors.Is(err, ErrActiveLoanExists):
		return http.StatusUnprocessableEntity, err.Error()

	// 404 (FX)
	case errors.Is(err, ErrFXRateNotFound):
		return http.StatusNotFound, err.Error()

	// 503
	case errors.Is(err, ErrEthSwitchTimeout),
		errors.Is(err, ErrEthSwitchUnavailable),
		errors.Is(err, ErrFaydaUnavailable),
		errors.Is(err, ErrFaydaAuthFailed),
		errors.Is(err, ErrFXRateStale),
		errors.Is(err, ErrProviderUnavailable):
		return http.StatusServiceUnavailable, "service temporarily unavailable"

	default:
		return http.StatusInternalServerError, "internal error"
	}
}

// Sentinel errors used across the domain layer.
// These are the canonical error values that usecases return and transport
// layers map to HTTP status codes.

var (
	// Identity & Auth
	ErrUserNotFound    = errors.New("user not found")
	ErrUserFrozen      = errors.New("user account is frozen")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionRevoked  = errors.New("session revoked")
	ErrSessionNotFound = errors.New("session not found")
	ErrUsernameTaken   = errors.New("username already taken")

	// Idempotency
	ErrIdempotencyKeyMissing      = errors.New("idempotency key header is required")
	ErrIdempotencyPayloadMismatch = errors.New("idempotency key reused with different payload")
	ErrIdempotencyRequestInFlight = errors.New("request with this idempotency key is already in progress")

	// Financial
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrNegativeAmount    = errors.New("negative amount")
	ErrZeroAmount        = errors.New("zero amount")
	ErrInvalidCurrency   = errors.New("invalid currency")

	// Wallets & Balances
	ErrWalletNotFound       = errors.New("wallet not found")
	ErrBalanceNotFound      = errors.New("balance not found")
	ErrBalanceAlreadyExists = errors.New("balance already exists")
	ErrBalanceAlreadyActive = errors.New("currency balance is already active")
	ErrBalanceNotActive     = errors.New("no active balance for this currency")
	ErrCannotDeletePrimary  = errors.New("cannot delete the primary currency balance")
	ErrBalanceNotEmpty      = errors.New("balance is not empty; move all funds before deleting")
	ErrCurrencyNotActive    = errors.New("currency is not active for this user")

	// Pots
	ErrPotNotFound = errors.New("pot not found")
	ErrPotArchived = errors.New("pot is archived")

	// Holds
	ErrHoldNotFound = errors.New("hold not found")
	ErrHoldClosed   = errors.New("hold is already closed")

	// Cards
	ErrCardNotFound = errors.New("card not found")
	ErrCardFrozen   = errors.New("card is frozen")
	ErrCardExpired  = errors.New("card is expired")

	// Lending
	ErrLoanNotFound       = errors.New("loan not found")
	ErrLoanLimitExceeded  = errors.New("requested amount exceeds approved limit")
	ErrNBEBlacklisted     = errors.New("user is blacklisted by the National Bank of Ethiopia")
	ErrNoEligibleProfile  = errors.New("user does not have an eligible credit profile")

	// Overdraft
	ErrOverdraftNotEligible   = errors.New("overdraft not available for this account")
	ErrOverdraftAlreadyActive = errors.New("overdraft is already enabled")
	ErrOverdraftNotActive     = errors.New("overdraft is not active")
	ErrOverdraftInUse         = errors.New("cannot opt out while overdraft is in use")
	ErrOverdraftLimitExceeded = errors.New("overdraft limit exceeded")
	ErrOverdraftETBOnly       = errors.New("overdraft is only available for ETB")

	// External Systems
	ErrEthSwitchTimeout     = errors.New("ethswitch request timed out")
	ErrEthSwitchUnavailable = errors.New("ethswitch service unavailable")
	ErrFaydaUnavailable     = errors.New("fayda service unavailable")
	ErrFaydaAuthFailed      = errors.New("fayda authentication failed")

	// Business
	ErrBusinessNotFound     = errors.New("business not found")
	ErrBusinessFrozen       = errors.New("business account is frozen")
	ErrBusinessNotActive    = errors.New("business is not active")
	ErrMemberNotFound       = errors.New("business member not found")
	ErrMemberNotActive      = errors.New("business member is not active")
	ErrRoleNotFound         = errors.New("business role not found")
	ErrSystemRoleImmutable  = errors.New("system roles cannot be modified")
	ErrRoleInUse            = errors.New("role is still assigned to members")
	ErrAlreadyMember        = errors.New("user is already a member of this business")
	ErrCannotRemoveOwner    = errors.New("cannot remove the business owner")
	ErrSelfApproval         = errors.New("cannot approve your own transfer")
	ErrTransferExceedsLimit = errors.New("transfer amount exceeds role limit")
	ErrDailyLimitExceeded   = errors.New("daily transfer limit exceeded")
	ErrApprovalRequired     = errors.New("transfer requires approval")
	ErrPendingTransferNotFound = errors.New("pending transfer not found")
	ErrInvoiceNotFound      = errors.New("invoice not found")
	ErrDocumentNotFound     = errors.New("document not found")
	ErrCategoryNotFound     = errors.New("category not found")
	ErrTaxPotNotFound       = errors.New("tax pot not found")
	ErrBatchNotFound        = errors.New("batch payment not found")
	ErrBatchNotDraft        = errors.New("batch is not in draft status")

	// Regulatory / FX Compliance
	ErrKYCInsufficientForFX   = errors.New("KYC Verified or higher required to hold foreign currency")
	ErrRegulatoryRuleNotFound = errors.New("regulatory rule not found")
	ErrTransferBlocked        = errors.New("transfer blocked by regulatory rule")
	ErrRemittanceCapExceeded  = errors.New("monthly outbound remittance cap exceeded")
	ErrDocumentRequired       = errors.New("supporting documents required for this transfer amount")
	ErrPurposeRequired        = errors.New("transfer purpose is required for outbound remittance")
	ErrInvestmentNotEnabled   = errors.New("outbound investment is not currently enabled")
	ErrFXConversionDisabled   = errors.New("FX conversion is currently disabled")
	ErrRateStaleness          = errors.New("exchange rate data is stale; conversion unavailable")
	ErrBeneficiaryNotFound    = errors.New("beneficiary not found")
	ErrBeneficiaryNotVerified = errors.New("beneficiary is not verified")
	ErrFamilyPaymentDisabled  = errors.New("family FX payments are currently disabled")
	ErrIntlCardSpendDisabled  = errors.New("international card spend is currently disabled")
	ErrIntlCardCapExceeded    = errors.New("monthly international card spend cap exceeded")
	ErrAutoConversionDisabled = errors.New("automatic currency conversion is currently disabled")
	ErrWaterfallExhausted     = errors.New("no currency in spend waterfall has sufficient funds")

	// FX Rates
	ErrFXRateNotFound = errors.New("exchange rate not found")
	ErrFXRateStale    = errors.New("exchange rate data is stale")

	// Pricing / Fees
	ErrFeeScheduleNotFound   = errors.New("fee schedule not found")
	ErrNoProviderForCorridor = errors.New("no remittance provider available for this corridor")
	ErrQuoteExpired          = errors.New("remittance quote has expired")
	ErrProviderUnavailable   = errors.New("remittance provider is temporarily unavailable")
	ErrRemittanceNotFound    = errors.New("remittance transfer not found")

	// Admin / Staff
	ErrStaffNotFound      = errors.New("staff member not found")
	ErrStaffDeactivated   = errors.New("staff account is deactivated")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrPermissionDenied   = errors.New("insufficient permissions")
	ErrFlagNotFound       = errors.New("customer flag not found")
	ErrConfigNotFound     = errors.New("system config key not found")

	// Recipients
	ErrRecipientNotFound    = errors.New("recipient not found")
	ErrRecipientArchived    = errors.New("recipient is archived")
	ErrUnknownInstitution   = errors.New("unknown financial institution code")
	ErrSelfRecipient        = errors.New("cannot add yourself as a recipient")

	// Payment Requests
	ErrPaymentRequestNotFound   = errors.New("payment request not found")
	ErrPaymentRequestNotPending = errors.New("payment request is no longer pending")
	ErrSelfRequest              = errors.New("cannot request funds from yourself")
	ErrReminderLimitReached     = errors.New("reminder limit reached for this request")
	ErrNotRequester             = errors.New("only the requester can perform this action")
	ErrNotPayer                 = errors.New("only the payer can perform this action")

	// Lending
	ErrActiveLoanExists = errors.New("you already have an active loan; repay it before applying for a new one")

	// General
	ErrNotFound         = errors.New("resource not found")
	ErrConflict         = errors.New("resource conflict")
	ErrInvalidInput     = errors.New("invalid input")
	ErrInternal         = errors.New("internal error")
)
