package business

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type PendingTransferHandler struct {
	ptRepo repository.PendingTransferRepository
	audit  repository.AuditRepository
}

func NewPendingTransferHandler(ptRepo repository.PendingTransferRepository, audit repository.AuditRepository) *PendingTransferHandler {
	return &PendingTransferHandler{ptRepo: ptRepo, audit: audit}
}

func (h *PendingTransferHandler) List(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	transfers, err := h.ptRepo.ListPendingByBusiness(r.Context(), biz.ID, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, transfers)
}

func (h *PendingTransferHandler) Get(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "transferId")
	pt, err := h.ptRepo.GetByID(r.Context(), transferID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pt)
}

func (h *PendingTransferHandler) Approve(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "transferId")
	userID := middleware.UserIDFromContext(r.Context())
	biz := middleware.BusinessFromContext(r.Context())

	pt, err := h.ptRepo.GetByID(r.Context(), transferID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if pt.InitiatedBy == userID {
		httputil.HandleError(w, r, domain.ErrSelfApproval)
		return
	}

	if err := h.ptRepo.Approve(r.Context(), transferID, userID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	meta, _ := json.Marshal(map[string]string{
		"business_id":   biz.ID,
		"transfer_type": pt.TransferType,
		"approver_id":   userID,
	})
	_ = h.audit.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditBusinessTransferApproved,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "pending_transfer",
		ResourceID:   transferID,
		Metadata:     meta,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *PendingTransferHandler) Reject(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "transferId")
	userID := middleware.UserIDFromContext(r.Context())
	biz := middleware.BusinessFromContext(r.Context())

	var req struct {
		Reason string `json:"reason"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if req.Reason == "" {
		httputil.WriteError(w, http.StatusBadRequest, "reason is required")
		return
	}

	if err := h.ptRepo.Reject(r.Context(), transferID, userID, req.Reason); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	meta, _ := json.Marshal(map[string]string{
		"business_id": biz.ID,
		"reason":      req.Reason,
	})
	_ = h.audit.Log(r.Context(), &domain.AuditEntry{
		Action:       domain.AuditBusinessTransferRejected,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "pending_transfer",
		ResourceID:   transferID,
		Metadata:     meta,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *PendingTransferHandler) Count(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	count, err := h.ptRepo.CountPendingByBusiness(r.Context(), biz.ID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}
