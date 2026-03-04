package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/password"
)

type AuthService struct {
	staffRepo repository.StaffRepository
	auditRepo repository.AuditRepository
	jwtSecret []byte
	jwtIssuer string
	jwtAud    string
	tokenTTL  time.Duration
}

func NewAuthService(
	staffRepo repository.StaffRepository,
	auditRepo repository.AuditRepository,
	jwtSecret string,
	jwtIssuer, jwtAud string,
) *AuthService {
	return &AuthService{
		staffRepo: staffRepo,
		auditRepo: auditRepo,
		jwtSecret: []byte(jwtSecret),
		jwtIssuer: jwtIssuer,
		jwtAud:    jwtAud,
		tokenTTL:  8 * time.Hour,
	}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expiresAt"`
	Staff     domain.Staff `json:"staff"`
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	staff, err := s.staffRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if !staff.IsActive {
		return nil, domain.ErrStaffDeactivated
	}

	if err := password.MatchPassword(staff.PasswordHash, req.Password); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	token, err := middleware.GenerateAdminJWT(staff.ID, staff.Role, s.jwtSecret, s.jwtIssuer, s.jwtAud, s.tokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generating admin JWT: %w", err)
	}

	_ = s.staffRepo.UpdateLastLogin(ctx, staff.ID)

	return &LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(s.tokenTTL),
		Staff:     *staff,
	}, nil
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
}

func (s *AuthService) ChangePassword(ctx context.Context, staffID string, req ChangePasswordRequest) error {
	staff, err := s.staffRepo.GetByID(ctx, staffID)
	if err != nil {
		return err
	}

	if err := password.MatchPassword(staff.PasswordHash, req.CurrentPassword); err != nil {
		return domain.ErrInvalidCredentials
	}

	staff.PasswordHash = password.GeneratePasswordHash(req.NewPassword)
	return s.staffRepo.Update(ctx, staff)
}

func HashPassword(pw string) string {
	return password.GeneratePasswordHash(pw)
}

type CreateStaffRequest struct {
	Email      string           `json:"email" validate:"required,email"`
	FullName   string           `json:"fullName" validate:"required,min=2"`
	Role       domain.StaffRole `json:"role" validate:"required"`
	Department string           `json:"department" validate:"required"`
	Password   string           `json:"password" validate:"required,min=8"`
}

func (s *AuthService) CreateStaff(ctx context.Context, createdBy string, req CreateStaffRequest) (*domain.Staff, error) {
	staff := &domain.Staff{
		ID:           uuid.New().String(),
		Email:        req.Email,
		FullName:     req.FullName,
		Role:         req.Role,
		Department:   req.Department,
		PasswordHash: HashPassword(req.Password),
		IsActive:     true,
		CreatedBy:    &createdBy,
	}

	if err := s.staffRepo.Create(ctx, staff); err != nil {
		return nil, fmt.Errorf("creating staff: %w", err)
	}

	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditStaffCreated,
		ActorType:    "admin",
		ActorID:      &createdBy,
		ResourceType: "staff",
		ResourceID:   staff.ID,
	})

	return staff, nil
}
