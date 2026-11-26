package organization_test

import (
	"testing"

	"github.com/semaian/platform/services/api/internal/modules/organization"
)

func TestPermissionMatrix(t *testing.T) {
	if !organization.HasPermission(organization.RoleOwner, organization.PermDelete) {
		t.Fatal("owner must have organization.delete")
	}
	if organization.HasPermission(organization.RoleAdmin, organization.PermDelete) {
		t.Fatal("admin must NOT have organization.delete")
	}
	if !organization.HasPermission(organization.RoleAdmin, organization.PermManageRoles) {
		t.Fatal("admin must have organization.manage_roles")
	}
	if organization.HasPermission(organization.RoleMember, organization.PermManageMembers) {
		t.Fatal("member must NOT have organization.manage_members")
	}
	if !organization.HasPermission(organization.RoleMember, organization.PermRead) {
		t.Fatal("member must have organization.read")
	}
}
