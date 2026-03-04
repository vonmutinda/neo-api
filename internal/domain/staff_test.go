package domain

import "testing"

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name string
		role StaffRole
		perm Permission
		want bool
	}{
		{"super_admin has everything", RoleSuperAdmin, PermStaffManage, true},
		{"super_admin has users:read", RoleSuperAdmin, PermUsersRead, true},
		{"customer_support has users:read", RoleCustomerSupport, PermUsersRead, true},
		{"customer_support lacks users:freeze", RoleCustomerSupport, PermUsersFreezeUnfreeze, false},
		{"customer_support_lead has users:freeze", RoleCustomerSupportLead, PermUsersFreezeUnfreeze, true},
		{"auditor has analytics:read", RoleAuditor, PermAnalyticsRead, true},
		{"auditor lacks staff:manage", RoleAuditor, PermStaffManage, false},
		{"lending_officer has loans:write_off", RoleLendingOfficer, PermLoansWriteOff, true},
		{"lending_officer lacks cards:manage", RoleLendingOfficer, PermCardsManage, false},
		{"treasury has system:accounts", RoleTreasury, PermSystemAccounts, true},
		{"treasury lacks users:freeze", RoleTreasury, PermUsersFreezeUnfreeze, false},
		{"recon_analyst has recon:manage", RoleReconAnalyst, PermReconManage, true},
		{"card_operations has cards:manage", RoleCardOperations, PermCardsManage, true},
		{"unknown role has nothing", StaffRole("unknown"), PermUsersRead, false},
		{"customer_support lacks staff:manage", RoleCustomerSupport, PermStaffManage, false},
		{"auditor has audit:read", RoleAuditor, PermAuditRead, true},
		{"super_admin has cards:manage", RoleSuperAdmin, PermCardsManage, true},
		{"super_admin has config:manage", RoleSuperAdmin, PermConfigManage, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := HasPermission(tt.role, tt.perm)
			if got != tt.want {
				t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.perm, got, tt.want)
			}
		})
	}
}

func TestRolePermissions_AllRolesHaveEntries(t *testing.T) {
	roles := []StaffRole{
		RoleSuperAdmin, RoleCustomerSupport, RoleCustomerSupportLead,
		RoleComplianceOfficer, RoleLendingOfficer, RoleReconAnalyst,
		RoleCardOperations, RoleTreasury, RoleAuditor,
	}
	for _, role := range roles {
		perms, ok := RolePermissions[role]
		if !ok {
			t.Errorf("role %q missing from RolePermissions map", role)
		}
		if len(perms) == 0 {
			t.Errorf("role %q has zero permissions", role)
		}
	}
}

func TestRolePermissions_SuperAdminHasAll(t *testing.T) {
	superPerms := RolePermissions[RoleSuperAdmin]
	if len(superPerms) != len(allPermissions) {
		t.Errorf("super_admin has %d permissions, want %d", len(superPerms), len(allPermissions))
	}
}
