package personal

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/recipient"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

// RecipientHandler exposes recipient management endpoints.
type RecipientHandler struct {
	svc *recipient.Service
}

func NewRecipientHandler(svc *recipient.Service) *RecipientHandler {
	return &RecipientHandler{svc: svc}
}

type createRecipientRequest struct {
	Type            domain.RecipientType `json:"type"`
	Identifier      string               `json:"identifier"`
	InstitutionCode string               `json:"institutionCode"`
	AccountNumber   string               `json:"accountNumber"`
	DisplayName     string               `json:"displayName"`
}

// Create handles POST /v1/recipients
func (h *RecipientHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req createRecipientRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	rec, err := h.svc.Create(r.Context(), userID, recipient.CreateRequest{
		Type:            req.Type,
		Identifier:      req.Identifier,
		InstitutionCode: req.InstitutionCode,
		AccountNumber:   req.AccountNumber,
		DisplayName:     req.DisplayName,
	})
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, rec)
}

type recipientListResponse struct {
	Recipients []domain.Recipient `json:"recipients"`
	Total      int                `json:"total"`
}

// List handles GET /v1/recipients
func (h *RecipientHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	filter := repository.RecipientFilter{
		Query: r.URL.Query().Get("q"),
		Limit: intQueryParam(r, "limit", 20),
		Offset: intQueryParam(r, "offset", 0),
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = &t
	}
	if f := r.URL.Query().Get("favorite"); f != "" {
		b := f == "true"
		filter.Favorite = &b
	}

	list, total, err := h.svc.List(r.Context(), userID, filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if list == nil {
		list = []domain.Recipient{}
	}

	httputil.WriteJSON(w, http.StatusOK, recipientListResponse{
		Recipients: list,
		Total:      total,
	})
}

// Get handles GET /v1/recipients/{id}
func (h *RecipientHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	rec, err := h.svc.GetByID(r.Context(), userID, id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, rec)
}

// SearchByBank handles GET /v1/recipients/search/bank?institution=CBE&account=1234
func (h *RecipientHandler) SearchByBank(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	institution := r.URL.Query().Get("institution")
	account := r.URL.Query().Get("account")

	if institution == "" || len(account) < 4 {
		httputil.WriteError(w, http.StatusBadRequest, "institution is required and account must be at least 4 characters")
		return
	}

	results, err := h.svc.SearchByBankAccount(r.Context(), userID, institution, account)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if results == nil {
		results = []domain.Recipient{}
	}

	httputil.WriteJSON(w, http.StatusOK, results)
}

// SearchByName handles GET /v1/recipients/search/name?q=Abebe
func (h *RecipientHandler) SearchByName(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	q := r.URL.Query().Get("q")

	if q == "" {
		httputil.WriteError(w, http.StatusBadRequest, "q query parameter is required")
		return
	}

	results, err := h.svc.SearchByName(r.Context(), userID, q)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if results == nil {
		results = []domain.Recipient{}
	}

	httputil.WriteJSON(w, http.StatusOK, results)
}

type setFavoriteRequest struct {
	IsFavorite bool `json:"isFavorite"`
}

// SetFavorite handles PATCH /v1/recipients/{id}/favorite
func (h *RecipientHandler) SetFavorite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req setFavoriteRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if err := h.svc.SetFavorite(r.Context(), userID, id, req.IsFavorite); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Archive handles DELETE /v1/recipients/{id}
func (h *RecipientHandler) Archive(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.Archive(r.Context(), userID, id); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListBanks handles GET /v1/banks
func (h *RecipientHandler) ListBanks(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, domain.ListBanks())
}

func intQueryParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
