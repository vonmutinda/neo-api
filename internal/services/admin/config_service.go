package admin

import (
	"context"
	"encoding/json"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type ConfigService struct {
	configRepo repository.SystemConfigRepository
	auditRepo  repository.AuditRepository
}

func NewConfigService(configRepo repository.SystemConfigRepository, auditRepo repository.AuditRepository) *ConfigService {
	return &ConfigService{configRepo: configRepo, auditRepo: auditRepo}
}

func (s *ConfigService) ListAll(ctx context.Context) ([]domain.SystemConfig, error) {
	return s.configRepo.ListAll(ctx)
}

func (s *ConfigService) Get(ctx context.Context, key string) (*domain.SystemConfig, error) {
	return s.configRepo.Get(ctx, key)
}

func (s *ConfigService) IsEnabled(ctx context.Context, key string) (bool, error) {
	cfg, err := s.configRepo.Get(ctx, key)
	if err != nil {
		return false, err
	}
	return cfg.BoolValue(), nil
}

type UpdateConfigRequest struct {
	Value json.RawMessage `json:"value" validate:"required"`
}

func (s *ConfigService) Update(ctx context.Context, staffID, key string, req UpdateConfigRequest) error {
	if err := s.configRepo.Set(ctx, key, req.Value, &staffID); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]any{
		"key":   key,
		"value": req.Value,
	})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditConfigChanged,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "system_config",
		ResourceID:   key,
		Metadata:     meta,
	})
	return nil
}
