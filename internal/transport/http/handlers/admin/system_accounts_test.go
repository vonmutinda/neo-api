package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/testutil"
	adminh "github.com/vonmutinda/neo/internal/transport/http/handlers/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

func TestSystemAccountsHandler_Get(t *testing.T) {
	chart := ledger.NewChart("neo")
	handler := adminh.NewSystemAccountsHandler(testutil.NewMockLedgerClient(), chart)

	r := chi.NewRouter()
	r.Use(middleware.AdminAuth(testutil.TestAdminJWTConfig()))
	r.Get("/system/accounts", handler.Get)

	req := testutil.NewAuthRequest(t, http.MethodGet, "http://test/system/accounts", nil,
		testutil.MustCreateAdminToken(t, "staff-1", domain.RoleTreasury))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			Pools []struct {
				Label        string `json:"label"`
				Account      string `json:"account"`
				BalanceCents int64  `json:"balanceCents"`
				Asset        string `json:"asset"`
			} `json:"pools"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body.Data.Pools, 2)
	require.Equal(t, "Loan capital", body.Data.Pools[0].Label)
	require.Equal(t, "Overdraft capital", body.Data.Pools[1].Label)
	require.Contains(t, body.Data.Pools[0].Account, "loan_capital")
	require.Contains(t, body.Data.Pools[1].Account, "overdraft_capital")
}
