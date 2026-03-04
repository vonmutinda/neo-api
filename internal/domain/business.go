package domain

import (
	"encoding/json"
	"time"

	"github.com/vonmutinda/neo/pkg/phone"
)

// AccountType distinguishes personal from business accounts.
type AccountType string

const (
	AccountTypePersonal AccountType = "personal"
	AccountTypeBusiness AccountType = "business"
)

// BusinessStatus tracks the lifecycle of a business account.
type BusinessStatus string

const (
	BusinessStatusPendingVerification BusinessStatus = "pending_verification"
	BusinessStatusActive              BusinessStatus = "active"
	BusinessStatusSuspended           BusinessStatus = "suspended"
	BusinessStatusDeactivated         BusinessStatus = "deactivated"
)

// IndustryCategory classifies a business for risk scoring and reporting.
type IndustryCategory string

const (
	IndustryRetail               IndustryCategory = "retail"
	IndustryWholesale            IndustryCategory = "wholesale"
	IndustryManufacturing        IndustryCategory = "manufacturing"
	IndustryAgriculture          IndustryCategory = "agriculture"
	IndustryTechnology           IndustryCategory = "technology"
	IndustryHealthcare           IndustryCategory = "healthcare"
	IndustryEducation            IndustryCategory = "education"
	IndustryConstruction         IndustryCategory = "construction"
	IndustryTransport            IndustryCategory = "transport"
	IndustryHospitality          IndustryCategory = "hospitality"
	IndustryFinancialServices    IndustryCategory = "financial_services"
	IndustryImportExport         IndustryCategory = "import_export"
	IndustryProfessionalServices IndustryCategory = "professional_services"
	IndustryNonProfit            IndustryCategory = "non_profit"
	IndustryOther                IndustryCategory = "other"
)

var ValidIndustryCategories = []string{
	string(IndustryRetail), string(IndustryWholesale), string(IndustryManufacturing),
	string(IndustryAgriculture), string(IndustryTechnology), string(IndustryHealthcare),
	string(IndustryEducation), string(IndustryConstruction), string(IndustryTransport),
	string(IndustryHospitality), string(IndustryFinancialServices), string(IndustryImportExport),
	string(IndustryProfessionalServices), string(IndustryNonProfit), string(IndustryOther),
}

// Business represents a business entity on the Neo platform.
type Business struct {
	ID                  string           `json:"id"`
	OwnerUserID         string           `json:"ownerUserId"`
	Name                string           `json:"name"`
	TradeName           *string          `json:"tradeName,omitempty"`
	TINNumber           string           `json:"tinNumber"`
	TradeLicenseNumber  string           `json:"tradeLicenseNumber"`
	IndustryCategory    IndustryCategory `json:"industryCategory"`
	IndustrySubCategory *string          `json:"industrySubCategory,omitempty"`
	RegistrationDate    *time.Time       `json:"registrationDate,omitempty"`
	Address             *string          `json:"address,omitempty"`
	City                *string          `json:"city,omitempty"`
	SubCity             *string          `json:"subCity,omitempty"`
	Woreda              *string          `json:"woreda,omitempty"`
	PhoneNumber         phone.PhoneNumber `json:"phoneNumber"`
	Email               *string           `json:"email,omitempty"`
	Website             *string          `json:"website,omitempty"`
	Status              BusinessStatus   `json:"status"`
	LedgerWalletID      string           `json:"ledgerWalletId"`
	KYBLevel            int              `json:"kybLevel"`
	IsFrozen            bool             `json:"isFrozen"`
	FrozenReason        *string          `json:"frozenReason,omitempty"`
	FrozenAt            *time.Time       `json:"frozenAt,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
}

// BusinessPermission is a fine-grained access control string.
type BusinessPermission string

const (
	BPermViewDashboard    BusinessPermission = "biz:dashboard:view"
	BPermViewBalances     BusinessPermission = "biz:balances:view"
	BPermViewTransactions BusinessPermission = "biz:transactions:view"
	BPermViewDocuments    BusinessPermission = "biz:documents:view"
	BPermViewLoans        BusinessPermission = "biz:loans:view"

	BPermTransferInternal BusinessPermission = "biz:transfers:initiate:internal"
	BPermTransferExternal BusinessPermission = "biz:transfers:initiate:external"
	BPermApproveTransfer  BusinessPermission = "biz:transfers:approve"
	BPermConvertCurrency  BusinessPermission = "biz:convert:initiate"

	BPermBatchCreate  BusinessPermission = "biz:batch:create"
	BPermBatchApprove BusinessPermission = "biz:batch:approve"
	BPermBatchExecute BusinessPermission = "biz:batch:execute"

	BPermExportTransactions BusinessPermission = "biz:transactions:export"
	BPermLabelTransactions  BusinessPermission = "biz:transactions:label"

	BPermManageInvoices BusinessPermission = "biz:invoices:manage"
	BPermViewInvoices   BusinessPermission = "biz:invoices:view"

	BPermManagePots      BusinessPermission = "biz:pots:manage"
	BPermManageTaxPots   BusinessPermission = "biz:tax_pots:manage"
	BPermWithdrawTaxPots BusinessPermission = "biz:tax_pots:withdraw"

	BPermManageDocuments BusinessPermission = "biz:documents:manage"

	BPermManageMembers  BusinessPermission = "biz:members:manage"
	BPermManageRoles    BusinessPermission = "biz:roles:manage"
	BPermManageSettings BusinessPermission = "biz:settings:manage"

	BPermApplyLoan BusinessPermission = "biz:loans:apply"
)

// AllBusinessPermissions is the canonical list used for validation.
var AllBusinessPermissions = []BusinessPermission{
	BPermViewDashboard, BPermViewBalances, BPermViewTransactions, BPermViewDocuments, BPermViewLoans,
	BPermTransferInternal, BPermTransferExternal, BPermApproveTransfer, BPermConvertCurrency,
	BPermBatchCreate, BPermBatchApprove, BPermBatchExecute,
	BPermExportTransactions, BPermLabelTransactions,
	BPermManageInvoices, BPermViewInvoices,
	BPermManagePots, BPermManageTaxPots, BPermWithdrawTaxPots,
	BPermManageDocuments,
	BPermManageMembers, BPermManageRoles, BPermManageSettings,
	BPermApplyLoan,
}

func ValidBusinessPermission(p string) bool {
	for _, bp := range AllBusinessPermissions {
		if string(bp) == p {
			return true
		}
	}
	return false
}

// BusinessRole defines a named set of permissions within a business.
// System roles (is_system=true) have business_id=NULL and cannot be modified.
type BusinessRole struct {
	ID                         string               `json:"id"`
	BusinessID                 *string              `json:"businessId,omitempty"`
	Name                       string               `json:"name"`
	Description                *string              `json:"description,omitempty"`
	IsSystem                   bool                 `json:"isSystem"`
	IsDefault                  bool                 `json:"isDefault"`
	MaxTransferCents           *int64               `json:"maxTransferCents,omitempty"`
	DailyTransferLimitCents    *int64               `json:"dailyTransferLimitCents,omitempty"`
	RequiresApprovalAboveCents *int64               `json:"requiresApprovalAboveCents,omitempty"`
	Permissions                []BusinessPermission `json:"permissions"`
	CreatedAt                  time.Time            `json:"createdAt"`
	UpdatedAt                  time.Time            `json:"updatedAt"`
}

// HasPermission checks whether this role grants the given permission.
func (r *BusinessRole) HasPermission(perm BusinessPermission) bool {
	for _, p := range r.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// BusinessMember links a Neo user to a business with a specific role.
type BusinessMember struct {
	ID         string        `json:"id"`
	BusinessID string        `json:"businessId"`
	UserID     string        `json:"userId"`
	RoleID     string        `json:"roleId"`
	Role       *BusinessRole `json:"role,omitempty"`
	Title      *string       `json:"title,omitempty"`
	InvitedBy  string        `json:"invitedBy"`
	JoinedAt   time.Time     `json:"joinedAt"`
	IsActive   bool          `json:"isActive"`
	RemovedAt  *time.Time    `json:"removedAt,omitempty"`
	RemovedBy  *string       `json:"removedBy,omitempty"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

// PendingTransferStatus tracks approval workflow state.
type PendingTransferStatus string

const (
	PendingTransferPending  PendingTransferStatus = "pending"
	PendingTransferApproved PendingTransferStatus = "approved"
	PendingTransferRejected PendingTransferStatus = "rejected"
	PendingTransferExpired  PendingTransferStatus = "expired"
)

// PendingTransfer is a business transfer awaiting approval.
type PendingTransfer struct {
	ID            string                `json:"id"`
	BusinessID    string                `json:"businessId"`
	InitiatedBy   string                `json:"initiatedBy"`
	TransferType  string                `json:"transferType"`
	AmountCents   int64                 `json:"amountCents"`
	CurrencyCode  string                `json:"currencyCode"`
	RecipientInfo json.RawMessage       `json:"recipientInfo"`
	Status        PendingTransferStatus `json:"status"`
	Reason        *string               `json:"reason,omitempty"`
	ApprovedBy    *string               `json:"approvedBy,omitempty"`
	ApprovedAt    *time.Time            `json:"approvedAt,omitempty"`
	RejectedBy    *string               `json:"rejectedBy,omitempty"`
	RejectedAt    *time.Time            `json:"rejectedAt,omitempty"`
	ExpiresAt     time.Time             `json:"expiresAt"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
}

// BatchStatus tracks the lifecycle of a batch payment.
type BatchStatus string

const (
	BatchDraft      BatchStatus = "draft"
	BatchApproved   BatchStatus = "approved"
	BatchProcessing BatchStatus = "processing"
	BatchCompleted  BatchStatus = "completed"
	BatchPartial    BatchStatus = "partial"
	BatchFailed     BatchStatus = "failed"
)

// BatchItemStatus tracks individual items within a batch.
type BatchItemStatus string

const (
	BatchItemPending    BatchItemStatus = "pending"
	BatchItemProcessing BatchItemStatus = "processing"
	BatchItemCompleted  BatchItemStatus = "completed"
	BatchItemFailed     BatchItemStatus = "failed"
)

// BatchPayment represents a bulk payment group.
type BatchPayment struct {
	ID           string      `json:"id"`
	BusinessID   string      `json:"businessId"`
	Name         string      `json:"name"`
	TotalCents   int64       `json:"totalCents"`
	CurrencyCode string      `json:"currencyCode"`
	ItemCount    int         `json:"itemCount"`
	Status       BatchStatus `json:"status"`
	InitiatedBy  string      `json:"initiatedBy"`
	ApprovedBy   *string     `json:"approvedBy,omitempty"`
	ApprovedAt   *time.Time  `json:"approvedAt,omitempty"`
	ProcessedAt  *time.Time  `json:"processedAt,omitempty"`
	CompletedAt  *time.Time  `json:"completedAt,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

// BatchPaymentItem is a single recipient within a batch.
type BatchPaymentItem struct {
	ID               string          `json:"id"`
	BatchID          string          `json:"batchId"`
	RecipientName    string          `json:"recipientName"`
	RecipientPhone   *phone.PhoneNumber `json:"recipientPhone,omitempty"`
	RecipientBank    *string         `json:"recipientBank,omitempty"`
	RecipientAccount *string         `json:"recipientAccount,omitempty"`
	AmountCents      int64           `json:"amountCents"`
	Narration        *string         `json:"narration,omitempty"`
	CategoryID       *string         `json:"categoryId,omitempty"`
	Status           BatchItemStatus `json:"status"`
	TransactionID    *string         `json:"transactionId,omitempty"`
	ErrorMessage     *string         `json:"errorMessage,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// TransactionCategory organises transactions for bookkeeping.
type TransactionCategory struct {
	ID         string    `json:"id"`
	BusinessID *string   `json:"businessId,omitempty"`
	Name       string    `json:"name"`
	Color      *string   `json:"color,omitempty"`
	Icon       *string   `json:"icon,omitempty"`
	IsSystem   bool      `json:"isSystem"`
	CreatedAt  time.Time `json:"createdAt"`
}

// TransactionLabel attaches a category and notes to a transaction receipt.
type TransactionLabel struct {
	ID             string    `json:"id"`
	TransactionID  string    `json:"transactionId"`
	CategoryID     *string   `json:"categoryId,omitempty"`
	CustomLabel    *string   `json:"customLabel,omitempty"`
	Notes          *string   `json:"notes,omitempty"`
	TaggedBy       string    `json:"taggedBy"`
	TaxDeductible  bool      `json:"taxDeductible"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TaxType enumerates Ethiopian tax obligations.
type TaxType string

const (
	TaxVAT            TaxType = "vat"
	TaxIncome         TaxType = "income_tax"
	TaxWithholding    TaxType = "withholding_tax"
	TaxPension        TaxType = "pension"
	TaxExcise         TaxType = "excise"
	TaxCustomDuty     TaxType = "custom_duty"
	TaxOther          TaxType = "other"
)

var ValidTaxTypes = []string{
	string(TaxVAT), string(TaxIncome), string(TaxWithholding),
	string(TaxPension), string(TaxExcise), string(TaxCustomDuty), string(TaxOther),
}

// TaxPot links a regular pot to tax-specific metadata and auto-sweep rules.
type TaxPot struct {
	ID               string    `json:"id"`
	BusinessID       string    `json:"businessId"`
	PotID            string    `json:"potId"`
	TaxType          TaxType   `json:"taxType"`
	AutoSweepPercent *float64  `json:"autoSweepPercent,omitempty"`
	DueDate          *string   `json:"dueDate,omitempty"`
	Notes            *string   `json:"notes,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`

	// Hydrated from underlying pot at query time.
	Pot *Pot `json:"pot,omitempty"`
}

// InvoiceStatus tracks the invoice lifecycle.
type InvoiceStatus string

const (
	InvoiceDraft         InvoiceStatus = "draft"
	InvoiceSent          InvoiceStatus = "sent"
	InvoiceViewed        InvoiceStatus = "viewed"
	InvoicePartiallyPaid InvoiceStatus = "partially_paid"
	InvoicePaid          InvoiceStatus = "paid"
	InvoiceOverdue       InvoiceStatus = "overdue"
	InvoiceCancelled     InvoiceStatus = "cancelled"
)

// Invoice represents a business invoice to a customer.
type Invoice struct {
	ID             string        `json:"id"`
	BusinessID     string        `json:"businessId"`
	InvoiceNumber  string        `json:"invoiceNumber"`
	CustomerName   string        `json:"customerName"`
	CustomerPhone  *phone.PhoneNumber `json:"customerPhone,omitempty"`
	CustomerEmail  *string       `json:"customerEmail,omitempty"`
	CustomerUserID *string       `json:"customerUserId,omitempty"`
	CurrencyCode   string        `json:"currencyCode"`
	SubtotalCents  int64         `json:"subtotalCents"`
	TaxCents       int64         `json:"taxCents"`
	TotalCents     int64         `json:"totalCents"`
	PaidCents      int64         `json:"paidCents"`
	Status         InvoiceStatus `json:"status"`
	IssueDate      string        `json:"issueDate"`
	DueDate        string        `json:"dueDate"`
	Notes          *string       `json:"notes,omitempty"`
	PaymentLink    *string       `json:"paymentLink,omitempty"`
	CreatedBy      string        `json:"createdBy"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`

	LineItems []InvoiceLineItem `json:"lineItems,omitempty"`
}

// InvoiceLineItem is a single row on an invoice.
type InvoiceLineItem struct {
	ID             string    `json:"id"`
	InvoiceID      string    `json:"invoiceId"`
	Description    string    `json:"description"`
	Quantity       float64   `json:"quantity"`
	UnitPriceCents int64     `json:"unitPriceCents"`
	TotalCents     int64     `json:"totalCents"`
	CategoryID     *string   `json:"categoryId,omitempty"`
	SortOrder      int       `json:"sortOrder"`
	CreatedAt      time.Time `json:"createdAt"`
}

// DocumentType classifies a stored business document.
type DocumentType string

const (
	DocTradeLicense           DocumentType = "trade_license"
	DocTINCertificate         DocumentType = "tin_certificate"
	DocMemorandum             DocumentType = "memorandum"
	DocArticlesOfAssociation  DocumentType = "articles_of_association"
	DocBankStatement          DocumentType = "bank_statement"
	DocTaxReturn              DocumentType = "tax_return"
	DocContract               DocumentType = "contract"
	DocInvoiceAttachment      DocumentType = "invoice_attachment"
	DocReceipt                DocumentType = "receipt"
	DocIDDocument             DocumentType = "id_document"
	DocOther                  DocumentType = "other"
)

// BusinessDocument stores metadata for a file kept in object storage.
type BusinessDocument struct {
	ID            string       `json:"id"`
	BusinessID    string       `json:"businessId"`
	Name          string       `json:"name"`
	DocumentType  DocumentType `json:"documentType"`
	FileKey       string       `json:"fileKey"`
	FileSizeBytes int64        `json:"fileSizeBytes"`
	MimeType      string       `json:"mimeType"`
	UploadedBy    string       `json:"uploadedBy"`
	Description   *string      `json:"description,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
	IsArchived    bool         `json:"isArchived"`
	ExpiresAt     *string      `json:"expiresAt,omitempty"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// BusinessCreditProfile holds the credit assessment for a business entity.
type BusinessCreditProfile struct {
	BusinessID              string    `json:"businessId"`
	TrustScore              int       `json:"trustScore"`
	ApprovedLimitCents      int64     `json:"approvedLimitCents"`
	AvgMonthlyRevenueCents  int64     `json:"avgMonthlyRevenueCents"`
	AvgMonthlyExpensesCents int64     `json:"avgMonthlyExpensesCents"`
	CashFlowScore           int       `json:"cashFlowScore"`
	TimeInBusinessMonths    int       `json:"timeInBusinessMonths"`
	IndustryRiskScore       int       `json:"industryRiskScore"`
	TotalLoansRepaid        int       `json:"totalLoansRepaid"`
	LatePaymentsCount       int       `json:"latePaymentsCount"`
	CurrentOutstandingCents int64     `json:"currentOutstandingCents"`
	CollateralValueCents    int64     `json:"collateralValueCents"`
	IsNBEBlacklisted        bool      `json:"isNbeBlacklisted"`
	LastCalculatedAt        time.Time `json:"lastCalculatedAt"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

func (cp *BusinessCreditProfile) IsEligibleForLoan() bool {
	return cp.TrustScore > 600 && cp.ApprovedLimitCents > 0 && !cp.IsNBEBlacklisted
}

func (cp *BusinessCreditProfile) AvailableBorrowingCents() int64 {
	available := cp.ApprovedLimitCents - cp.CurrentOutstandingCents
	if available < 0 {
		return 0
	}
	return available
}

// BusinessLoan represents a credit facility extended to a business.
type BusinessLoan struct {
	ID                     string     `json:"id"`
	BusinessID             string     `json:"businessId"`
	PrincipalAmountCents   int64      `json:"principalAmountCents"`
	InterestFeeCents       int64      `json:"interestFeeCents"`
	TotalDueCents          int64      `json:"totalDueCents"`
	TotalPaidCents         int64      `json:"totalPaidCents"`
	DurationDays           int        `json:"durationDays"`
	DisbursedAt            time.Time  `json:"disbursedAt"`
	DueDate                time.Time  `json:"dueDate"`
	Status                 LoanStatus `json:"status"`
	DaysPastDue            int        `json:"daysPastDue"`
	Purpose                *string    `json:"purpose,omitempty"`
	CollateralDescription  *string    `json:"collateralDescription,omitempty"`
	LedgerLoanAccount      string     `json:"ledgerLoanAccount"`
	LedgerDisbursementTx   *string    `json:"ledgerDisbursementTx,omitempty"`
	AppliedBy              string     `json:"appliedBy"`
	ApprovedBy             *string    `json:"approvedBy,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

func (l *BusinessLoan) RemainingCents() int64 {
	return l.TotalDueCents - l.TotalPaidCents
}

// BusinessLoanInstallment is a scheduled repayment on a business loan.
type BusinessLoanInstallment struct {
	ID                string     `json:"id"`
	LoanID            string     `json:"loanId"`
	InstallmentNumber int        `json:"installmentNumber"`
	AmountDueCents    int64      `json:"amountDueCents"`
	AmountPaidCents   int64      `json:"amountPaidCents"`
	DueDate           time.Time  `json:"dueDate"`
	IsPaid            bool       `json:"isPaid"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	LedgerRepaymentTx *string   `json:"ledgerRepaymentTx,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// System default role IDs (deterministic UUIDs for seeding).
const (
	SystemRoleOwnerID      = "00000000-0000-0000-0000-000000000001"
	SystemRoleAdminID      = "00000000-0000-0000-0000-000000000002"
	SystemRoleFinanceID    = "00000000-0000-0000-0000-000000000003"
	SystemRoleAccountantID = "00000000-0000-0000-0000-000000000004"
	SystemRoleViewerID     = "00000000-0000-0000-0000-000000000005"
)
