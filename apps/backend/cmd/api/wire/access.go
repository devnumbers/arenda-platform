package wire

import (
	"context"

	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Access holds the access module's policy, member-management service and the
// recipient tariff slot coordinator wired by WireAccess.
type Access struct {
	Policy           *accessapp.MembershipPolicy
	AccessService    *accessapp.AccessService
	SlotCoordinator  *accessapp.SlotCoordinator
	SharedProperties *accesspg.SharedProperties
	SuspendedCounter *accesspg.SuspendedCounter
	AccessibleScopes *accesspg.AccessibleScopes
}

// Compile-time checks that the access SlotCoordinator satisfies the cross-
// context slot-policy ports consumed by billing and properties (issue #158,
// T4). These assertions live in the wiring layer (the only place allowed to
// depend on several bounded contexts at once) so the access application never
// imports billing or properties.
var (
	_ billingapp.RecipientSlotEnforcer     = (*accessapp.SlotCoordinator)(nil)
	_ propertiesapp.RecipientSlotPolicy    = (*accessapp.SlotCoordinator)(nil)
	_ propertiesapp.SuspendedSharedCounter = (*accesspg.SuspendedCounter)(nil)
)

// WireAccess constructs the access bounded context (issue #156, T3): the
// membership repository, the property owner resolver, the user lookup adapter
// (over the identity user repository), the membership-aware policy, the access
// service, and the recipient tariff slot coordinator (issue #158, T4). It
// returns the policy so the composition root can install it on the shared
// platform deps before property/lease/operation services are wired (they all
// receive the policy). The slot coordinator needs the billing property limiter
// (built from the billing repos) plus the properties occupancy/owned-property
// providers, so billingRepos is required; the properties adapters are rebuilt
// here from the shared db pool (they are stateless pointers over the pool).
func WireAccess(_ context.Context, p platformDeps, billingRepos *BillingRepos) (*Access, error) {
	memberRepo := accesspg.NewMembershipRepository(p.DB)
	ownerResolver := accesspg.NewOwnerResolver(p.DB)
	userRepo := identitypg.NewUserRepository(p.DB, p.Encryptor)
	userLookup := accesspg.NewUserLookup(userRepo)
	sharedProperties := accesspg.NewSharedProperties(p.DB)
	suspendedCounter := accesspg.NewSuspendedCounter(p.DB)
	accessibleScopes := accesspg.NewAccessibleScopes(p.DB)

	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)

	// Slot coordinator bridges (issue #158, T4). The billing limiter wraps the
	// billing application SubscriptionPropertyLimiter; the occupancy and
	// owned-property ports wrap the properties postgres adapters, rebuilt from
	// the shared pool (they hold no state beyond the db handle).
	propertyLimiter := billingapp.NewSubscriptionPropertyLimiter(billingRepos.SubscriptionRepo, billingRepos.TariffRepo)
	recipientLimiter := accesspg.NewRecipientLimiterAdapter(propertyLimiter)
	occupancyPort := accesspg.NewOccupancyPortAdapter(propertiespg.NewOccupancyProvider(p.DB))
	ownedActiveProps := accesspg.NewOwnedActivePropertiesAdapter(propertiespg.NewPropertyRepository(p.DB))

	slotCoordinator := accessapp.NewSlotCoordinator(
		memberRepo,
		ownerResolver,
		recipientLimiter,
		occupancyPort,
		ownedActiveProps,
		p.AuditRecorder,
		p.Beginner,
	)

	accessService := accessapp.NewAccessService(
		memberRepo,
		ownerResolver,
		userLookup,
		policy,
		slotCoordinator,
		p.Beginner,
		p.AuditRecorder,
		p.Logger,
	)

	return &Access{
		Policy:           policy,
		AccessService:    accessService,
		SlotCoordinator:  slotCoordinator,
		SharedProperties: sharedProperties,
		SuspendedCounter: suspendedCounter,
		AccessibleScopes: accessibleScopes,
	}, nil
}
