package personal

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/services/payment_requests"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type PaymentRequestHandler struct {
	svc *payment_requests.Service
}

func NewPaymentRequestHandler(svc *payment_requests.Service) *PaymentRequestHandler {
	return &PaymentRequestHandler{svc: svc}
}

func (h *PaymentRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req payment_requests.CreatePaymentRequestForm
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	pr, err := h.svc.Create(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, pr)
}

func (h *PaymentRequestHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req payment_requests.BatchPaymentRequestForm
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	results, err := h.svc.CreateBatch(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, results)
}

func (h *PaymentRequestHandler) ListSent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	list, err := h.svc.ListSent(r.Context(), userID, limit, offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if list == nil {
		list = []domain.PaymentRequest{}
	}
	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *PaymentRequestHandler) ListReceived(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	list, err := h.svc.ListReceived(r.Context(), userID, limit, offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if list == nil {
		list = []domain.PaymentRequest{}
	}
	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *PaymentRequestHandler) PendingCount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	count, err := h.svc.PendingCount(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *PaymentRequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	pr, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pr)
}

func (h *PaymentRequestHandler) Pay(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.Pay(r.Context(), userID, id); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "paid"})
}

func (h *PaymentRequestHandler) Decline(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var form payment_requests.DeclineForm
	_ = httputil.DecodeJSON(r, &form)
	if err := h.svc.Decline(r.Context(), userID, id, &form); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentRequestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.Cancel(r.Context(), userID, id); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentRequestHandler) Remind(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.Remind(r.Context(), userID, id); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
