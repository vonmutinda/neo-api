package personal

import (
	"context"
	"net/http"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type RecipientInfo struct {
	ID          string            `json:"id"`
	PhoneNumber phone.PhoneNumber `json:"phoneNumber"`
	Username    *string           `json:"username,omitempty"`
	FirstName   *string           `json:"firstName,omitempty"`
	LastName    *string           `json:"lastName,omitempty"`
}

type ResolveHandler struct {
	users repository.UserRepository
}

func NewResolveHandler(users repository.UserRepository) *ResolveHandler {
	return &ResolveHandler{users: users}
}

func (h *ResolveHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	identifier := strings.TrimSpace(r.URL.Query().Get("identifier"))
	if identifier == "" {
		httputil.WriteError(w, http.StatusBadRequest, "identifier query parameter is required")
		return
	}

	user, err := resolveUserByIdentifier(r.Context(), h.users, identifier)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, RecipientInfo{
		ID:          user.ID,
		PhoneNumber: user.PhoneNumber,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
	})
}

func resolveUserByIdentifier(ctx context.Context, users repository.UserRepository, identifier string) (*domain.User, error) {
	if p, err := phone.Parse(identifier); err == nil {
		user, userErr := users.GetByPhone(ctx, p)
		if userErr == nil {
			return user, nil
		}
	}

	return users.GetByUsername(ctx, identifier)
}
