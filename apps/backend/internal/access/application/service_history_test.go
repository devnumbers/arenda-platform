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
	members MembershipRepository, invitations InvitationRepository, audit auditapp.Recorder, history historyapp.Recorder,
) txStoreFactory {
	return NewTxStoreFactory(members, invitations, audit, history, fakeUoW{beginner: noopBeginner{}})
}

// historyFixture wires the membership services over the in-memory repos with
// a capturing history recorder, the access-test canon.
type historyFixture struct {
	t       *testing.T
	owner   uuid.UUID
	member  uuid.UUID
	strager uuid.UUID // Stand-in id of an unregistered person (never resolved).
	invited uuid.UUID

	property uuid.UUID
	repo     *memRepo
	history  *fakeHistory
	svc      *AccessService
}

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	strager := uuid.Must(uuid.NewV7())
	invited := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())

	repo := newMemRepo()
	resolver := staticResolver{property: owner}
	lookup := fakeUserLookup{
		owner:   {ID: owner, Name: new("Иван"), Surname: new("Иванов"), Phone: "+79120000001"},
		member:  {ID: member, Name: new("Пётр"), Surname: new("Сидоров"), Phone: "+79120000002"},
		strager: {ID: strager, Phone: "+79120000003"},
	}
	history := &fakeHistory{}
	policy := NewMembershipPolicy(resolver, repo)
	svc := NewAccessService(repo, resolver, nil, lookup, policy, nil, nil,
		newTestFactoryWithHistory(repo, &memInvitationsRepo{}, auditapp.Noop{}, history), nil)
	return &historyFixture{
		t: t, owner: owner, member: member, strager: strager, invited: invited,
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
		newTestFactoryWithHistory(repo, invitations, auditapp.Noop{}, history), nil)
	svc := NewInvitationService(access, repo, invitations, owners, nil, lookup, policy,
		nil, nil, nil, nil,
		newTestFactoryWithHistory(repo, invitations, auditapp.Noop{}, history), nil, nil, nil)
	// The email is unknown to the user book: the unregistered path journals
	// by email — the only label the platform knows (ADR 0061 §5).

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
