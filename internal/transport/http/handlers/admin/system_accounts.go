package admin

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/pkg/httputil"
)

const assetETB = "ETB"

// SystemAccountsHandler serves GET /admin/v1/system/accounts (capital pools).
type SystemAccountsHandler struct {
	ledger ledger.Client
	chart  *ledger.Chart
}

// NewSystemAccountsHandler creates a new system accounts handler.
func NewSystemAccountsHandler(ledgerClient ledger.Client, chart *ledger.Chart) *SystemAccountsHandler {
	return &SystemAccountsHandler{ledger: ledgerClient, chart: chart}
}

// CapitalPool is a single system capital pool for the admin view.
type CapitalPool struct {
	Label        string `json:"label"`
	Account      string `json:"account"`
	BalanceCents int64  `json:"balanceCents"`
	Asset        string `json:"asset"`
}

// Get returns the list of system capital pools (loan, overdraft).
func (h *SystemAccountsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pools := []CapitalPool{
		{Label: "Loan capital", Account: h.chart.SystemLoanCapital(), Asset: assetETB},
		{Label: "Overdraft capital", Account: h.chart.SystemOverdraftCapital(), Asset: assetETB},
	}

	for i := range pools {
		bal, err := h.ledger.GetAccountBalance(ctx, pools[i].Account, pools[i].Asset)
		if err != nil {
			httputil.HandleError(w, r, err)
			return
		}
		pools[i].BalanceCents = bal.Int64()
		if pools[i].BalanceCents < 0 {
			pools[i].BalanceCents = 0
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"pools": pools})
}
