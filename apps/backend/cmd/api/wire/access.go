package wire

import (
	"context"

	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
)

// Access holds the access module's policy and member-management service wired
// by WireAccess.
type Access struct {
	Policy           *accessapp.MembershipPolicy
	AccessService    *accessapp.AccessService
	SharedProperties *accesspg.SharedProperties
	AccessibleScopes *accesspg.AccessibleScopes
}

// WireAccess constructs the access bounded context (issue #156, T3): the
// membership repository, the property owner resolver, the user lookup adapter
// (over the identity user repository), the membership-aware policy and the
// access service. It returns the policy so the composition root can install it
// on the shared platform deps before property/lease/operation services are
// wired (they all receive the policy).
func WireAccess(_ context.Context, p platformDeps) (*Access, error) {
	memberRepo := accesspg.NewMembershipRepository(p.DB)
	ownerResolver := accesspg.NewOwnerResolver(p.DB)
	userRepo := identitypg.NewUserRepository(p.DB, p.Encryptor)
	userLookup := accesspg.NewUserLookup(userRepo)
	sharedProperties := accesspg.NewSharedProperties(p.DB)
	accessibleScopes := accesspg.NewAccessibleScopes(p.DB)

	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)
	accessService := accessapp.NewAccessService(
		memberRepo,
		ownerResolver,
		userLookup,
		policy,
		p.Beginner,
		p.AuditRecorder,
		p.Logger,
	)

	return &Access{
		Policy:           policy,
		AccessService:    accessService,
		SharedProperties: sharedProperties,
		AccessibleScopes: accessibleScopes,
	}, nil
}
