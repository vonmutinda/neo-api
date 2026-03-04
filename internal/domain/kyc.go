package domain

import "time"

type KYCVerificationStatus string

const (
	KYCStatusPending  KYCVerificationStatus = "pending"
	KYCStatusVerified KYCVerificationStatus = "verified"
	KYCStatusFailed   KYCVerificationStatus = "failed"
	KYCStatusExpired  KYCVerificationStatus = "expired"
)

// KYCVerification is an audit record of a single Fayda eKYC attempt.
// NBE requires proof that the user consented to biometric verification.
type KYCVerification struct {
	ID                  string                `json:"id"`
	UserID              string                `json:"userId"`
	FaydaFIN            string                `json:"faydaFin"`
	FaydaTransactionID  string                `json:"faydaTransactionId"`
	Status              KYCVerificationStatus `json:"status"`
	VerifiedAt          *time.Time            `json:"verifiedAt,omitempty"`
	FaydaExpiryDate     *time.Time            `json:"faydaExpiryDate,omitempty"`
	RawResponseHash     *string               `json:"rawResponseHash,omitempty"`
	CreatedAt           time.Time             `json:"createdAt"`
}
