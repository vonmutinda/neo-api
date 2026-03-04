package admin

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type RulesHandler struct {
	ruleRepo   repository.RegulatoryRuleRepository
	auditRepo  repository.AuditRepository
	regulatory *regulatory.Service
}

func NewRulesHandler(
	ruleRepo repository.RegulatoryRuleRepository,
	auditRepo repository.AuditRepository,
	regulatorySvc *regulatory.Service,
) *RulesHandler {
	return &RulesHandler{
		ruleRepo:   ruleRepo,
		auditRepo:  auditRepo,
		regulatory: regulatorySvc,
	}
}

func (h *RulesHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.ruleRepo.ListAll(r.Context())
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	if rules == nil {
		rules = []domain.RegulatoryRule{}
	}
	httputil.WriteJSON(w, http.StatusOK, rules)
}

func (h *RulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.ruleRepo.GetByID(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, rule)
}

func (h *RulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key           string  `json:"key"`
		Description   string  `json:"description"`
		ValueType     string  `json:"valueType"`
		Value         string  `json:"value"`
		Scope         string  `json:"scope"`
		ScopeValue    string  `json:"scopeValue"`
		NBEReference  string  `json:"nbeReference"`
		EffectiveFrom *string `json:"effectiveFrom"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if req.Key == "" || req.Value == "" || req.ValueType == "" {
		httputil.WriteError(w, http.StatusBadRequest, "key, value, and valueType are required")
		return
	}

	effectiveFrom := time.Now()
	if req.EffectiveFrom != nil {
		parsed, err := time.Parse(time.RFC3339, *req.EffectiveFrom)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid effectiveFrom format, use RFC3339")
			return
		}
		effectiveFrom = parsed
	}

	rule := &domain.RegulatoryRule{
		Key:           req.Key,
		Description:   req.Description,
		ValueType:     domain.RuleValueType(req.ValueType),
		Value:         req.Value,
		Scope:         domain.RuleScope(req.Scope),
		ScopeValue:    req.ScopeValue,
		NBEReference:  req.NBEReference,
		EffectiveFrom: effectiveFrom,
	}

	if err := h.ruleRepo.Create(r.Context(), rule); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	h.regulatory.InvalidateCache()

	actorID := middleware.UserIDFromContext(r.Context())
	_ = h.auditRepo.Log(r.Context(), &domain.AuditEntry{
		Action:            domain.AuditRuleUpdated,
		ActorType:         "admin",
		ActorID:           &actorID,
		ResourceType:      "regulatory_rule",
		ResourceID:        rule.ID,
		RegulatoryRuleKey: &rule.Key,
	})

	httputil.WriteJSON(w, http.StatusCreated, rule)
}

func (h *RulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.ruleRepo.GetByID(r.Context(), id)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	var req struct {
		Description *string `json:"description"`
		Value       *string `json:"value"`
		EffectiveTo *string `json:"effectiveTo"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Value != nil {
		existing.Value = *req.Value
	}
	if req.EffectiveTo != nil {
		parsed, err := time.Parse(time.RFC3339, *req.EffectiveTo)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid effectiveTo format, use RFC3339")
			return
		}
		existing.EffectiveTo = &parsed
	}

	if err := h.ruleRepo.Update(r.Context(), existing); err != nil {
		httputil.HandleError(w, r, err)
		return
	}

	h.regulatory.InvalidateCache()

	actorID := middleware.UserIDFromContext(r.Context())
	_ = h.auditRepo.Log(r.Context(), &domain.AuditEntry{
		Action:            domain.AuditRuleUpdated,
		ActorType:         "admin",
		ActorID:           &actorID,
		ResourceType:      "regulatory_rule",
		ResourceID:        existing.ID,
		RegulatoryRuleKey: &existing.Key,
	})

	httputil.WriteJSON(w, http.StatusOK, existing)
}
