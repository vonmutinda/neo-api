package admin

import (
	"net/http"
	"time"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type ComplianceHandler struct {
	auditRepo repository.AuditRepository
	ruleRepo  repository.RegulatoryRuleRepository
}

func NewComplianceHandler(
	auditRepo repository.AuditRepository,
	ruleRepo repository.RegulatoryRuleRepository,
) *ComplianceHandler {
	return &ComplianceHandler{auditRepo: auditRepo, ruleRepo: ruleRepo}
}

func (h *ComplianceHandler) Report(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "from and to query params required (YYYY-MM-DD)")
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid from date format, use YYYY-MM-DD")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid to date format, use YYYY-MM-DD")
		return
	}

	rules, _ := h.ruleRepo.ListAll(r.Context())

	activeRules := make([]map[string]any, 0)
	for _, rule := range rules {
		if rule.EffectiveTo != nil && rule.EffectiveTo.Before(from) {
			continue
		}
		if rule.EffectiveFrom.After(to) {
			continue
		}
		activeRules = append(activeRules, map[string]any{
			"key":          rule.Key,
			"value":        rule.Value,
			"scope":        rule.Scope,
			"scopeValue":   rule.ScopeValue,
			"nbeReference": rule.NBEReference,
		})
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"from": from.Format("2006-01-02"),
			"to":   to.Format("2006-01-02"),
		},
		"activeRules": activeRules,
	})
}
