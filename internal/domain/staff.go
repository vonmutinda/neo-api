package domain

import "time"

type StaffRole string

const (
	RoleSuperAdmin         StaffRole = "super_admin"
	RoleCustomerSupport    StaffRole = "customer_support"
	RoleCustomerSupportLead StaffRole = "customer_support_lead"
	RoleComplianceOfficer  StaffRole = "compliance_officer"
	RoleLendingOfficer     StaffRole = "lending_officer"
	RoleReconAnalyst       StaffRole = "reconciliation_analyst"
	RoleCardOperations     StaffRole = "card_operations"
	RoleTreasury           StaffRole = "treasury"
	RoleAuditor            StaffRole = "auditor"
)

type Staff struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	FullName      string     `json:"fullName"`
	Role          StaffRole  `json:"role"`
	Department    string     `json:"department"`
	IsActive      bool       `json:"isActive"`
	PasswordHash  string     `json:"-"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
	CreatedBy     *string    `json:"createdBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeactivatedAt *time.Time `json:"deactivatedAt,omitempty"`
}

type Permission string

const (
	PermUsersRead            Permission = "users:read"
	PermUsersFreezeUnfreeze  Permission = "users:freeze"
	PermUsersKYCOverride     Permission = "users:kyc_override"

	PermTransactionsRead     Permission = "transactions:read"
	PermTransactionsReverse  Permission = "transactions:reverse"
	PermTransactionsExport   Permission = "transactions:export"

	PermLoansRead            Permission = "loans:read"
	PermLoansWriteOff        Permission = "loans:write_off"
	PermLoansCreditOverride  Permission = "loans:credit_override"

	PermCardsRead            Permission = "cards:read"
	PermCardsManage          Permission = "cards:manage"

	PermReconRead            Permission = "recon:read"
	PermReconManage          Permission = "recon:manage"

	PermAuditRead            Permission = "audit:read"

	PermAnalyticsRead        Permission = "analytics:read"

	PermStaffManage          Permission = "staff:manage"

	PermSystemAccounts       Permission = "system:accounts"
	PermFlagsManage          Permission = "flags:manage"

	PermBusinessRead         Permission = "business:read"
	PermBusinessManage       Permission = "business:manage"

	PermConfigManage         Permission = "config:manage"
)

var allPermissions = []Permission{
	PermUsersRead, PermUsersFreezeUnfreeze, PermUsersKYCOverride,
	PermTransactionsRead, PermTransactionsReverse, PermTransactionsExport,
	PermLoansRead, PermLoansWriteOff, PermLoansCreditOverride,
	PermCardsRead, PermCardsManage,
	PermReconRead, PermReconManage,
	PermAuditRead, PermAnalyticsRead,
	PermStaffManage, PermSystemAccounts, PermFlagsManage,
	PermBusinessRead, PermBusinessManage, PermConfigManage,
}

// RolePermissions maps each staff role to its granted permissions.
// Hardcoded in code (not database) for auditability via version control.
var RolePermissions = map[StaffRole][]Permission{
	RoleSuperAdmin: allPermissions,
	RoleCustomerSupport: {
		PermUsersRead, PermTransactionsRead, PermCardsRead, PermLoansRead,
		PermAuditRead, PermBusinessRead,
	},
	RoleCustomerSupportLead: {
		PermUsersRead, PermUsersFreezeUnfreeze, PermTransactionsRead,
		PermTransactionsReverse, PermCardsRead, PermLoansRead, PermAuditRead,
		PermBusinessRead, PermFlagsManage,
	},
	RoleComplianceOfficer: {
		PermUsersRead, PermUsersFreezeUnfreeze, PermTransactionsRead,
		PermTransactionsExport, PermLoansRead, PermCardsRead, PermReconRead,
		PermAuditRead, PermAnalyticsRead, PermFlagsManage, PermBusinessRead,
	},
	RoleLendingOfficer: {
		PermUsersRead, PermLoansRead, PermLoansWriteOff,
		PermLoansCreditOverride, PermAuditRead, PermBusinessRead,
	},
	RoleReconAnalyst: {
		PermTransactionsRead, PermReconRead, PermReconManage, PermAuditRead,
	},
	RoleCardOperations: {
		PermUsersRead, PermCardsRead, PermCardsManage, PermAuditRead,
	},
	RoleTreasury: {
		PermTransactionsRead, PermAnalyticsRead, PermSystemAccounts,
		PermAuditRead, PermReconRead, PermBusinessRead,
	},
	RoleAuditor: {
		PermUsersRead, PermTransactionsRead, PermLoansRead, PermCardsRead,
		PermReconRead, PermAuditRead, PermAnalyticsRead, PermSystemAccounts,
		PermBusinessRead,
	},
}

// HasPermission checks whether a staff role includes the given permission.
func HasPermission(role StaffRole, perm Permission) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
