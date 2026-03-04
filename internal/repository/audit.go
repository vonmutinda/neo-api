package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
)

// safeMetadataKeys is the allow-list of keys that can appear in audit metadata.
// Any other key is redacted to prevent PII leaking into audit logs.
var safeMetadataKeys = map[string]bool{
	"action": true, "amount_cents": true, "currency": true,
	"status": true, "reason": true, "loan_id": true,
	"pot_id": true, "card_id": true, "hold_id": true,
	"transaction_id": true, "source": true, "destination": true,
	"business_id": true, "role_id": true, "role_name": true,
	"member_id": true, "member_user_id": true, "batch_id": true,
	"invoice_id": true, "transfer_type": true, "permission": true,
	"old_role_id": true, "new_role_id": true, "approver_id": true,
	"rule_key": true, "rule_value": true, "nbe_reference": true,
	"purpose": true, "beneficiary_id": true, "fx_rate": true,
	"from_currency": true, "to_currency": true, "spread": true,
	"merchant_currency": true, "direction": true,
}

func sanitizeMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	for k := range m {
		if !safeMetadataKeys[k] {
			m[k] = "[REDACTED]"
		}
	}
	sanitized, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return sanitized
}

type AuditRepository interface {
	Log(ctx context.Context, entry *domain.AuditEntry) error
	ListByResource(ctx context.Context, resourceType, resourceID string, limit int) ([]domain.AuditEntry, error)
}

type pgAuditRepo struct{ db DBTX }

func NewAuditRepository(db DBTX) AuditRepository { return &pgAuditRepo{db: db} }

func (r *pgAuditRepo) Log(ctx context.Context, e *domain.AuditEntry) error {
	e.Metadata = sanitizeMetadata(e.Metadata)
	query := `
		INSERT INTO audit_log (action, actor_type, actor_id, resource_type, resource_id, metadata, ip_address, user_agent,
			regulatory_rule_key, regulatory_action, nbe_reference)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		e.Action, e.ActorType, e.ActorID, e.ResourceType, e.ResourceID, e.Metadata, e.IPAddress, e.UserAgent,
		e.RegulatoryRuleKey, e.RegulatoryAction, e.NBEReference,
	).Scan(&e.ID, &e.CreatedAt)
}

func (r *pgAuditRepo) ListByResource(ctx context.Context, resourceType, resourceID string, limit int) ([]domain.AuditEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, action, actor_type, actor_id, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		FROM audit_log WHERE resource_type = $1 AND resource_id = $2 ORDER BY created_at DESC LIMIT $3`,
		resourceType, resourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()
	var result []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		if err := rows.Scan(&e.ID, &e.Action, &e.ActorType, &e.ActorID, &e.ResourceType, &e.ResourceID, &e.Metadata, &e.IPAddress, &e.UserAgent, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

var _ AuditRepository = (*pgAuditRepo)(nil)
