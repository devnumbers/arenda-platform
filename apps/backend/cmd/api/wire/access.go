package wire

import (
	"context"

	accessemail "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/email"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Access holds the access module's policy, member-management service and the
// recipient tariff slot coordinator wired by WireAccess.
type Access struct {
	Policy               *accessapp.MembershipPolicy
	AccessService        *accessapp.AccessService
	InvitationService    *accessapp.InvitationService
	SlotCoordinator      *accessapp.SlotCoordinator
	PropertyDeleteMailer *accessapp.PropertyDeleteMailer
	SharedProperties     *accesspg.SharedProperties
	SuspendedCounter     *accesspg.SuspendedCounter
	AccessibleScopes     *accesspg.AccessibleScopes
}

// Compile-time checks that the access SlotCoordinator satisfies the cross-
// context slot-policy port consumed by properties (issue #158, T4), and that
// the PropertyDeleteMailer satisfies the properties delete-mail port (issue
// #162, T6). These assertions live in the wiring layer (the only place allowed
// to depend on several bounded contexts at once) so the access application
// never imports billing or properties. The billing-side recipient-slot
// enforcer port returns with the renewal-worker ticket (#252).
var (
	_ propertiesapp.RecipientSlotPolicy       = (*accessapp.SlotCoordinator)(nil)
	_ propertiesapp.SuspendedSharedCounter    = (*accesspg.SuspendedCounter)(nil)
	_ propertiesapp.SharedMembersDeleteMailer = (*accessapp.PropertyDeleteMailer)(nil)
	// The SharedProperties adapter serves the recipient access context (issue
	// T11); the AccessService resolves owner display names for the banner.
	_ propertiesapp.SharedMemberships        = (*accesspg.SharedProperties)(nil)
	_ propertiesapp.OwnerDisplayNameResolver = (*accessapp.AccessService)(nil)
	// The SharedProperties adapter also serves the leases context (operation
	// list, finance report, lease list, recurring-operations list) and the
	// notifications context (calendar agenda, reminders list, free-reminders
	// list): both consume the ids of properties shared with the actor to fold
	// shared data into aggregate reads (issue #157, T3).
	_ leasesapp.SharedPropertyIDs        = (*accesspg.SharedProperties)(nil)
	_ notificationsapp.SharedPropertyIDs = (*accesspg.SharedProperties)(nil)
	// The OwnerResolver doubles as the archived-status resolver of the access
	// application services (issue #163).
	_ accessapp.PropertyStatusResolver = (*accesspg.OwnerResolver)(nil)
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
// emailMailer is the platform mailer (wired by the identity module) used for
// the invite email of the email invitation lifecycle (issue #161, T5) and the
// sharing lifecycle emails (issue #162, T6).
func WireAccess(_ context.Context, p platformDeps, billing *Billing, emailMailer mailer.Sender) (*Access, error) {
	memberRepo := accesspg.NewMembershipRepository(p.DB)
	invitationRepo := accesspg.NewInvitationRepository(p.DB)
	ownerResolver := accesspg.NewOwnerResolver(p.DB)
	userRepo := identitypg.NewUserRepository(p.DB, p.Encryptor)
	userLookup := accesspg.NewUserLookup(userRepo)
	emailResolver := accesspg.NewUserEmailResolver(userRepo)
	sharedProperties := accesspg.NewSharedProperties(p.DB)
	suspendedCounter := accesspg.NewSuspendedCounter(p.DB)
	accessibleScopes := accesspg.NewAccessibleScopes(p.DB)

	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)

	// Slot coordinator bridges (issue #158, T4). The billing limiter wraps the
	// billing application SubscriptionPropertyLimiter; the occupancy and
	// owned-property ports wrap the properties postgres adapters, rebuilt from
	// the shared pool (they hold no state beyond the db handle).
	propertyLimiter := billing.Services.Limiter
	recipientLimiter := accesspg.NewRecipientLimiterAdapter(propertyLimiter)
	occupancyPort := accesspg.NewOccupancyPortAdapter(propertiespg.NewOccupancyProvider(p.DB))
	ownedActiveProps := accesspg.NewOwnedActivePropertiesAdapter(propertiespg.NewPropertyRepository(p.DB))

	// The access email sender renders through the shared renderer and platform
	// mailer (T5 invite + T6 lifecycle emails); the owner resolver doubles as
	// the property title resolver for the email texts.
	accessMailer := accessemail.NewSender(emailMailer, p.Renderer, p.Cfg.AppBaseURL)
	lifecycleMailer := accessapp.NewLifecycleMailer(accessMailer, emailResolver, ownerResolver, p.Logger)

	slotCoordinator := accessapp.NewSlotCoordinator(
		memberRepo,
		ownerResolver,
		recipientLimiter,
		occupancyPort,
		ownedActiveProps,
		lifecycleMailer,
		p.AuditRecorder,
		p.Beginner,
	)

	accessService := accessapp.NewAccessService(
		memberRepo,
		ownerResolver,
		ownerResolver,
		userLookup,
		policy,
		slotCoordinator,
		lifecycleMailer,
		p.Beginner,
		p.AuditRecorder,
		p.Logger,
	)

	invitationService := accessapp.NewInvitationService(
		accessService,
		memberRepo,
		invitationRepo,
		ownerResolver,
		ownerResolver,
		userLookup,
		policy,
		slotCoordinator,
		accessMailer,
		lifecycleMailer,
		ownerResolver,
		p.Beginner,
		p.AuditRecorder,
		p.Clock,
		p.Logger,
	)

	// The "object deleted" emails to former shared members (issue #162, T6):
	// collected inside the property delete transaction, sent after commit by
	// the properties service.
	propertyDeleteMailer := accessapp.NewPropertyDeleteMailer(memberRepo, emailResolver, accessMailer, p.Logger)

	return &Access{
		Policy:               policy,
		AccessService:        accessService,
		InvitationService:    invitationService,
		SlotCoordinator:      slotCoordinator,
		PropertyDeleteMailer: propertyDeleteMailer,
		SharedProperties:     sharedProperties,
		SuspendedCounter:     suspendedCounter,
		AccessibleScopes:     accessibleScopes,
	}, nil
}
