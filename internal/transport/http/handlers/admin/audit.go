package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type AuditHandler struct {
	adminRepo repository.AdminQueryRepository
}

func NewAuditHandler(adminRepo repository.AdminQueryRepository) *AuditHandler {
	return &AuditHandler{adminRepo: adminRepo}
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := parseAuditFilter(r.URL.Query())
	result, err := h.adminRepo.ListAuditEntries(r.Context(), filter)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *AuditHandler) Get(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "id")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "not_implemented"})
}
