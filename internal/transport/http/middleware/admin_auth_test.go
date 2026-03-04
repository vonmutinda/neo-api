package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
)

func TestGenerateAndValidateAdminJWT(t *testing.T) {
	secret := []byte("test-admin-secret-key-32-bytes!!")
	issuer := "neobank-admin"
	audience := "neobank-admin-api"

	token, err := GenerateAdminJWT("staff-123", domain.RoleSuperAdmin, secret, issuer, audience, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAdminJWT: %v", err)
	}

	staffID, role, err := validateAdminJWT(token, JWTConfig{
		Secret:   secret,
		Issuer:   issuer,
		Audience: audience,
	})
	if err != nil {
		t.Fatalf("validateAdminJWT: %v", err)
	}

	if staffID != "staff-123" {
		t.Errorf("staffID = %q, want %q", staffID, "staff-123")
	}
	if role != domain.RoleSuperAdmin {
		t.Errorf("role = %q, want %q", role, domain.RoleSuperAdmin)
	}
}

func TestAdminAuth_ValidJWT(t *testing.T) {
	secret := []byte("test-admin-secret-key-32-bytes!!")
	cfg := JWTConfig{Secret: secret}

	token, err := GenerateAdminJWT("staff-abc", domain.RoleAuditor, secret, "", "", time.Hour)
	if err != nil {
		t.Fatalf("GenerateAdminJWT: %v", err)
	}

	handler := AdminAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staffID := StaffIDFromContext(r.Context())
		role := StaffRoleFromContext(r.Context())
		if staffID != "staff-abc" {
			t.Errorf("staffID = %q, want %q", staffID, "staff-abc")
		}
		if role != domain.RoleAuditor {
			t.Errorf("role = %q, want %q", role, domain.RoleAuditor)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminAuth_MissingHeader(t *testing.T) {
	cfg := JWTConfig{Secret: []byte("test-secret")}
	handler := AdminAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/admin/v1/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequirePermission_Allowed(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequirePermission(domain.PermUsersRead)(inner)

	req := httptest.NewRequest("GET", "/", nil)
	ctx := req.Context()
	ctx = setAdminContext(ctx, "staff-1", domain.RoleSuperAdmin)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := RequirePermission(domain.PermStaffManage)(inner)

	req := httptest.NewRequest("GET", "/", nil)
	ctx := req.Context()
	ctx = setAdminContext(ctx, "staff-1", domain.RoleAuditor)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func setAdminContext(ctx context.Context, staffID string, role domain.StaffRole) context.Context {
	ctx = context.WithValue(ctx, adminStaffIDKey{}, staffID)
	ctx = context.WithValue(ctx, adminStaffRoleKey{}, role)
	return ctx
}
