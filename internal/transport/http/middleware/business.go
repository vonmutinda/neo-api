package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/permissions"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type businessContextKey struct{}
type memberContextKey struct{}

// BusinessFromContext extracts the business set by BusinessContext middleware.
func BusinessFromContext(ctx context.Context) *domain.Business {
	if v, ok := ctx.Value(businessContextKey{}).(*domain.Business); ok {
		return v
	}
	return nil
}

// MemberFromContext extracts the business member set by BusinessContext middleware.
func MemberFromContext(ctx context.Context) *domain.BusinessMember {
	if v, ok := ctx.Value(memberContextKey{}).(*domain.BusinessMember); ok {
		return v
	}
	return nil
}

// BusinessContext loads the business from the {id} URL parameter and verifies
// the authenticated user is an active member. Sets business and member in context.
func BusinessContext(
	bizRepo repository.BusinessRepository,
	memberRepo repository.BusinessMemberRepository,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bizID := chi.URLParam(r, "id")
			if bizID == "" {
				bizID = r.Header.Get("X-Business-ID")
			}
			if bizID == "" {
				httputil.WriteError(w, http.StatusBadRequest, "business ID is required")
				return
			}

			biz, err := bizRepo.GetByID(r.Context(), bizID)
			if err != nil {
				httputil.HandleError(w, r, err)
				return
			}
			if biz.IsFrozen {
				httputil.HandleError(w, r, domain.ErrBusinessFrozen)
				return
			}

			userID := UserIDFromContext(r.Context())
			member, err := memberRepo.GetByBusinessAndUser(r.Context(), bizID, userID)
			if err != nil {
				httputil.HandleError(w, r, domain.ErrForbidden)
				return
			}
			if !member.IsActive {
				httputil.HandleError(w, r, domain.ErrMemberNotActive)
				return
			}

			ctx := context.WithValue(r.Context(), businessContextKey{}, biz)
			ctx = context.WithValue(ctx, memberContextKey{}, member)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireBusinessPermission checks that the current member has the specified permission.
func RequireBusinessPermission(permSvc *permissions.Service, perm domain.BusinessPermission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			member := MemberFromContext(r.Context())
			biz := BusinessFromContext(r.Context())
			if member == nil || biz == nil {
				httputil.WriteError(w, http.StatusForbidden, "business context required")
				return
			}

			ok, err := permSvc.HasPermission(r.Context(), member.UserID, biz.ID, perm)
			if err != nil {
				httputil.HandleError(w, r, err)
				return
			}
			if !ok {
				httputil.WriteError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
