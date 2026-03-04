package personal

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

// OverdraftHandler handles GET/POST overdraft endpoints.
type OverdraftHandler struct {
	svc *overdraft.Service
}

// NewOverdraftHandler creates a new overdraft HTTP handler.
func NewOverdraftHandler(svc *overdraft.Service) *OverdraftHandler {
	return &OverdraftHandler{svc: svc}
}

// Get returns the user's overdraft status and fee summary.
func (h *OverdraftHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	resp, err := h.svc.GetStatus(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// OptIn enables overdraft for the user.
func (h *OverdraftHandler) OptIn(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	o, err := h.svc.OptIn(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, o)
}

// OptOut disables overdraft when balance is zero.
func (h *OverdraftHandler) OptOut(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.OptOut(r.Context(), userID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "opted_out"})
}

// Repay handles manual overdraft repayment.
func (h *OverdraftHandler) Repay(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	ik := r.Header.Get(middleware.HeaderIdempotencyKey)
	if ik == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	var req overdraft.RepayRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Repay(r.Context(), userID, ik, req.AmountCents); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "repaid"})
}
