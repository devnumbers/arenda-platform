package application

// The action journal of the access use cases (карта #704, тикет #707,
// ADR 0061): every manual membership action journals with the person's
// display name (or the invitee email) as the label snapshot and the actor's
// real role attribution — the audit pattern.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeHistory struct {
	entries []historydomain.Entry
	err     error
}

func (h *fakeHistory) Record(_ context.Context, e historydomain.Entry) error {
	if h.err != nil {
		return h.err
	}
	h.entries = append(h.entries, e)
	return nil
}

func (h *fakeHistory) WithTx(transaction.Tx) historyapp.Recorder { return h }

func newTestFactoryWithHistory(
	members MembershipRepository, invitations InvitationRepository, history historyapp.Recorder,
) txStoreFactory {
	return NewTxStoreFactory(members, invitations, auditapp.Noop{}, history, fakeUoW{beginner: noopBeginner{}})
}

// historyFixture wires the membership services over the in-memory repos with
// a capturing history recorder, the access-test canon.
type historyFixture struct {
	t        *testing.T
	owner    uuid.UUID
	member   uuid.UUID
	stranger uuid.UUID // Stand-in id of an unregistered person (never resolved).
	invited  uuid.UUID

	property uuid.UUID
	repo     *memRepo
	history  *fakeHistory
	svc      *AccessService
}

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())
	invited := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	lookup := fakeUserLookup{
		owner:    {ID: owner, Name: new("Иван"), Surname: new("Иванов"), Phone: "+79120000001"},
		member:   {ID: member, Name: new("Пётр"), Surname: new("Сидоров"), Phone: "+79120000002"},
		stranger: {ID: stranger, Phone: "+79120000003"},
	}
	history := &fakeHistory{}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, nil, lookup, policy, nil, nil,
		newTestFactoryWithHistory(repo, &memInvitationsRepo{}, history), nil)
	return &historyFixture{
		t: t, owner: owner, member: member, stranger: stranger, invited: invited,
		property: property, repo: repo, history: history, svc: svc,
	}
}

func TestHistory_MembershipLifecycleRows(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	ctx := context.Background()

	if _, err := h.svc.AddMember(ctx, h.owner, h.property, h.member, domain.RoleFullAccess); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	added := h.history.entries[len(h.history.entries)-1]
	if added.Action != historydomain.ActionMemberAdded {
		t.Fatalf("action = %s, want member.added", added.Action)
	}
	if added.Segments.PlainText() != "Добавлен участник: Пётр Сидоров" {
		t.Errorf("row text = %q, want the display-name snapshot", added.Segments.PlainText())
	}
	if added.ActorRole != historydomain.ActorRoleOwner {
		t.Errorf("actor role = %s, want owner", added.ActorRole)
	}

	if _, err := h.svc.ChangeMemberRole(ctx, h.owner, h.property, h.repo.rows[0].ID, domain.RoleViewer); err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
	changed := h.history.entries[len(h.history.entries)-1]
	if changed.Action != historydomain.ActionMemberRoleChanged {
		t.Fatalf("action = %s, want member.role_changed", changed.Action)
	}
	if text := changed.Segments.PlainText(); text != "Роль участника изменена: Пётр Сидоров: Полный доступ → Просмотр" {
		t.Errorf("row text = %q, want the old → new role snapshot", text)
	}

	if err := h.svc.RevokeMember(ctx, h.owner, h.property, h.repo.rows[0].ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}
	revoked := h.history.entries[len(h.history.entries)-1]
	if revoked.Action != historydomain.ActionMemberRemoved {
		t.Fatalf("action = %s, want member.removed", revoked.Action)
	}
	if revoked.Segments.PlainText() != "Участник удалён: Пётр Сидоров" {
		t.Errorf("row text = %q, want the label snapshot", revoked.Segments.PlainText())
	}
}

// TestHistory_MemberRoleNoOpWritesNoRow pins ADR 0061 §3 «A no-op action
// writes no row»: re-setting the member's current role journals nothing —
// the same gate the role-change notice applies below the transaction.
func TestHistory_MemberRoleNoOpWritesNoRow(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	ctx := context.Background()

	if _, err := h.svc.AddMember(ctx, h.owner, h.property, h.member, domain.RoleFullAccess); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	h.history.entries = nil

	if _, err := h.svc.ChangeMemberRole(ctx, h.owner, h.property, h.repo.rows[0].ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeMemberRole (same role): %v", err)
	}
	if len(h.history.entries) != 0 {
		t.Fatalf("journal entries = %d, want 0 — a same-role change writes no row", len(h.history.entries))
	}
}

func TestHistory_LeaveCarriesTheLeaver(t *testing.T) {
	t.Parallel()
	h := newHistoryFixture(t)
	ctx := context.Background()

	if _, err := h.svc.AddMember(ctx, h.owner, h.property, h.member, domain.RoleFullAccess); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	h.history.entries = nil

	if err := h.svc.LeaveProperty(ctx, h.member, h.property); err != nil {
		t.Fatalf("LeaveProperty: %v", err)
	}
	if len(h.history.entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(h.history.entries))
	}
	e := h.history.entries[0]
	if e.Action != historydomain.ActionMemberLeft {
		t.Fatalf("action = %s, want member.left", e.Action)
	}
	// The leaver is their own label, and the row attributes them to their
	// real role — not the owner's.
	if e.Segments.PlainText() != "Участник вышел: Пётр Сидоров" {
		t.Errorf("row text = %q, want the leaver's display name", e.Segments.PlainText())
	}
	if e.ActorRole != historydomain.ActorRoleFullAccess {
		t.Errorf("actor role = %s, want full_access", e.ActorRole)
	}
	if e.ActorID == nil || *e.ActorID != h.member {
		t.Errorf("actor = %v, want the leaver", e.ActorID)
	}
}

// newInvitationHistoryFixture wires the invitation service over the in-memory
// repos with a capturing history recorder — the invitation-lifecycle twin of
// newHistoryFixture (the invitation use cases live on InvitationService, the
// factory wiring is the same).
func newInvitationHistoryFixture(t *testing.T) (
	svc *InvitationService, invitations *memInvitationsRepo, history *fakeHistory, owner, property uuid.UUID,
) {
	t.Helper()
	owner = uuid.Must(uuid.NewV7())
	property = uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	invitations = &memInvitationsRepo{removalScope: repo.inManageScope}
	owners := staticResolver{property: owner}
	lookup := newFakeLookup()
	history = &fakeHistory{}
	policy := NewMembershipPolicy(owners, repo)
	access := NewAccessService(repo, owners, nil, lookup, policy, nil, nil,
		newTestFactoryWithHistory(repo, invitations, history), nil)
	svc = NewInvitationService(access, repo, invitations, owners, nil, lookup, policy,
		nil, nil, nil, nil,
		newTestFactoryWithHistory(repo, invitations, history), nil, nil, nil)
	return svc, invitations, history, owner, property
}

// TestHistory_InvitationRoleChangeRow pins the owner decision of 24.09: the
// manual role change of a pending invitation journals like the rest of the
// invitation lifecycle — one member.role_changed row in the mutation's
// transaction, the unregistered invitee named by email (ADR 0061 §5), and a
// same-role re-set writes no row (ADR 0061 §3).
func TestHistory_InvitationRoleChangeRow(t *testing.T) {
	t.Parallel()
	svc, invitations, history, owner, property := newInvitationHistoryFixture(t)
	ctx := context.Background()

	if _, err := svc.InviteByEmail(ctx, owner, property, testNewUserEmail, domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	history.entries = nil
	pending := invitations.rows[0]

	if _, err := svc.ChangeInvitationRole(ctx, owner, property, pending.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeInvitationRole: %v", err)
	}
	if len(history.entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(history.entries))
	}
	changed := history.entries[0]
	if changed.Action != historydomain.ActionMemberRoleChanged {
		t.Fatalf("action = %s, want member.role_changed", changed.Action)
	}
	if text := changed.Segments.PlainText(); text != "Роль участника изменена: "+testNewUserEmail+": Просмотр → Полный доступ" {
		t.Errorf("row text = %q, want the old → new roles over the email snapshot", text)
	}
	if changed.ActorRole != historydomain.ActorRoleOwner {
		t.Errorf("actor role = %s, want owner", changed.ActorRole)
	}

	if _, err := svc.ChangeInvitationRole(ctx, owner, property, pending.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeInvitationRole (same role): %v", err)
	}
	if len(history.entries) != 1 {
		t.Fatalf("journal entries = %d, want still 1 — a same-role change writes no row", len(history.entries))
	}
}

func TestHistory_InvitationLifecycleRows(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	invitations := &memInvitationsRepo{removalScope: repo.inManageScope}
	owners := staticResolver{property: owner}
	lookup := newFakeLookup()
	history := &fakeHistory{}
	policy := NewMembershipPolicy(owners, repo)
	access := NewAccessService(repo, owners, nil, lookup, policy, nil, nil,
		newTestFactoryWithHistory(repo, invitations, history), nil)
	svc := NewInvitationService(access, repo, invitations, owners, nil, lookup, policy,
		nil, nil, nil, nil,
		newTestFactoryWithHistory(repo, invitations, history), nil, nil, nil)
	// The unregistered invitee journals by email — the only label the
	// platform knows at this point (ADR 0061 §5).
	if _, err := svc.InviteByEmail(context.Background(), owner, property, testNewUserEmail, domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	invited := history.entries[len(history.entries)-1]
	if invited.Action != historydomain.ActionMemberInvited {
		t.Fatalf("action = %s, want member.invited", invited.Action)
	}
	if invited.Segments.PlainText() != "Отправлено приглашение: "+testNewUserEmail {
		t.Errorf("row text = %q, want the email snapshot", invited.Segments.PlainText())
	}

	pending := invitations.rows[0]
	if err := svc.CancelInvitation(context.Background(), owner, property, pending.ID); err != nil {
		t.Fatalf("CancelInvitation: %v", err)
	}
	cancelled := history.entries[len(history.entries)-1]
	if cancelled.Action != historydomain.ActionMemberInvitationCancelled {
		t.Fatalf("action = %s, want member.invitation_cancelled", cancelled.Action)
	}
	if cancelled.Segments.PlainText() != "Приглашение отменено: "+testNewUserEmail {
		t.Errorf("row text = %q, want the email snapshot", cancelled.Segments.PlainText())
	}
}

// TestHistory_ParticipantRemovedRows pins the journal of the bulk «Отозвать и
// удалить» (issue #694): a person holding a membership and a pending
// invitation in the actor's scope gets exactly one member.participant_removed
// row per leg, both naming the same person; only the membership leg carries
// the target's user id in the context — the invitation leg knows them by
// email only (ADR 0061 §5).
func TestHistory_ParticipantRemovedRows(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	invitations := &memInvitationsRepo{removalScope: repo.inManageScope}
	owners := staticResolver{property: owner}
	repo.SetOwner(property, owner)
	lookup := newFakeLookup()
	emails := fakeEmailResolver{member: testMemberEmail}
	history := &fakeHistory{}
	policy := NewMembershipPolicy(owners, repo)
	access := NewAccessService(repo, owners, nil, lookup, policy, nil, nil,
		newTestFactoryWithHistory(repo, invitations, history), nil)
	svc := NewParticipantMutationService(access, owners, nil, lookup, emails, policy,
		nil, nil, nil, nil,
		newTestFactoryWithHistory(repo, invitations, history), nil, nil)

	ctx := context.Background()
	if _, err := repo.Create(ctx, domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: property, UserID: member,
		Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if _, err := invitations.Create(ctx, domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: property, Email: testMemberEmail,
		Role: domain.RoleViewer, InvitedBy: owner,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	if err := svc.Remove(ctx, owner, member.String()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(history.entries) != 2 {
		t.Fatalf("journal entries = %d, want 2 — one row per removal leg", len(history.entries))
	}
	for i, e := range history.entries {
		if e.Action != historydomain.ActionMemberParticipantRemoved {
			t.Fatalf("entries[%d] action = %s, want member.participant_removed", i, e.Action)
		}
		if e.PropertyID != property {
			t.Errorf("entries[%d] property = %s, want %s", i, e.PropertyID, property)
		}
		if e.Segments.PlainText() != "Участник удалён: Member" {
			t.Errorf("entries[%d] row text = %q, want the display-name snapshot on both legs", i, e.Segments.PlainText())
		}
	}
	// The legs differ in the structured context: the membership leg names the
	// removed user's id, the invitation leg has none to name.
	if got := history.entries[0].Context[historydomain.CtxKeyUserID]; got != member {
		t.Errorf("membership leg user_id = %v, want %s", got, member)
	}
	if got, ok := history.entries[1].Context[historydomain.CtxKeyUserID]; ok {
		t.Errorf("invitation leg carries user_id %v, want none", got)
	}
}
