package admin

import (
	"net/http"

	adminsvc "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type ConfigHandler struct {
	svc *adminsvc.ConfigService
}

func NewConfigHandler(svc *adminsvc.ConfigService) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

func (h *ConfigHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	configs, err := h.svc.ListAll(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, configs)
}

func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	key := r.URL.Query().Get("key")
	if key == "" {
		httputil.WriteError(w, http.StatusBadRequest, "missing key parameter")
		return
	}
	var req adminsvc.UpdateConfigRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.svc.Update(r.Context(), staffID, key, req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
