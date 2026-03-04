package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type RegulatoryRuleRepository interface {
	GetEffective(ctx context.Context, key, scope, scopeValue string) (*domain.RegulatoryRule, error)
	ListEffectiveByKey(ctx context.Context, key string) ([]domain.RegulatoryRule, error)
	ListAll(ctx context.Context) ([]domain.RegulatoryRule, error)
	Create(ctx context.Context, rule *domain.RegulatoryRule) error
	Update(ctx context.Context, rule *domain.RegulatoryRule) error
	GetByID(ctx context.Context, id string) (*domain.RegulatoryRule, error)
}

type pgRegulatoryRuleRepo struct{ db DBTX }

func NewRegulatoryRuleRepository(db DBTX) RegulatoryRuleRepository {
	return &pgRegulatoryRuleRepo{db: db}
}

const ruleColumns = `id, key, description, value_type, value, scope, scope_value,
	nbe_reference, effective_from, effective_to, created_at, updated_at`

func scanRule(row pgx.Row) (*domain.RegulatoryRule, error) {
	var r domain.RegulatoryRule
	err := row.Scan(
		&r.ID, &r.Key, &r.Description, &r.ValueType, &r.Value,
		&r.Scope, &r.ScopeValue, &r.NBEReference,
		&r.EffectiveFrom, &r.EffectiveTo, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetEffective returns the currently active rule for a given key + scope + scopeValue.
func (repo *pgRegulatoryRuleRepo) GetEffective(ctx context.Context, key, scope, scopeValue string) (*domain.RegulatoryRule, error) {
	query := fmt.Sprintf(`SELECT %s FROM regulatory_rules
		WHERE key = $1 AND scope = $2 AND scope_value = $3
		  AND effective_from <= NOW()
		  AND (effective_to IS NULL OR effective_to > NOW())
		ORDER BY effective_from DESC LIMIT 1`, ruleColumns)

	r, err := scanRule(repo.db.QueryRow(ctx, query, key, scope, scopeValue))
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRegulatoryRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting effective rule %s/%s/%s: %w", key, scope, scopeValue, err)
	}
	return r, nil
}

// ListEffectiveByKey returns all currently active rules for a given key across all scopes.
func (repo *pgRegulatoryRuleRepo) ListEffectiveByKey(ctx context.Context, key string) ([]domain.RegulatoryRule, error) {
	query := fmt.Sprintf(`SELECT %s FROM regulatory_rules
		WHERE key = $1
		  AND effective_from <= NOW()
		  AND (effective_to IS NULL OR effective_to > NOW())
		ORDER BY scope, scope_value`, ruleColumns)

	rows, err := repo.db.Query(ctx, query, key)
	if err != nil {
		return nil, fmt.Errorf("listing effective rules for %s: %w", key, err)
	}
	defer rows.Close()

	var rules []domain.RegulatoryRule
	for rows.Next() {
		var r domain.RegulatoryRule
		if err := rows.Scan(
			&r.ID, &r.Key, &r.Description, &r.ValueType, &r.Value,
			&r.Scope, &r.ScopeValue, &r.NBEReference,
			&r.EffectiveFrom, &r.EffectiveTo, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (repo *pgRegulatoryRuleRepo) ListAll(ctx context.Context) ([]domain.RegulatoryRule, error) {
	query := fmt.Sprintf(`SELECT %s FROM regulatory_rules ORDER BY key, scope, effective_from DESC`, ruleColumns)
	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing all rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.RegulatoryRule
	for rows.Next() {
		var r domain.RegulatoryRule
		if err := rows.Scan(
			&r.ID, &r.Key, &r.Description, &r.ValueType, &r.Value,
			&r.Scope, &r.ScopeValue, &r.NBEReference,
			&r.EffectiveFrom, &r.EffectiveTo, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (repo *pgRegulatoryRuleRepo) Create(ctx context.Context, rule *domain.RegulatoryRule) error {
	query := `INSERT INTO regulatory_rules
		(key, description, value_type, value, scope, scope_value, nbe_reference, effective_from, effective_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`
	return repo.db.QueryRow(ctx, query,
		rule.Key, rule.Description, rule.ValueType, rule.Value,
		rule.Scope, rule.ScopeValue, rule.NBEReference,
		rule.EffectiveFrom, rule.EffectiveTo,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
}

func (repo *pgRegulatoryRuleRepo) Update(ctx context.Context, rule *domain.RegulatoryRule) error {
	query := `UPDATE regulatory_rules
		SET description = $2, value = $3, effective_to = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`
	return repo.db.QueryRow(ctx, query,
		rule.ID, rule.Description, rule.Value, rule.EffectiveTo,
	).Scan(&rule.UpdatedAt)
}

func (repo *pgRegulatoryRuleRepo) GetByID(ctx context.Context, id string) (*domain.RegulatoryRule, error) {
	query := fmt.Sprintf(`SELECT %s FROM regulatory_rules WHERE id = $1`, ruleColumns)
	r, err := scanRule(repo.db.QueryRow(ctx, query, id))
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRegulatoryRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting rule by id: %w", err)
	}
	return r, nil
}

var _ RegulatoryRuleRepository = (*pgRegulatoryRuleRepo)(nil)
