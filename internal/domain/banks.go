package domain

import "sort"

// BankInfo holds metadata for a financial institution in the EthSwitch network.
type BankInfo struct {
	InstitutionCode string `json:"institutionCode"`
	Name            string `json:"name"`
	SwiftBIC        string `json:"swiftBic"`
	CountryCode     string `json:"countryCode"`
}

// EthiopianBanks maps institution_code to bank metadata.
var EthiopianBanks = map[string]BankInfo{
	"CBE":       {InstitutionCode: "CBE", Name: "Commercial Bank of Ethiopia", SwiftBIC: "CBETETAA", CountryCode: "ET"},
	"DASHEN":    {InstitutionCode: "DASHEN", Name: "Dashen Bank", SwiftBIC: "DASHETAA", CountryCode: "ET"},
	"AWASH":     {InstitutionCode: "AWASH", Name: "Awash International Bank", SwiftBIC: "AWINETAA", CountryCode: "ET"},
	"ABYSSINIA": {InstitutionCode: "ABYSSINIA", Name: "Bank of Abyssinia", SwiftBIC: "AABORETAA", CountryCode: "ET"},
	"TELEBIRR":  {InstitutionCode: "TELEBIRR", Name: "Telebirr (Ethio Telecom)", SwiftBIC: "", CountryCode: "ET"},
	"COOP":      {InstitutionCode: "COOP", Name: "Cooperative Bank of Oromia", SwiftBIC: "CBORETAA", CountryCode: "ET"},
	"WEGAGEN":   {InstitutionCode: "WEGAGEN", Name: "Wegagen Bank", SwiftBIC: "WEBAETAA", CountryCode: "ET"},
	"UNITED":    {InstitutionCode: "UNITED", Name: "United Bank", SwiftBIC: "UNTDETAA", CountryCode: "ET"},
	"NIB":       {InstitutionCode: "NIB", Name: "Nib International Bank", SwiftBIC: "NIBIETET", CountryCode: "ET"},
	"ZEMEN":     {InstitutionCode: "ZEMEN", Name: "Zemen Bank", SwiftBIC: "ZEMEETAA", CountryCode: "ET"},
	"BUNNA":     {InstitutionCode: "BUNNA", Name: "Bunna International Bank", SwiftBIC: "BUNAETAA", CountryCode: "ET"},
	"BERHAN":    {InstitutionCode: "BERHAN", Name: "Berhan International Bank", SwiftBIC: "BEHAETAA", CountryCode: "ET"},
	"ABAY":      {InstitutionCode: "ABAY", Name: "Abay Bank", SwiftBIC: "ABAYETAA", CountryCode: "ET"},
	"ENAT":      {InstitutionCode: "ENAT", Name: "Enat Bank", SwiftBIC: "ABORETAA", CountryCode: "ET"},
	"OROMIA":    {InstitutionCode: "OROMIA", Name: "Oromia International Bank", SwiftBIC: "ORIRETAA", CountryCode: "ET"},
	"MPESA":     {InstitutionCode: "MPESA", Name: "M-PESA (Safaricom Ethiopia)", SwiftBIC: "", CountryCode: "ET"},
	"DBE":       {InstitutionCode: "DBE", Name: "Development Bank of Ethiopia", SwiftBIC: "DBETETAA", CountryCode: "ET"},
	"NEOBANK":   {InstitutionCode: "NEOBANK", Name: "Neo", SwiftBIC: "NEOBETET", CountryCode: "ET"},
}

// LookupBank returns bank metadata for an institution code, or nil if unknown.
func LookupBank(code string) *BankInfo {
	if b, ok := EthiopianBanks[code]; ok {
		return &b
	}
	return nil
}

// ListBanks returns all known banks sorted by name.
func ListBanks() []BankInfo {
	banks := make([]BankInfo, 0, len(EthiopianBanks))
	for _, b := range EthiopianBanks {
		banks = append(banks, b)
	}
	sort.Slice(banks, func(i, j int) bool {
		return banks[i].Name < banks[j].Name
	})
	return banks
}
