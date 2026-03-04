package webhooks

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type WiseWebhookHandler struct {
	transfers repository.RemittanceTransferRepository
	secret    string
}

func NewWiseWebhookHandler(transfers repository.RemittanceTransferRepository, secret string) *WiseWebhookHandler {
	return &WiseWebhookHandler{transfers: transfers, secret: secret}
}

func (h *WiseWebhookHandler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	// TODO: verify webhook signature using h.secret
	// TODO: parse Wise webhook payload, map status, update remittance_transfers
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "received"})
}
