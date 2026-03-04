package ledger

import (
	"bytes"
	_ "embed"
	"strconv"
	"text/template"
)

// Embedded Numscript templates -- compiled into the binary.
var (
	//go:embed numscript/credit_wallet.numscript
	creditWalletScript string
	//go:embed numscript/debit_wallet.numscript
	debitWalletScript string
	//go:embed numscript/confirm_hold.numscript
	confirmHoldScript string
	//go:embed numscript/cancel_hold.numscript
	cancelHoldScript string
	//go:embed numscript/disburse_loan.numscript
	disburseLoanScript string
	//go:embed numscript/collect_repayment.numscript
	collectRepaymentScript string
	//go:embed numscript/convert_currency.numscript
	convertCurrencyScript string
)

// renderTemplate parses and executes a Numscript Go template.
func renderTemplate(tplStr string, data any) string {
	buf := bytes.NewBufferString("")
	tpl := template.Must(template.New("numscript").Funcs(template.FuncMap{
		"quote": strconv.Quote,
	}).Parse(tplStr))
	if err := tpl.Execute(buf, data); err != nil {
		panic("numscript template render failed: " + err.Error())
	}
	return buf.String()
}

// Posting represents a single Formance posting entry (source → destination for amount).
type Posting struct {
	Source      string
	Destination string
	Amount      int64
	Asset       string
}

// BuildCreditWalletScript renders the credit-wallet Numscript with the given sources.
func BuildCreditWalletScript(sources ...string) string {
	return renderTemplate(creditWalletScript, map[string]any{
		"Sources": sources,
	})
}

// BuildDebitWalletScript renders the debit-wallet Numscript with sources and
// optional account metadata to set on the hold account.
func BuildDebitWalletScript(metadata map[string]map[string]string, sources ...string) string {
	return renderTemplate(debitWalletScript, map[string]any{
		"Sources":  sources,
		"Metadata": metadata,
	})
}

// BuildConfirmHoldScript renders the confirm-hold Numscript.
// If final is true, any remaining held funds are returned to the wallet.
func BuildConfirmHoldScript(final bool, asset string) string {
	return renderTemplate(confirmHoldScript, map[string]any{
		"Final": final,
		"Asset": asset,
	})
}

// BuildCancelHoldScript renders the cancel-hold Numscript that reverses all
// postings from the original hold transaction.
func BuildCancelHoldScript(asset string, postings ...Posting) string {
	return renderTemplate(cancelHoldScript, map[string]any{
		"Asset":    asset,
		"Postings": postings,
	})
}

// BuildDisburseLoanScript renders the loan disbursement Numscript.
func BuildDisburseLoanScript(loanCapitalAccount string) string {
	return renderTemplate(disburseLoanScript, map[string]any{
		"LoanCapitalAccount": loanCapitalAccount,
	})
}

// BuildCollectRepaymentScript renders the loan repayment collection Numscript.
func BuildCollectRepaymentScript(loanCapitalAccount, interestAccount string) string {
	return renderTemplate(collectRepaymentScript, map[string]any{
		"LoanCapitalAccount": loanCapitalAccount,
		"InterestAccount":    interestAccount,
	})
}

// BuildConvertCurrencyScript renders the atomic FX conversion Numscript.
// A single transaction debits fromAsset from the user to the FX account
// and credits toAsset from @world to the user.
func BuildConvertCurrencyScript() string {
	return convertCurrencyScript
}
