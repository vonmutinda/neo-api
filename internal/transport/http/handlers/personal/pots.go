package personal

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/services/pots"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/money"
)

type PotHandler struct{ svc *pots.Service }

func NewPotHandler(svc *pots.Service) *PotHandler {
	return &PotHandler{svc: svc}
}

func (h *PotHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req pots.CreatePotRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	pot, err := h.svc.CreatePot(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, pot)
}

func (h *PotHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	list, err := h.svc.ListPots(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if list == nil {
		list = []domain.Pot{}
	}

	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *PotHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	potID := chi.URLParam(r, "id")

	pot, err := h.svc.GetPot(r.Context(), userID, potID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, pot)
}

func (h *PotHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	potID := chi.URLParam(r, "id")

	var req pots.UpdatePotRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	pot, err := h.svc.UpdatePot(r.Context(), userID, potID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, pot)
}

func (h *PotHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	potID := chi.URLParam(r, "id")

	returnedCents, currency, err := h.svc.ArchivePot(r.Context(), userID, potID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if returnedCents > 0 {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"archived":            true,
			"fundsReturned":       true,
			"amountReturnedCents": returnedCents,
			"currency":            currency,
			"display":             money.Display(returnedCents, currency) + " returned to your " + currency + " balance",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PotHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	potID := chi.URLParam(r, "id")

	var req pots.PotTransferRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	pot, err := h.svc.AddToPot(r.Context(), userID, potID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, pot)
}

func (h *PotHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	potID := chi.URLParam(r, "id")

	var req pots.PotTransferRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	pot, err := h.svc.WithdrawFromPot(r.Context(), userID, potID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, pot)
}

func (h *PotHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	potID := chi.URLParam(r, "id")

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	txs, err := h.svc.GetPotTransactions(r.Context(), userID, potID, limit)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, txs)
}
