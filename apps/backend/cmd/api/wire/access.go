package wire

import (
	"context"

	accessemail "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/email"
	accessevents "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/events"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Access holds the access module's policy, member-management service and the
// recipient tariff slot coordinator wired by WireAccess.
type Access struct {
	Policy                 *accessapp.MembershipPolicy
	AccessService          *accessapp.AccessService
	InvitationService      *accessapp.InvitationService
	ParticipantService     *accessapp.ParticipantService
	ParticipantMutationSvc *accessapp.ParticipantMutationService
	SlotCoordinator        *accessapp.SlotCoordinator
	SharedProperties       *accesspg.SharedProperties
	SharedListEnricher     *accesspg.SharedListEnricher
	UserEmailResolver      *accesspg.UserEmailResolverAdapter
	PropertyDeleteMailer   *accessapp.PropertyDeleteMailer
}

// Compile-time checks that the access SlotCoordinator satisfies the cross-
// context slot-policy port consumed by properties (issue #158, T4). These
// assertions live in the wiring layer (the only place allowed to depend on
// several bounded contexts at once) so the access application never imports
// billing or properties.
var (
	_ propertiesapp.RecipientSlotPolicy = (*accessapp.SlotCoordinator)(nil)
	// The SharedProperties adapter serves the recipient access context (issue
	// T11); the AccessService resolves owner display names for the banner.
	_ propertiesapp.SharedMemberships        = (*accesspg.SharedProperties)(nil)
	_ propertiesapp.OwnerDisplayNameResolver = (*accessapp.AccessService)(nil)
	// The UserEmailResolverAdapter doubles as the owner-email resolver of the
	// detail's owner contact row (Figma 2200-97365, #757 walkthrough fixes).
	_ propertiesapp.OwnerEmailResolver = (*accesspg.UserEmailResolverAdapter)(nil)
	// The PropertyDeleteMailer satisfies the properties delete-mail port
	// (issue #162, T6): the "object deleted" letter stays direct — its event
	// is not in the notifications catalog.
	_ propertiesapp.SharedMembersDeleteMailer = (*accessapp.PropertyDeleteMailer)(nil)
	// The SharedListEnricher serves the list reads' access projections
	// (ticket #702): member names on the owner's cards and the suspended
	// blur-card placeholders.
	_ propertiesapp.SuspendedSharedMemberships = (*accesspg.SharedListEnricher)(nil)
	// The OwnerResolver doubles as the archived-status resolver of the access
	// application services (issue #163).
	_ accessapp.PropertyStatusResolver = (*accesspg.OwnerResolver)(nil)
)

// WireAccess constructs the access bounded context (issue #156, T3): the
// membership repository, the property owner resolver, the user lookup adapter
// (over the identity user repository), the membership-aware policy, the access
// service, and the recipient tariff slot coordinator (issue #158, T4). It
// returns the policy so the composition root can install it on the shared
// platform deps before property services are wired (they all
// receive the policy). The slot coordinator needs the billing property limiter
// (built from the billing repos) plus the properties occupancy/owned-property
// providers, so billingRepos is required; the properties adapters are rebuilt
// here from the shared db pool (they are stateless pointers over the pool).
// The emailMailer parameter is the platform mailer (wired by the identity
// module) used for the invite email of the email invitation lifecycle
// (issue #161, T5) and the "object deleted" notice (issue #162, T6). The
// dispatcher carries the lifecycle events (#751) to the notifications
// context's subscriber.
func WireAccess(
	_ context.Context,
	p platformDeps,
	billing *Billing,
	emailMailer mailer.Sender,
	eventDispatcher *events.InProcessDispatcher,
) (*Access, error) {
	memberRepo := accesspg.NewMembershipRepository(p.DB)
	invitationRepo := accesspg.NewInvitationRepository(p.DB)
	participantRepo := accesspg.NewParticipantRepository(p.DB)
	ownerResolver := accesspg.NewOwnerResolver(p.DB)
	userRepo := identitypg.NewUserRepository(p.DB, p.Encryptor)
	userLookup := accesspg.NewUserLookup(userRepo)
	emailResolver := accesspg.NewUserEmailResolver(userRepo)
	sharedProperties := accesspg.NewSharedProperties(p.DB)
	sharedListEnricher := accesspg.NewSharedListEnricher(p.DB, userRepo)

	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)

	// The single canonical txStoreFactory bundles the access
	// repositories, the audit recorder, and the UoW (ADR 0033 γ-factory). It is
	// passed to every access service so adding an Nth repository is a change
	// here, not in several constructors.
	factory := accessapp.NewTxStoreFactory(memberRepo, invitationRepo, p.AuditRecorder, p.UoW)

	// Slot coordinator bridges (issue #158, T4). The billing limiter wraps the
	// billing application SubscriptionPropertyLimiter; the owned-property port
	// wraps the properties postgres adapter, rebuilt from the shared pool (it
	// holds no state beyond the db handle).
	propertyLimiter := billing.Services.Limiter
	recipientLimiter := accesspg.NewRecipientLimiterAdapter(propertyLimiter)
	ownedActiveProps := accesspg.NewOwnedActivePropertiesAdapter(propertiespg.NewPropertyRepository(p.DB))

	// The access email sender renders through the shared renderer and platform
	// mailer (T5 invite + the T6 object-deleted notice — the two direct emails
	// that stay); the owner resolver doubles as the property title resolver
	// for the email texts.
	accessMailer := accessemail.NewSender(emailMailer, p.Renderer, p.Cfg.AppBaseURL)

	// The lifecycle events publisher (карта #734, #751): the transitions'
	// notifications leave the context through the shared dispatcher, the
	// composition root subscribes the notifications publisher to them.
	accessEventPublisher := accessevents.NewPublisher(eventDispatcher)

	slotCoordinator := accessapp.NewSlotCoordinator(
		memberRepo,
		ownerResolver,
		recipientLimiter,
		ownedActiveProps,
		accessEventPublisher,
		p.AuditRecorder,
		p.Beginner,
		p.Clock,
		p.Logger,
	)

	accessService := accessapp.NewAccessService(
		memberRepo,
		ownerResolver,
		ownerResolver,
		userLookup,
		policy,
		slotCoordinator,
		accessEventPublisher,
		factory,
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
		accessEventPublisher,
		ownerResolver,
		factory,
		p.Clock,
		p.Logger,
		emailResolver,
	)

	// The owner's participant read model (issue #693): the aggregate over
	// memberships ∪ invitations scoped to the reading actor's manage scope.
	participantService := accessapp.NewParticipantService(
		participantRepo,
		userLookup,
		emailResolver,
		memberRepo,
		p.Logger,
	)

	// The mutation side of the participant aggregate (issue #694): the
	// multi-object invite, «Пригласить в объект» и «Отозвать и удалить» —
	// over the same per-property gates and the slot coordinator.
	participantMutations := accessapp.NewParticipantMutationService(
		accessService,
		ownerResolver,
		ownerResolver,
		userLookup,
		emailResolver,
		policy,
		slotCoordinator,
		accessMailer,
		accessEventPublisher,
		ownerResolver,
		factory,
		p.Clock,
		p.Logger,
	)

	// The "object deleted" emails to former shared members (issue #162, T6):
	// collected inside the property delete transaction, sent after commit by
	// the properties service.
	propertyDeleteMailer := accessapp.NewPropertyDeleteMailer(memberRepo, emailResolver, accessMailer, p.Logger)

	return &Access{
		Policy:                 policy,
		AccessService:          accessService,
		InvitationService:      invitationService,
		ParticipantService:     participantService,
		ParticipantMutationSvc: participantMutations,
		SlotCoordinator:        slotCoordinator,
		SharedProperties:       sharedProperties,
		SharedListEnricher:     sharedListEnricher,
		UserEmailResolver:      emailResolver,
		PropertyDeleteMailer:   propertyDeleteMailer,
	}, nil
}
