package personal

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/services/onboarding"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type OnboardingHandler struct{ svc *onboarding.Service }

func NewOnboardingHandler(svc *onboarding.Service) *OnboardingHandler {
	return &OnboardingHandler{svc: svc}
}

func (h *OnboardingHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req onboarding.RegisterRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	user, err := h.svc.RegisterUser(r.Context(), &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, user)
}

func (h *OnboardingHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req onboarding.KYCOTPRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	txID, err := h.svc.RequestOTP(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"transactionId": txID})
}

func (h *OnboardingHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req onboarding.KYCVerifyRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.VerifyOTP(r.Context(), userID, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}
