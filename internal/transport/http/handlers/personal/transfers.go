package personal

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/services/payments"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/money"
)

type TransferHandler struct{ svc *payments.Service }

func NewTransferHandler(svc *payments.Service) *TransferHandler {
	return &TransferHandler{svc: svc}
}

// Outbound handles external transfers via EthSwitch.
// Accepts a currency field; defaults to ETB if omitted.
func (h *TransferHandler) Outbound(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req payments.OutboundTransferRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if req.Currency == "" {
		req.Currency = money.CurrencyETB
	}
	if err := h.svc.ProcessOutboundTransfer(r.Context(), userID, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// Inbound handles wallet-to-wallet P2P transfers between neobank users.
func (h *TransferHandler) Inbound(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req payments.InboundTransferRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if req.Currency == "" {
		req.Currency = money.CurrencyETB
	}
	if err := h.svc.ProcessInboundTransfer(r.Context(), userID, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// Batch handles multi-recipient P2P transfers.
func (h *TransferHandler) Batch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req payments.BatchTransferRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if req.Currency == "" {
		req.Currency = money.CurrencyETB
	}
	result, err := h.svc.ProcessBatchTransfer(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}
