package recipient

import "github.com/vonmutinda/neo/internal/domain"

// TransferCounterparty carries the data needed to build a Recipient from a
// completed transfer. Constructed by the payments service after resolving the
// counterparty.
type TransferCounterparty struct {
	Type            domain.RecipientType
	DisplayName     string
	NeoUserID       string
	CountryCode     string
	Number          string
	Username        string
	InstitutionCode string
	AccountNumber   string
}
