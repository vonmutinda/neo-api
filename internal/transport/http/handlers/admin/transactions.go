package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type TransactionHandler struct {
	svc *adminsvc.TransactionService
}

func NewTransactionHandler(svc *adminsvc.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := parseTransactionFilter(r.URL.Query())
	result, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *TransactionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	txn, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, txn)
}

func (h *TransactionHandler) GetConversion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	view, err := h.svc.GetConversion(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, view)
}

func (h *TransactionHandler) Reverse(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	txnID := chi.URLParam(r, "id")
	var req adminsvc.ReverseRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Reverse(r.Context(), staffID, txnID, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "reversed"})
}
