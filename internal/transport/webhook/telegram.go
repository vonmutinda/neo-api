package webhook

import (
	"encoding/json"
	"net/http"

	tgclient "github.com/vonmutinda/neo/internal/gateway/telegram"
	tgsvc "github.com/vonmutinda/neo/internal/services/telegram"
	"github.com/vonmutinda/neo/pkg/httputil"
)

// TelegramWebhook handles incoming Telegram Bot API webhook POSTs.
type TelegramWebhook struct {
	svc *tgsvc.Service
}

func NewTelegramWebhook(svc *tgsvc.Service) *TelegramWebhook {
	return &TelegramWebhook{svc: svc}
}

func (h *TelegramWebhook) Handle(w http.ResponseWriter, r *http.Request) {
	var update tgclient.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid telegram update")
		return
	}

	if err := h.svc.HandleUpdate(r.Context(), update); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
