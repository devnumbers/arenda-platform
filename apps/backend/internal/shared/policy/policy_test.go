package policy

import "testing"

// TestCapabilityFunctions verifies the capability predicates across all five
// roles (issue #156, T3; suspended added in T9): owner, full_access, viewer,
// none, suspended.
func TestCapabilityFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role Role
		view bool
		edit bool
		mng  bool
		life bool
	}{
		{"owner", RoleOwner, true, true, true, true},
		{"full_access", RoleFullAccess, true, true, true, false},
		{"viewer", RoleViewer, true, false, false, false},
		{"none", RoleNone, false, false, false, false},
		// A suspended membership (tariff limit exceeded, issue #158 T4)
		// grants no capabilities; it is only distinguishable from none so the
		// property page entry point can signal the dedicated screen (T9).
		{"suspended", RoleSuspended, false, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CanView(tt.role); got != tt.view {
				t.Errorf("CanView(%s) = %v, want %v", tt.role, got, tt.view)
			}
			if got := CanEdit(tt.role); got != tt.edit {
				t.Errorf("CanEdit(%s) = %v, want %v", tt.role, got, tt.edit)
			}
			if got := CanManageMembers(tt.role); got != tt.mng {
				t.Errorf("CanManageMembers(%s) = %v, want %v", tt.role, got, tt.mng)
			}
			if got := CanLifecycle(tt.role); got != tt.life {
				t.Errorf("CanLifecycle(%s) = %v, want %v", tt.role, got, tt.life)
			}
		})
	}
}
