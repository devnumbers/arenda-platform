package policy

// GateDecision is the outcome of the authorization gate for one capability:
// what a resolved Role allows. Callers map the decision onto their own error
// vocabulary — none and suspended belong to the privacy-preserving not-found
// outcome, a role without the capability to a straight forbidden — so the
// sentinel errors stay in each context while the gate skeleton lives here.
type GateDecision string

const (
	// GateAllow — the role passes the capability check.
	GateAllow GateDecision = "allow"
	// GateNone — the actor has no access to the scope's data at all.
	GateNone GateDecision = "none"
	// GateSuspended — the actor's membership is suspended by a tariff slot
	// shortage; like none it grants nothing, but it stays distinguishable.
	GateSuspended GateDecision = "suspended"
	// GateForbidden — the role is valid but lacks this capability.
	GateForbidden GateDecision = "forbidden"
)

// GateFor maps a resolved role onto the gate decision for a capability
// predicate (CanView, CanEdit, CanLifecycle): none and suspended never pass
// (object privacy), a role without the capability is forbidden, otherwise
// allow. The ADR 0028 matrix in one place.
func GateFor(r Role, can func(Role) bool) GateDecision {
	switch r {
	case RoleNone:
		return GateNone
	case RoleSuspended:
		return GateSuspended
	case RoleOwner, RoleFullAccess, RoleViewer:
		if !can(r) {
			return GateForbidden
		}
		return GateAllow
	default:
		// An unknown future role grants nothing; forbidden, not none, so a
		// stale caller surfaces the capability mismatch instead of a missing
		// object.
		return GateForbidden
	}
}
