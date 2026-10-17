package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
)

// Ролевой тарифный гейт access-роутов объекта (ADR 0064, ревизия #1462):
// проверка живёт в use-case'ах после уже существующего резолва роли (ADR
// 0028) — без второго резолва. Чтения участников проходят у участника объекта
// на любом тарифе («пригласили — значит пользуется»), владелец на базовом
// тарифе получает отказ и в чтениях, и в мутациях управления участниками;
// самовыход остаётся вне гейта.

// stubGate is the PaidSectionsGate test double: a fixed verdict plus the call
// counter — «гейт спрашивают только у владельца» pins itself through calls.
type stubGate struct {
	allows bool
	err    error
	calls  int
}

func (g *stubGate) CanUsePaidFeatures(context.Context, uuid.UUID) (bool, error) {
	g.calls++
	return g.allows, g.err
}

var _ PaidSectionsGate = (*stubGate)(nil)

// gateFixture is the minimal stand for the membership use cases: an owner,
// one seeded viewer member, the in-memory ports and the gate under test.
type gateFixture struct {
	repo       *memRepo
	gate       *stubGate
	property   uuid.UUID
	owner      uuid.UUID
	member     uuid.UUID
	membership domain.Membership
	svc        *AccessService
}

func newGateFixture(ctx context.Context, allows bool) *gateFixture {
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	repo.SetOwner(property, owner)
	gate := &stubGate{allows: allows}
	policy := NewMembershipPolicy(staticResolver{property: owner}, repo)
	svc := NewAccessService(repo, staticResolver{property: owner}, fakeStatuses{}, stubLookup{}, policy, nil, nil,
		gate, newTestFactory(repo, &memInvitationsRepo{}, auditapp.Noop{}), nil)

	membership, err := repo.Create(ctx, domain.Membership{
		ID:         uuid.Must(uuid.NewV7()),
		PropertyID: property,
		UserID:     member,
		Role:       domain.RoleViewer,
		GrantedBy:  owner,
	})
	if err != nil {
		panic("seed membership: " + err.Error())
	}
	return &gateFixture{
		repo:       repo,
		gate:       gate,
		property:   property,
		owner:      owner,
		member:     member,
		membership: membership,
		svc:        svc,
	}
}

// TestTariffGate_OwnerReadRefusedOnBasicTariff pins the read direction of the
// gate: the members list is a paid section for the owner — 402 by contract.
func TestTariffGate_OwnerReadRefusedOnBasicTariff(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), false)

	_, err := f.svc.ListMembers(t.Context(), f.owner, f.property)
	if !errors.Is(err, ErrTariffRequired) {
		t.Fatalf("ListMembers by owner on basic: expected ErrTariffRequired, got %v", err)
	}
}

// TestTariffGate_MembersReadOnBasicTariff pins «пригласили — значит
// пользуется»: a property member reads the list on the basic tariff, and the
// gate is not consulted for them at all — the verdict belongs to the role.
func TestTariffGate_MembersReadOnBasicTariff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		role domain.Role
	}{
		{name: "full access", role: domain.RoleFullAccess},
		{name: "viewer", role: domain.RoleViewer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newGateFixture(t.Context(), false)
			f.membership.Role = tc.role
			if _, err := f.repo.UpdateRole(t.Context(), f.membership.ID, f.property, tc.role); err != nil {
				t.Fatalf("seed role: %v", err)
			}

			members, err := f.svc.ListMembers(t.Context(), f.member, f.property)
			if err != nil {
				t.Fatalf("ListMembers by member on basic: %v", err)
			}
			if len(members) < 2 {
				t.Errorf("members = %d, want the owner and the reader rows", len(members))
			}
			if f.gate.calls != 0 {
				t.Errorf("gate calls = %d, want 0 — the gate is the owner's refusal, not the member's", f.gate.calls)
			}
		})
	}
}

// TestTariffGate_SuspendedAndStrangerReadsStaysHidden pins the unchanged
// privacy policy: a suspended member and a stranger get the
// privacy-preserving not-found on any tariff, the gate unconsulted.
func TestTariffGate_SuspendedAndStrangerReadsStaysHidden(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), false)
	if err := f.repo.Suspend(t.Context(), f.membership.ID, f.property); err != nil {
		t.Fatalf("seed suspend: %v", err)
	}

	if _, err := f.svc.ListMembers(t.Context(), f.member, f.property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("suspended member read: expected ErrMemberNotFound, got %v", err)
	}
	if _, err := f.svc.ListMembers(t.Context(), uuid.Must(uuid.NewV7()), f.property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("stranger read: expected ErrMemberNotFound, got %v", err)
	}
	if f.gate.calls != 0 {
		t.Errorf("gate calls = %d, want 0 — suspension and privacy are role-policy answers", f.gate.calls)
	}
}

// TestTariffGate_OwnerReadOnPaidTariff passes — the gate refuses only the
// non-paid tariff, the owner's own section on a paid one works as before.
func TestTariffGate_OwnerReadOnPaidTariff(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), true)

	members, err := f.svc.ListMembers(t.Context(), f.owner, f.property)
	if err != nil {
		t.Fatalf("ListMembers by owner on paid: %v", err)
	}
	if len(members) < 2 {
		t.Errorf("members = %d, want the owner and the member rows", len(members))
	}
}

// TestTariffGate_ReadGateFailurePropagates keeps the gate fail-closed on a
// storage error: the failure is not the tariff verdict, it propagates for the
// HTTP layer's 500 (ADR 0064: the gate never fails open).
func TestTariffGate_ReadGateFailurePropagates(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), false)
	f.gate.err = errors.New("subscriptions store down")

	if _, err := f.svc.ListMembers(t.Context(), f.owner, f.property); err == nil || errors.Is(err, ErrTariffRequired) {
		t.Fatalf("ListMembers with a broken gate: expected a propagated error, got %v", err)
	}
}

// TestTariffGate_MutationsRefusedOnBasicTariff pins the mutation direction:
// the owner's member-management stays a paid section — every manage use case
// refuses with ErrTariffRequired on the basic tariff.
func TestTariffGate_MutationsRefusedOnBasicTariff(t *testing.T) {
	t.Parallel()
	newMember := uuid.Must(uuid.NewV7())

	t.Run("add member", func(t *testing.T) {
		t.Parallel()
		f := newGateFixture(t.Context(), false)

		if _, err := f.svc.AddMember(t.Context(), f.owner, f.property, newMember, domain.RoleViewer); !errors.Is(err, ErrTariffRequired) {
			t.Fatalf("AddMember by owner on basic: expected ErrTariffRequired, got %v", err)
		}
		if len(f.repo.rows) != 1 {
			t.Errorf("rows = %d, want only the seeded membership", len(f.repo.rows))
		}
	})
	t.Run("change role", func(t *testing.T) {
		t.Parallel()
		f := newGateFixture(t.Context(), false)

		if _, err := f.svc.ChangeMemberRole(t.Context(), f.owner, f.property,
			f.membership.ID, domain.RoleFullAccess); !errors.Is(err, ErrTariffRequired) {
			t.Fatalf("ChangeMemberRole by owner on basic: expected ErrTariffRequired, got %v", err)
		}
	})
	t.Run("revoke", func(t *testing.T) {
		t.Parallel()
		f := newGateFixture(t.Context(), false)

		if err := f.svc.RevokeMember(t.Context(), f.owner, f.property, f.membership.ID); !errors.Is(err, ErrTariffRequired) {
			t.Fatalf("RevokeMember by owner on basic: expected ErrTariffRequired, got %v", err)
		}
		if _, err := f.repo.GetByID(t.Context(), f.membership.ID, f.property); err != nil {
			t.Errorf("membership must survive the refused revoke, got %v", err)
		}
	})
}

// TestTariffGate_MemberMutationsRefusedByRolePolicy pins the unchanged role
// policy (owner-only since D8, #1304): a member's manage attempt gets the
// privacy-preserving not-found, the gate unconsulted — the tariff verdict is
// not the member's to hear.
func TestTariffGate_MemberMutationsRefusedByRolePolicy(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), false)
	newMember := uuid.Must(uuid.NewV7())

	if _, err := f.svc.AddMember(t.Context(), f.member, f.property, newMember, domain.RoleViewer); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("AddMember by member: expected ErrMemberNotFound, got %v", err)
	}
	if _, err := f.svc.ChangeMemberRole(t.Context(), f.member, f.property,
		f.membership.ID, domain.RoleFullAccess); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("ChangeMemberRole by member: expected ErrMemberNotFound, got %v", err)
	}
	if err := f.svc.RevokeMember(t.Context(), f.member, f.property, f.membership.ID); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("RevokeMember by member: expected ErrMemberNotFound, got %v", err)
	}
	if f.gate.calls != 0 {
		t.Errorf("gate calls = %d, want 0 — the role policy answers before the tariff", f.gate.calls)
	}
}

// TestTariffGate_MutationsOnPaidTariff is the regression direction: a paid
// owner manages members exactly as before.
func TestTariffGate_MutationsOnPaidTariff(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), true)
	newMember := uuid.Must(uuid.NewV7())

	created, err := f.svc.AddMember(t.Context(), f.owner, f.property, newMember, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("AddMember by owner on paid: %v", err)
	}
	if _, err := f.svc.ChangeMemberRole(t.Context(), f.owner, f.property, created.ID, domain.RoleViewer); err != nil {
		t.Fatalf("ChangeMemberRole by owner on paid: %v", err)
	}
	if err := f.svc.RevokeMember(t.Context(), f.owner, f.property, created.ID); err != nil {
		t.Fatalf("RevokeMember by owner on paid: %v", err)
	}
}

// TestTariffGate_MutationGateFailurePropagates keeps the mutation direction
// fail-closed on a storage error too.
func TestTariffGate_MutationGateFailurePropagates(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), false)
	f.gate.err = errors.New("subscriptions store down")

	if _, err := f.svc.AddMember(t.Context(), f.owner, f.property, uuid.Must(uuid.NewV7()),
		domain.RoleViewer); err == nil || errors.Is(err, ErrTariffRequired) {
		t.Fatalf("AddMember with a broken gate: expected a propagated error, got %v", err)
	}
}

// TestTariffGate_SelfExitOutsideTheGate pins «Покинуть объект» living outside
// the tariff gate: a member leaves on the basic tariff, and the owner's
// cannot-leave refusal stays the role-policy answer, not a tariff one.
func TestTariffGate_SelfExitOutsideTheGate(t *testing.T) {
	t.Parallel()
	f := newGateFixture(t.Context(), false)

	if err := f.svc.LeaveProperty(t.Context(), f.member, f.property); err != nil {
		t.Errorf("LeaveProperty by member on basic: %v", err)
	}
	if err := f.svc.LeaveProperty(t.Context(), f.owner, f.property); !errors.Is(err, domain.ErrCannotLeaveOwnProperty) {
		t.Errorf("owner leave: expected ErrCannotLeaveOwnProperty, got %v", err)
	}
	if f.gate.calls != 0 {
		t.Errorf("gate calls = %d, want 0 — self-exit lives outside the gate", f.gate.calls)
	}
}

// TestTariffGate_InvitationLifecycleRefusedOnBasicTariff covers the email
// invitation lifecycle and the member-card read through the invitation
// service: the owner on the basic tariff is refused everywhere, the gate
// consulted once per call.
func TestTariffGate_InvitationLifecycleRefusedOnBasicTariff(t *testing.T) {
	t.Parallel()
	gate := &stubGate{allows: false}
	f := newInvitationFixtureWithGate(gate)
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	invitee := uuid.Must(uuid.NewV7())
	f.lookup.add("friend@example.com", invitee)
	f.limiter.set(invitee, 10)

	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "friend@example.com",
		domain.RoleViewer); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("InviteByEmail by owner on basic: expected ErrTariffRequired, got %v", err)
	}

	invitation := domain.Invitation{
		ID:         uuid.Must(uuid.NewV7()),
		PropertyID: property,
		Email:      "pending@example.com",
		Role:       domain.RoleViewer,
		InvitedBy:  owner,
		LastSentAt: f.clk.Now().Add(-25 * time.Hour),
	}
	if _, err := f.invitations.Create(t.Context(), invitation); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	if _, err := f.svc.ChangeInvitationRole(t.Context(), owner, property, invitation.ID,
		domain.RoleFullAccess); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("ChangeInvitationRole by owner on basic: expected ErrTariffRequired, got %v", err)
	}
	if err := f.svc.CancelInvitation(t.Context(), owner, property, invitation.ID); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("CancelInvitation by owner on basic: expected ErrTariffRequired, got %v", err)
	}
	if err := f.svc.ResendInvitation(t.Context(), owner, property, invitation.ID); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("ResendInvitation by owner on basic: expected ErrTariffRequired, got %v", err)
	}
	if _, err := f.svc.GetMember(t.Context(), owner, property, invitation.ID); !errors.Is(err, ErrTariffRequired) {
		t.Errorf("GetMember by owner on basic: expected ErrTariffRequired, got %v", err)
	}
	if len(f.invitations.rows) != 1 {
		t.Errorf("invitations = %d, want only the seeded row — refusals write nothing", len(f.invitations.rows))
	}
}

// TestTariffGate_InvitationMemberRefusedByRolePolicy pins the D8 answer on
// the invitation lifecycle: a member's invite attempt is the
// privacy-preserving not-found, the gate unconsulted.
func TestTariffGate_InvitationMemberRefusedByRolePolicy(t *testing.T) {
	t.Parallel()
	gate := &stubGate{allows: false}
	f := newInvitationFixtureWithGate(gate)
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	member := uuid.Must(uuid.NewV7())
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID:         uuid.Must(uuid.NewV7()),
		PropertyID: property,
		UserID:     member,
		Role:       domain.RoleFullAccess,
		GrantedBy:  owner,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	if _, err := f.svc.InviteByEmail(t.Context(), member, property, "friend@example.com",
		domain.RoleViewer); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("InviteByEmail by member: expected ErrMemberNotFound, got %v", err)
	}
	if gate.calls != 0 {
		t.Errorf("gate calls = %d, want 0 — the role policy answers before the tariff", gate.calls)
	}
}

// TestTariffGate_InvitationOnPaidTariff is the regression direction: a paid
// owner invites by email exactly as before.
func TestTariffGate_InvitationOnPaidTariff(t *testing.T) {
	t.Parallel()
	gate := &stubGate{allows: true}
	f := newInvitationFixtureWithGate(gate)
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	invitee := uuid.Must(uuid.NewV7())
	f.lookup.add("friend@example.com", invitee)
	f.limiter.set(invitee, 10)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "friend@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail by owner on paid: %v", err)
	}
	if outcome.Member == nil {
		t.Errorf("expected the instant membership outcome, got %+v", outcome)
	}
}
