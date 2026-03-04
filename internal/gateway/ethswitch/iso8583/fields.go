package iso8583

import (
	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/padding"
	"github.com/moov-io/iso8583/prefix"
)

// Spec defines the ISO 8583 v1987 ASCII message specification used by
// SmartVista / EthSwitch for card authorization routing.
//
// Only the data elements (DEs) relevant to Ethiopian card processing are
// included. Additional DEs can be added as EthSwitch requirements expand.
var Spec *iso8583.MessageSpec = &iso8583.MessageSpec{
	Name: "EthSwitch SmartVista Card Authorization",
	Fields: map[int]field.Field{
		// DE 0: Message Type Indicator (0100=Auth, 0110=AuthResp, 0200=Financial, etc.)
		0: field.NewString(&field.Spec{
			Length:      4,
			Description: "Message Type Indicator",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 2: Primary Account Number (tokenized for PCI compliance)
		2: field.NewString(&field.Spec{
			Length:      19,
			Description: "Primary Account Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LL,
		}),
		// DE 3: Processing Code (00=Purchase, 01=ATM, 20=Refund)
		3: field.NewNumeric(&field.Spec{
			Length:      6,
			Description: "Processing Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 4: Transaction Amount (in minor units / cents)
		4: field.NewNumeric(&field.Spec{
			Length:      12,
			Description: "Transaction Amount",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 7: Transmission Date & Time (MMDDhhmmss)
		7: field.NewString(&field.Spec{
			Length:      10,
			Description: "Transmission Date and Time",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 11: System Trace Audit Number (STAN)
		11: field.NewNumeric(&field.Spec{
			Length:      6,
			Description: "System Trace Audit Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 12: Local Transaction Time (hhmmss)
		12: field.NewString(&field.Spec{
			Length:      6,
			Description: "Local Transaction Time",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 13: Local Transaction Date (MMDD)
		13: field.NewString(&field.Spec{
			Length:      4,
			Description: "Local Transaction Date",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 14: Expiration Date (YYMM)
		14: field.NewString(&field.Spec{
			Length:      4,
			Description: "Expiration Date",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 18: Merchant Category Code (MCC)
		18: field.NewNumeric(&field.Spec{
			Length:      4,
			Description: "Merchant Category Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 22: Point of Service Entry Mode
		22: field.NewString(&field.Spec{
			Length:      3,
			Description: "POS Entry Mode",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 23: Card Sequence Number
		23: field.NewNumeric(&field.Spec{
			Length:      3,
			Description: "Card Sequence Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 25: Point of Service Condition Code
		25: field.NewNumeric(&field.Spec{
			Length:      2,
			Description: "POS Condition Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 32: Acquiring Institution ID
		32: field.NewString(&field.Spec{
			Length:      11,
			Description: "Acquiring Institution ID",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LL,
		}),
		// DE 35: Track 2 Data (for chip/magstripe - encrypted)
		35: field.NewString(&field.Spec{
			Length:      37,
			Description: "Track 2 Data",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LL,
		}),
		// DE 37: Retrieval Reference Number
		37: field.NewString(&field.Spec{
			Length:      12,
			Description: "Retrieval Reference Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 38: Authorization ID Response
		38: field.NewString(&field.Spec{
			Length:      6,
			Description: "Authorization ID Response",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 39: Response Code
		39: field.NewString(&field.Spec{
			Length:      2,
			Description: "Response Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 41: Card Acceptor Terminal ID
		41: field.NewString(&field.Spec{
			Length:      8,
			Description: "Card Acceptor Terminal ID",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 42: Card Acceptor ID
		42: field.NewString(&field.Spec{
			Length:      15,
			Description: "Card Acceptor ID Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 43: Card Acceptor Name/Location
		43: field.NewString(&field.Spec{
			Length:      40,
			Description: "Card Acceptor Name/Location",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// DE 49: Currency Code, Transaction (840=USD, 230=ETB)
		49: field.NewNumeric(&field.Spec{
			Length:      3,
			Description: "Currency Code, Transaction",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
			Pad:         padding.Left('0'),
		}),
		// DE 54: Additional Amounts
		54: field.NewString(&field.Spec{
			Length:      120,
			Description: "Additional Amounts",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
	},
}

// AuthorizationRequest represents the data carried in an 0100 Authorization
// Request message. Struct tags map to ISO 8583 data elements.
type AuthorizationRequest struct {
	MTI                     string `iso8583:"0"`
	PAN                     string `iso8583:"2"`
	ProcessingCode          string `iso8583:"3"`
	TransactionAmount       int64  `iso8583:"4"`
	TransmissionDateTime    string `iso8583:"7"`
	STAN                    string `iso8583:"11"`
	LocalTime               string `iso8583:"12"`
	LocalDate               string `iso8583:"13"`
	ExpirationDate          string `iso8583:"14"`
	MCC                     string `iso8583:"18"`
	POSEntryMode            string `iso8583:"22"`
	AcquiringInstitutionID  string `iso8583:"32"`
	RRN                     string `iso8583:"37"`
	TerminalID              string `iso8583:"41"`
	MerchantID              string `iso8583:"42"`
	MerchantNameLocation    string `iso8583:"43"`
	CurrencyCode            string `iso8583:"49"`
}

// AuthorizationResponse represents the data carried in an 0110 Authorization
// Response message.
type AuthorizationResponse struct {
	MTI                     string `iso8583:"0"`
	PAN                     string `iso8583:"2"`
	ProcessingCode          string `iso8583:"3"`
	TransactionAmount       int64  `iso8583:"4"`
	TransmissionDateTime    string `iso8583:"7"`
	STAN                    string `iso8583:"11"`
	LocalTime               string `iso8583:"12"`
	LocalDate               string `iso8583:"13"`
	AcquiringInstitutionID  string `iso8583:"32"`
	RRN                     string `iso8583:"37"`
	AuthorizationCode       string `iso8583:"38"`
	ResponseCode            string `iso8583:"39"`
	TerminalID              string `iso8583:"41"`
	MerchantID              string `iso8583:"42"`
}
