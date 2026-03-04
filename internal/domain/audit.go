package domain

import (
	"encoding/json"
	"net"
	"time"
)

type AuditAction string

const (
	AuditUserCreated   AuditAction = "user_created"
	AuditUserFrozen    AuditAction = "user_frozen"
	AuditUserUnfrozen  AuditAction = "user_unfrozen"

	AuditKYCOTPRequested  AuditAction = "kyc_otp_requested"
	AuditKYCVerified      AuditAction = "kyc_verified"
	AuditKYCFailed        AuditAction = "kyc_failed"
	AuditKYCLevelUpgraded AuditAction = "kyc_level_upgraded"

	AuditTransferInitiated AuditAction = "transfer_initiated"
	AuditTransferSettled   AuditAction = "transfer_settled"
	AuditTransferVoided    AuditAction = "transfer_voided"
	AuditTransferFailed    AuditAction = "transfer_failed"
	AuditP2PTransfer       AuditAction = "p2p_transfer"

	AuditCardIssued       AuditAction = "card_issued"
	AuditCardFrozen       AuditAction = "card_frozen"
	AuditCardUnfrozen     AuditAction = "card_unfrozen"
	AuditCardCancelled    AuditAction = "card_cancelled"
	AuditCardLimitChanged AuditAction = "card_limit_changed"

	AuditCardAuthApproved AuditAction = "card_auth_approved"
	AuditCardAuthDeclined AuditAction = "card_auth_declined"
	AuditCardAuthSettled  AuditAction = "card_auth_settled"
	AuditCardAuthReversed AuditAction = "card_auth_reversed"

	AuditLoanDisbursed      AuditAction = "loan_disbursed"
	AuditLoanRepayment      AuditAction = "loan_repayment"
	AuditLoanDefaulted      AuditAction = "loan_defaulted"
	AuditCreditScoreUpdated AuditAction = "credit_score_updated"

	AuditReconExceptionOpened   AuditAction = "recon_exception_opened"
	AuditReconExceptionResolved AuditAction = "recon_exception_resolved"

	AuditTelegramBound   AuditAction = "telegram_bound"
	AuditTelegramUnbound AuditAction = "telegram_unbound"

	// Business entity lifecycle
	AuditBusinessRegistered   AuditAction = "business_registered"
	AuditBusinessUpdated      AuditAction = "business_updated"
	AuditBusinessVerified     AuditAction = "business_verified"
	AuditBusinessSuspended    AuditAction = "business_suspended"
	AuditBusinessDeactivated  AuditAction = "business_deactivated"

	// RBAC role management
	AuditBusinessRoleCreated AuditAction = "business_role_created"
	AuditBusinessRoleUpdated AuditAction = "business_role_updated"
	AuditBusinessRoleDeleted AuditAction = "business_role_deleted"

	// Member management
	AuditBusinessMemberInvited     AuditAction = "business_member_invited"
	AuditBusinessMemberRoleChanged AuditAction = "business_member_role_changed"
	AuditBusinessMemberRemoved     AuditAction = "business_member_removed"

	// Transfer approval workflow
	AuditBusinessTransferApprovalRequested AuditAction = "business_transfer_approval_requested"
	AuditBusinessTransferApproved          AuditAction = "business_transfer_approved"
	AuditBusinessTransferRejected          AuditAction = "business_transfer_rejected"
	AuditBusinessTransferExpired           AuditAction = "business_transfer_expired"

	// Batch payments
	AuditBusinessBatchCreated   AuditAction = "business_batch_created"
	AuditBusinessBatchApproved  AuditAction = "business_batch_approved"
	AuditBusinessBatchProcessed AuditAction = "business_batch_processed"

	// Lending
	AuditBusinessLoanApplied   AuditAction = "business_loan_applied"
	AuditBusinessLoanDisbursed AuditAction = "business_loan_disbursed"

	// FX Rates
	AuditFXRateUpdated        AuditAction = "fx_rate_updated"
	AuditFXRateManualOverride AuditAction = "fx_rate_manual_override"

	// Regulatory / FX Compliance
	AuditTransferBlocked       AuditAction = "transfer_blocked"
	AuditTransferPendingReview AuditAction = "transfer_pending_review"
	AuditRuleEvaluated         AuditAction = "rule_evaluated"
	AuditRuleUpdated           AuditAction = "rule_updated"
	AuditFXConversion          AuditAction = "fx_conversion"
	AuditBeneficiaryAdded      AuditAction = "beneficiary_added"
	AuditBeneficiaryRemoved    AuditAction = "beneficiary_removed"

	// Admin operations
	AuditAdminNote           AuditAction = "admin_note"
	AuditAdminFreeze         AuditAction = "admin_freeze"
	AuditAdminUnfreeze       AuditAction = "admin_unfreeze"
	AuditAdminKYCOverride    AuditAction = "admin_kyc_override"
	AuditAdminLoanWriteOff   AuditAction = "admin_loan_writeoff"
	AuditAdminCreditOverride AuditAction = "admin_credit_override"
	AuditAdminCardIssued     AuditAction = "admin_card_issued"
	AuditAdminCardFreeze     AuditAction = "admin_card_freeze"
	AuditAdminCardCancel     AuditAction = "admin_card_cancel"
	AuditAdminTxnReversed    AuditAction = "admin_txn_reversed"
	AuditFlagCreated         AuditAction = "flag_created"
	AuditFlagResolved        AuditAction = "flag_resolved"
	AuditStaffCreated        AuditAction = "staff_created"
	AuditStaffDeactivated    AuditAction = "staff_deactivated"
	AuditStaffUpdated        AuditAction = "staff_updated"
	AuditConfigChanged       AuditAction = "config_changed"
	AuditBulkFreeze          AuditAction = "bulk_freeze"
	AuditReconAssigned       AuditAction = "recon_assigned"
	AuditReconEscalated      AuditAction = "recon_escalated"
	AuditReconInvestigating  AuditAction = "recon_investigating"

	// Recipients
	AuditRecipientSaved     AuditAction = "recipient_saved"
	AuditRecipientCreated   AuditAction = "recipient_created"
	AuditRecipientFavorited AuditAction = "recipient_favorited"
	AuditRecipientArchived  AuditAction = "recipient_archived"
	AuditBatchTransfer      AuditAction = "batch_transfer"

	// Payment Requests
	AuditPaymentRequestCreated   AuditAction = "payment_request_created"
	AuditPaymentRequestPaid      AuditAction = "payment_request_paid"
	AuditPaymentRequestDeclined  AuditAction = "payment_request_declined"
	AuditPaymentRequestCancelled AuditAction = "payment_request_cancelled"
	AuditPaymentRequestExpired   AuditAction = "payment_request_expired"
	AuditPaymentRequestReminded  AuditAction = "payment_request_reminded"

	// Lending (manual repayment)
	AuditLoanManualRepayment AuditAction = "loan_manual_repayment"

	// Overdraft
	AuditOverdraftOptedIn   AuditAction = "overdraft_opted_in"
	AuditOverdraftOptedOut  AuditAction = "overdraft_opted_out"
	AuditOverdraftUsed      AuditAction = "overdraft_used"
	AuditOverdraftRepaid    AuditAction = "overdraft_repaid"
	AuditOverdraftFeeAccrued AuditAction = "overdraft_fee_accrued"
	AuditOverdraftSuspended AuditAction = "overdraft_suspended"

	// Fees & Pricing
	AuditFeeScheduleCreated  AuditAction = "fee_schedule_created"
	AuditFeeScheduleUpdated  AuditAction = "fee_schedule_updated"
	AuditFeeScheduleDisabled AuditAction = "fee_schedule_disabled"
	AuditFeeCollected        AuditAction = "fee_collected"
	AuditRemittanceInitiated AuditAction = "remittance_initiated"
	AuditRemittanceCompleted AuditAction = "remittance_completed"
	AuditRemittanceFailed    AuditAction = "remittance_failed"
)

// AuditEntry represents a single immutable audit log row.
// This table is INSERT-only by policy.
type AuditEntry struct {
	ID           string          `json:"id"`
	Action       AuditAction     `json:"action"`
	ActorType    string          `json:"actorType"`
	ActorID      *string         `json:"actorId,omitempty"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	IPAddress    net.IP          `json:"ipAddress,omitempty"`
	UserAgent    *string         `json:"userAgent,omitempty"`

	RegulatoryRuleKey *string `json:"regulatoryRuleKey,omitempty"`
	RegulatoryAction  *string `json:"regulatoryAction,omitempty"`
	NBEReference      *string `json:"nbeReference,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
