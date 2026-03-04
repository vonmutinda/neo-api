package fayda

import "time"

// OTPRequest is sent to trigger an SMS to the user's Fayda-registered phone.
type OTPRequest struct {
	ID            string    `json:"id"`
	Version       string    `json:"version"`
	TransactionID string    `json:"transactionID"`
	RequestTime   time.Time `json:"requestTime"`
	IndividualID  string    `json:"individualId"`
}

// AuthRequest is sent with the OTP to retrieve decrypted KYC data.
type AuthRequest struct {
	ID            string    `json:"id"`
	Version       string    `json:"version"`
	TransactionID string    `json:"transactionID"`
	RequestTime   time.Time `json:"requestTime"`
	IndividualID  string    `json:"individualId"`
	PinValue      string    `json:"pinValue"`
}

// KYCResponse is the decrypted JWT payload from Fayda.
type KYCResponse struct {
	Status        string `json:"status"` // 'y' or 'n'
	TransactionID string `json:"transactionID"`
	Identity      struct {
		FullName string `json:"fullName"`
		DOB      string `json:"dob"`
		Gender   string `json:"gender"`
		Address  string `json:"address"`
		Photo    string `json:"photo"` // Base64 encoded JPEG
	} `json:"identity"`
}
