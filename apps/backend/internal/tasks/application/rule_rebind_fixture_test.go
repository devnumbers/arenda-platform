package application

import (
	"reflect"
	"strings"
	"testing"
)

// TestUpdateRuleCommandCarriesNoPropertyField is the D10 fixture (issue
// #1309): the task rule PATCH has no property field — the rule's object is
// chosen at creation (ADR 0052) and the update route is scoped by it, so no
// cross-property move exists to gate. If this test fails, the command grew a
// property field: before shipping that path, wire the owner-only rebind gate
// (sharedpolicy.CanRebind) — non-owners get the straight 403.
func TestUpdateRuleCommandCarriesNoPropertyField(t *testing.T) {
	t.Parallel()

	cmdType := reflect.TypeFor[UpdateRuleCommand]()
	for field := range cmdType.Fields() {
		if strings.Contains(strings.ToLower(field.Name), "property") {
			t.Fatalf(
				"UpdateRuleCommand carries a property field (%s): a rebind path appeared without its owner-only gate (D10 #1309)",
				field.Name,
			)
		}
	}
}
