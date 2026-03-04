package domain

import "time"

// RuleValueType describes how to parse a regulatory rule's value.
type RuleValueType string

const (
	RuleTypeAmountCents RuleValueType = "amount_cents"
	RuleTypeBool        RuleValueType = "bool"
	RuleTypePercent     RuleValueType = "percent"
	RuleTypeEnum        RuleValueType = "enum"
	RuleTypeDuration    RuleValueType = "duration"
)

// RuleScope determines which user segment a rule applies to.
type RuleScope string

const (
	RuleScopeGlobal      RuleScope = "global"
	RuleScopeKYCLevel    RuleScope = "kyc_level"
	RuleScopeAccountType RuleScope = "account_type"
	RuleScopeCurrency    RuleScope = "currency"
)

// RegulatoryRule is a config-driven compliance constraint stored in Postgres.
// Rules are versioned by effective date so historical lookups are possible.
type RegulatoryRule struct {
	ID            string        `json:"id"`
	Key           string        `json:"key"`
	Description   string        `json:"description"`
	ValueType     RuleValueType `json:"valueType"`
	Value         string        `json:"value"`
	Scope         RuleScope     `json:"scope"`
	ScopeValue    string        `json:"scopeValue"`
	EffectiveFrom time.Time     `json:"effectiveFrom"`
	EffectiveTo   *time.Time    `json:"effectiveTo"`
	NBEReference  string        `json:"nbeReference"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// TransferPurpose classifies the reason for an outbound transfer (NBE Clause 8/15).
type TransferPurpose string

const (
	PurposeGeneral    TransferPurpose = "general"
	PurposeMedical    TransferPurpose = "medical"
	PurposeEducation  TransferPurpose = "education"
	PurposeTravel     TransferPurpose = "travel"
	PurposeFamily     TransferPurpose = "family_support"
	PurposeInvestment TransferPurpose = "investment"
	PurposeTrade      TransferPurpose = "trade"
)

var ValidTransferPurposes = []string{
	string(PurposeGeneral), string(PurposeMedical), string(PurposeEducation),
	string(PurposeTravel), string(PurposeFamily), string(PurposeInvestment), string(PurposeTrade),
}

// FXSource tracks how foreign currency was obtained (NBE reporting requirement).
type FXSource string

const (
	FXSourceRemittance   FXSource = "remittance"
	FXSourceExportRetain FXSource = "export_retention"
	FXSourceGrant        FXSource = "grant_gift"
	FXSourceConversion   FXSource = "conversion"
	FXSourceDeposit      FXSource = "cash_deposit"
)

// SpendWaterfall is the user's preferred currency drain order for card transactions.
// The magic value "merchant_currency" means "try the merchant's currency first".
type SpendWaterfall []string

var DefaultSpendWaterfall = SpendWaterfall{"merchant_currency", "ETB"}

// BeneficiaryRelType classifies the relationship for family payments (NBE Clause 3).
type BeneficiaryRelType string

const (
	BeneficiarySpouse BeneficiaryRelType = "spouse"
	BeneficiaryChild  BeneficiaryRelType = "child"
	BeneficiaryParent BeneficiaryRelType = "parent"
)

// Beneficiary is a registered family member for FX payments (Clause 3).
type Beneficiary struct {
	ID           string             `json:"id"`
	UserID       string             `json:"userId"`
	FullName     string             `json:"fullName"`
	Relationship BeneficiaryRelType `json:"relationship"`
	DocumentURL  *string            `json:"documentUrl,omitempty"`
	IsVerified   bool               `json:"isVerified"`
	CreatedAt    time.Time          `json:"createdAt"`
	DeletedAt    *time.Time         `json:"-"`
}
