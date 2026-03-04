package personal

import (
	"net/http"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type MeHandler struct {
	users repository.UserRepository
}

func NewMeHandler(users repository.UserRepository) *MeHandler {
	return &MeHandler{users: users}
}

func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, user)
}
