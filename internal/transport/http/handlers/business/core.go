package business

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	bizsvc "github.com/vonmutinda/neo/internal/services/business"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type BusinessHandler struct {
	svc *bizsvc.Service
}

func NewBusinessHandler(svc *bizsvc.Service) *BusinessHandler {
	return &BusinessHandler{svc: svc}
}

// --- Registration ---

func (h *BusinessHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req bizsvc.RegisterRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	biz, err := h.svc.RegisterBusiness(r.Context(), userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, biz)
}

// --- Business CRUD ---

func (h *BusinessHandler) Get(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	if biz == nil {
		httputil.WriteError(w, http.StatusNotFound, "business not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, biz)
}

func (h *BusinessHandler) Update(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	var req bizsvc.UpdateBusinessRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	updated, err := h.svc.UpdateBusiness(r.Context(), biz.ID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, updated)
}

func (h *BusinessHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	list, err := h.svc.ListMyBusinesses(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if list == nil {
		list = []domain.Business{}
	}
	httputil.WriteJSON(w, http.StatusOK, list)
}

// --- Member Management ---

func (h *BusinessHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	members, err := h.svc.ListMembers(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if members == nil {
		members = []domain.BusinessMember{}
	}
	httputil.WriteJSON(w, http.StatusOK, members)
}

func (h *BusinessHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	var req bizsvc.InviteMemberRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	member, err := h.svc.InviteMember(r.Context(), biz.ID, userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, member)
}

func (h *BusinessHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	memberID := chi.URLParam(r, "memberId")
	userID := middleware.UserIDFromContext(r.Context())
	var req bizsvc.UpdateMemberRoleRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.UpdateMemberRole(r.Context(), biz.ID, memberID, userID, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *BusinessHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	memberID := chi.URLParam(r, "memberId")
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.RemoveMember(r.Context(), biz.ID, memberID, userID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Role Management ---

func (h *BusinessHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	roles, err := h.svc.ListRoles(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if roles == nil {
		roles = []domain.BusinessRole{}
	}
	httputil.WriteJSON(w, http.StatusOK, roles)
}

func (h *BusinessHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	var req bizsvc.CreateRoleRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	role, err := h.svc.CreateRole(r.Context(), biz.ID, userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, role)
}

func (h *BusinessHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "roleId")
	role, err := h.svc.GetRole(r.Context(), roleID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, role)
}

func (h *BusinessHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	roleID := chi.URLParam(r, "roleId")
	userID := middleware.UserIDFromContext(r.Context())
	var req bizsvc.UpdateRoleRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	role, err := h.svc.UpdateRole(r.Context(), biz.ID, roleID, userID, &req)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, role)
}

func (h *BusinessHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	roleID := chi.URLParam(r, "roleId")
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.DeleteRole(r.Context(), biz.ID, roleID, userID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Permissions Introspection ---

func (h *BusinessHandler) GetMyPermissions(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	perms, err := h.svc.GetMyPermissions(r.Context(), userID, biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}
