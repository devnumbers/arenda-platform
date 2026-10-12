package policy

import "testing"

// TestCapabilityFunctions verifies the capability predicates across all five
// roles (issue #156, T3; suspended added in T9): owner, full_access, viewer,
// none, suspended. Member management is owner-only since the D8 decision of
// the sharing effort (issue #1304): full_access keeps every in-object
// capability but loses invite/change-role/revoke. Since D9 (issue #1309)
// full_access also keeps the deletions (payments, rentals) — they ride the
// edit capability; the cross-object rebind stays owner-only (D10).
func TestCapabilityFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		role   Role
		view   bool
		edit   bool
		mng    bool
		life   bool
		rebind bool
	}{
		{"owner", RoleOwner, true, true, true, true, true},
		{"full_access", RoleFullAccess, true, true, false, false, false},
		{"viewer", RoleViewer, true, false, false, false, false},
		{"none", RoleNone, false, false, false, false, false},
		// A suspended membership (tariff limit exceeded, issue #158 T4)
		// grants no capabilities; it is only distinguishable from none so the
		// property page entry point can signal the dedicated screen (T9).
		{"suspended", RoleSuspended, false, false, false, false, false},
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
			if got := CanRebind(tt.role); got != tt.rebind {
				t.Errorf("CanRebind(%s) = %v, want %v", tt.role, got, tt.rebind)
			}
		})
	}
}
