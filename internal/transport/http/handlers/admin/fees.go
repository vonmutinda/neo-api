package admin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type FeeHandler struct {
	scheduleRepo repository.FeeScheduleRepository
	providerRepo repository.RemittanceProviderRepository
	auditRepo    repository.AuditRepository
	cache        cache.Cache
}

func NewFeeHandler(
	scheduleRepo repository.FeeScheduleRepository,
	providerRepo repository.RemittanceProviderRepository,
	auditRepo repository.AuditRepository,
	c cache.Cache,
) *FeeHandler {
	return &FeeHandler{
		scheduleRepo: scheduleRepo,
		providerRepo: providerRepo,
		auditRepo:    auditRepo,
		cache:        c,
	}
}

func (h *FeeHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.scheduleRepo.ListAll(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if schedules == nil {
		schedules = []domain.FeeSchedule{}
	}
	httputil.WriteJSON(w, http.StatusOK, schedules)
}

func (h *FeeHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	var fs domain.FeeSchedule
	if err := httputil.DecodeJSON(r, &fs); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	fs.IsActive = true
	if err := h.scheduleRepo.Create(r.Context(), &fs); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	meta, _ := json.Marshal(map[string]any{"id": fs.ID, "name": fs.Name})
	_ = h.auditRepo.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditFeeScheduleCreated,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "fee_schedule",
		ResourceID:   fs.ID,
		Metadata:     meta,
	})

	_ = h.cache.DeleteByPrefix(r.Context(), "neo:fees:")
	httputil.WriteJSON(w, http.StatusCreated, fs)
}

func (h *FeeHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var fs domain.FeeSchedule
	if err := httputil.DecodeJSON(r, &fs); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	fs.ID = id
	if err := h.scheduleRepo.Update(r.Context(), &fs); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	meta, _ := json.Marshal(map[string]any{"id": id, "name": fs.Name})
	_ = h.auditRepo.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditFeeScheduleUpdated,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "fee_schedule",
		ResourceID:   id,
		Metadata:     meta,
	})

	_ = h.cache.DeleteByPrefix(r.Context(), "neo:fees:")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *FeeHandler) DeactivateSchedule(w http.ResponseWriter, r *http.Request) {
	staffID := middleware.StaffIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.scheduleRepo.Deactivate(r.Context(), id); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	meta, _ := json.Marshal(map[string]any{"id": id})
	_ = h.auditRepo.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditFeeScheduleDisabled,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "fee_schedule",
		ResourceID:   id,
		Metadata:     meta,
	})

	_ = h.cache.DeleteByPrefix(r.Context(), "neo:fees:")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

func (h *FeeHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.providerRepo.ListActive(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if providers == nil {
		providers = []repository.RemittanceProviderRow{}
	}
	httputil.WriteJSON(w, http.StatusOK, providers)
}

func (h *FeeHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		IsActive bool `json:"isActive"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if err := h.providerRepo.UpdateStatus(r.Context(), id, req.IsActive); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
