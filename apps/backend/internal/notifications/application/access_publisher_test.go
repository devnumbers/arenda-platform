package application

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessViews is the AccessEventViewSource stub: id-keyed property and user
// profiles the tests plant; a missing id errors like a real store would.
type accessViews struct {
	props map[uuid.UUID]AccessPropertyView
	users map[uuid.UUID]AccessUserProfile
}

func (v *accessViews) PropertyView(_ context.Context, propertyID uuid.UUID) (AccessPropertyView, error) {
	view, ok := v.props[propertyID]
	if !ok {
		return AccessPropertyView{}, ErrNotFound
	}
	return view, nil
}

func (v *accessViews) UserProfileView(_ context.Context, userID uuid.UUID) (AccessUserProfile, error) {
	profile, ok := v.users[userID]
	if !ok {
		return AccessUserProfile{}, ErrNotFound
	}
	return profile, nil
}

// accessPublisherHarness builds the access publisher over the real pipeline
// backed by the publisher tests' fakes, so the tests read the feed rows the
// events created.
type accessPublisherHarness struct {
	feed  *fakeFeedRepo
	pub   *AccessPublisher
	views *accessViews
}

func newAccessPublisherHarness() *accessPublisherHarness {
	feed := &fakeFeedRepo{}
	views := &accessViews{props: map[uuid.UUID]AccessPropertyView{}, users: map[uuid.UUID]AccessUserProfile{}}
	pipeline := NewPublisher(feed, &fakeQueue{}, nil, &fakeUoW{}, nil)
	return &accessPublisherHarness{
		feed:  feed,
		pub:   NewAccessPublisher(pipeline, views),
		views: views,
	}
}

// accessIDs is the shared id fixture of the tests — table-driven and plain
// alike: a property, the two users of an access transition and the membership
// the transition landed on, spelled once as the stable …a001–…a004 literals.
type accessIDs struct {
	property   uuid.UUID
	owner      uuid.UUID
	member     uuid.UUID
	membership uuid.UUID
}

func newAccessIDs() accessIDs {
	return accessIDs{
		property:   uuid.MustParse("00000000-0000-7000-8000-00000000a001"),
		owner:      uuid.MustParse("00000000-0000-7000-8000-00000000a002"),
		member:     uuid.MustParse("00000000-0000-7000-8000-00000000a003"),
		membership: uuid.MustParse("00000000-0000-7000-8000-00000000a004"),
	}
}

func (h *accessPublisherHarness) plantProperty(id uuid.UUID, name, address string) {
	h.views.props[id] = AccessPropertyView{Name: name, Address: address}
}

func (h *accessPublisherHarness) plantUser(id uuid.UUID, name, email string) {
	h.views.users[id] = AccessUserProfile{DisplayName: name, Email: email}
}

// notifyActivated publishes one active-or-suspended activation of the shared
// fixture ids and indexes the rows it created by event type.
func (h *accessPublisherHarness) notifyActivated(t *testing.T, ids accessIDs, suspended bool) map[domain.EventType]domain.Notification {
	t.Helper()
	err := h.pub.NotifyInvitationActivated(
		t.Context(), ids.membership, ids.property, ids.owner, ids.member,
		suspended, time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.Len(t, h.feed.inserted, 2)

	byEvent := make(map[domain.EventType]domain.Notification, 2)
	for _, n := range h.feed.inserted {
		byEvent[n.EventType] = n
	}
	return byEvent
}

// TestNotifyInvitationActivatedActive checks the active activation's fan-out
// (решение #737, типы №5–№6): the invitee's «Приглашение в объект» row with
// the inviter as the actor, and the inviter's «Приглашение принято» row with
// the invitee as the actor — the texts verbatim, the dedup keys bare
// membership ids.
func TestNotifyInvitationActivatedActive(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Дом на Рублёвке", "ул. Рублёвское шоссе, 1")
	h.plantUser(ids.owner, "Пётр Петров", "inviter@example.com")
	h.plantUser(ids.member, "Иван Иванов", "invitee@example.com")

	byEvent := h.notifyActivated(t, ids, false)

	invite := byEvent[domain.EventPropertyInvitation]
	require.NotZero(t, invite.ID, "the invitee's invitation row must exist")
	assert.Equal(t, ids.member, invite.UserID)
	assert.Equal(t, "Приглашение в объект", invite.Title)
	assert.Equal(t, "Пётр Петров пригласил вас в объект «Дом на Рублёвке». Теперь объект доступен вам совместно", invite.Body)
	assert.Equal(t, "Дом на Рублёвке", invite.ContextLabel)
	assert.Equal(t, domain.DedupKey("property_invitation:"+ids.membership.String()), invite.DedupKey)
	assert.Equal(t, domain.CategorySharedAccess, invite.Category)
	require.NotNil(t, invite.Payload.Property)
	assert.Equal(t, ids.property, invite.Payload.Property.ID)
	assert.Equal(t, "Дом на Рублёвке", invite.Payload.Property.Name)
	assert.Equal(t, "ул. Рублёвское шоссе, 1", invite.Payload.Property.Address)
	require.NotNil(t, invite.Payload.Actor)
	assert.Equal(t, ids.owner, invite.Payload.Actor.ID)
	assert.Equal(t, "Пётр Петров", invite.Payload.Actor.Name)
	assert.Equal(t, "inviter@example.com", invite.Payload.Actor.Email)
	require.NotNil(t, invite.Payload.MembershipID)
	assert.Equal(t, ids.membership, *invite.Payload.MembershipID)

	accepted := byEvent[domain.EventInvitationAccepted]
	require.NotZero(t, accepted.ID, "the inviter's accepted row must exist")
	assert.Equal(t, ids.owner, accepted.UserID)
	assert.Equal(t, "Приглашение принято", accepted.Title)
	assert.Equal(t, "Иван Иванов принял приглашение в объект «Дом на Рублёвке»", accepted.Body)
	assert.Equal(t, "Дом на Рублёвке", accepted.ContextLabel)
	assert.Equal(t, domain.DedupKey("invitation_accepted:"+ids.membership.String()), accepted.DedupKey)
	require.NotNil(t, accepted.Payload.Actor)
	assert.Equal(t, ids.member, accepted.Payload.Actor.ID)
	assert.Equal(t, "Иван Иванов", accepted.Payload.Actor.Name)
	assert.Equal(t, "invitee@example.com", accepted.Payload.Actor.Email)
}

// TestNotifyInvitationActivatedSuspended checks the no-slot activation: the
// invitee gets the system «Доступ приостановлен» row instead of the
// invitation one (the «Теперь объект доступен вам совместно» copy would be
// false), the inviter still learns the invitation was accepted — the copy
// names the no-name invitee by the anonymous label «Пользователь» (карта
// #1105, аменд #1123: телефон больше не фолбэк) — and the paused row's
// dedup key carries the activation instant.
func TestNotifyInvitationActivatedSuspended(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Квартира на Невском", "Невский проспект, 5")
	h.plantUser(ids.owner, "Пётр Петров", "inviter@example.com")
	h.plantUser(ids.member, "Пользователь", "invitee@example.com")

	byEvent := h.notifyActivated(t, ids, true)

	assert.Zero(t, byEvent[domain.EventPropertyInvitation].ID,
		"no invitation row when the access landed suspended")

	accepted := byEvent[domain.EventInvitationAccepted]
	require.NotZero(t, accepted.ID, "the inviter still learns about the acceptance")
	assert.Equal(t, ids.owner, accepted.UserID)
	assert.Equal(t, "Пользователь принял приглашение в объект «Квартира на Невском»", accepted.Body)

	paused := byEvent[domain.EventAccessPaused]
	require.NotZero(t, paused.ID, "the invitee learns the access waits for a slot")
	assert.Equal(t, ids.member, paused.UserID)
	assert.Equal(t, "Доступ приостановлен", paused.Title)
	assert.Equal(t, "Ваш доступ к объекту «Квартира на Невском» приостановлен. Данные объекта скрыты, пока доступ приостановлен", paused.Body)
	activatedAt := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, domain.DedupKey("access_paused:"+ids.membership.String()+":"+unixDedupStamp(activatedAt)), paused.DedupKey)
	assert.Nil(t, paused.Payload.Actor, "a system suspension has no actor card")
	require.NotNil(t, paused.Payload.Property)
	assert.Equal(t, ids.property, paused.Payload.Property.ID)
}

// TestNotifyMembershipSuspendedCopy checks the pause's two renderings
// (решение #737, тип №8): the system suspension — the no-actor copy today's
// slot enforcement always produces — and the with-actor one a future manual
// suspend grows into; both stamp the dedup key with the instant.
func TestNotifyMembershipSuspendedCopy(t *testing.T) {
	t.Parallel()

	ids := newAccessIDs()
	for _, tc := range []struct {
		name         string
		actorID      uuid.UUID
		actorProfile string
		actorEmail   string
		wantBody     string
		wantActorRef bool
	}{
		{
			name:         "system pause renders the no-name copy",
			actorID:      uuid.Nil,
			wantBody:     "Ваш доступ к объекту «Дом на Рублёвке» приостановлен. Данные объекта скрыты, пока доступ приостановлен",
			wantActorRef: false,
		},
		{
			name:         "human actor names the copy and the card",
			actorID:      ids.owner,
			actorProfile: "Пётр Петров",
			actorEmail:   "owner@example.com",
			wantBody:     "Пётр Петров приостановил ваш доступ к объекту «Дом на Рублёвке». Данные объекта скрыты, пока доступ приостановлен",
			wantActorRef: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newAccessPublisherHarness()
			h.plantProperty(ids.property, "Дом на Рублёвке", "")
			if tc.actorProfile != "" {
				h.plantUser(tc.actorID, tc.actorProfile, tc.actorEmail)
			}

			suspendedAt := time.Date(2026, 9, 20, 15, 30, 0, 0, time.UTC)
			err := h.pub.NotifyMembershipSuspended(
				t.Context(), ids.membership, ids.property, ids.member,
				tc.actorID, suspendedAt,
			)
			require.NoError(t, err)
			require.Len(t, h.feed.inserted, 1)

			n := h.feed.inserted[0]
			assert.Equal(t, domain.EventAccessPaused, n.EventType)
			assert.Equal(t, ids.member, n.UserID)
			assert.Equal(t, "Доступ приостановлен", n.Title)
			assert.Equal(t, tc.wantBody, n.Body)
			assert.Equal(t, "Дом на Рублёвке", n.ContextLabel)
			assert.Equal(t,
				domain.DedupKey("access_paused:"+ids.membership.String()+":"+unixDedupStamp(suspendedAt)),
				n.DedupKey)
			if tc.wantActorRef {
				require.NotNil(t, n.Payload.Actor)
				assert.Equal(t, tc.actorID, n.Payload.Actor.ID)
				assert.Equal(t, tc.actorEmail, n.Payload.Actor.Email)
				return
			}
			assert.Nil(t, n.Payload.Actor)
		})
	}
}

// TestNotifyMembershipResumed checks the recovery copy (решение #737, тип
// №9): today the recovery is always the slot coordinator's — the no-actor
// rendering — and the key carries the instant.
func TestNotifyMembershipResumed(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Квартира на Невском", "")

	resumedAt := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	err := h.pub.NotifyMembershipResumed(
		context.Background(), ids.membership, ids.property, ids.member, uuid.Nil, resumedAt,
	)
	require.NoError(t, err)
	require.Len(t, h.feed.inserted, 1)

	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventAccessResumed, n.EventType)
	assert.Equal(t, ids.member, n.UserID)
	assert.Equal(t, "Доступ восстановлен", n.Title)
	assert.Equal(t, "Ваш доступ к объекту «Квартира на Невском» восстановлен", n.Body)
	assert.Equal(t, domain.DedupKey("access_resumed:"+ids.membership.String()+":"+unixDedupStamp(resumedAt)), n.DedupKey)
}

// TestNotifyHumanActorTransitions checks the two human-actor transitions'
// copies and keys (решение #737, типы №7 и №10): the revoke names the
// revoking owner, the self-exit names the leaving member to the owner.
func TestNotifyHumanActorTransitions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		notify    func(h *accessPublisherHarness, ids accessIDs) error
		wantEvent domain.EventType
		wantUser  func(ids accessIDs) uuid.UUID
		wantActor func(ids accessIDs) uuid.UUID
		wantTitle string
		wantBody  string
		wantKey   string
	}{
		{
			name: "revoke",
			notify: func(h *accessPublisherHarness, ids accessIDs) error {
				return h.pub.NotifyAccessRevoked(t.Context(),
					ids.membership, ids.property, ids.member, ids.owner)
			},
			wantEvent: domain.EventAccessRevoked,
			wantUser:  func(ids accessIDs) uuid.UUID { return ids.member },
			wantActor: func(ids accessIDs) uuid.UUID { return ids.owner },
			wantTitle: "Доступ отозван",
			wantBody:  "Пётр Петров отозвал ваш доступ к объекту «Дом на Рублёвке». Объект больше не отображается в вашей книге",
			wantKey:   "access_revoked:",
		},
		{
			name: "member left",
			notify: func(h *accessPublisherHarness, ids accessIDs) error {
				return h.pub.NotifyMemberLeft(t.Context(),
					ids.membership, ids.property, ids.owner, ids.member)
			},
			wantEvent: domain.EventMemberLeft,
			wantUser:  func(ids accessIDs) uuid.UUID { return ids.owner },
			wantActor: func(ids accessIDs) uuid.UUID { return ids.member },
			wantTitle: "Участник покинул объект",
			wantBody:  "Иван Иванов больше не имеет доступа к объекту «Дом на Рублёвке»",
			wantKey:   "member_left:",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newAccessPublisherHarness()
			ids := newAccessIDs()
			h.plantProperty(ids.property, "Дом на Рублёвке", "")
			h.plantUser(ids.owner, "Пётр Петров", "owner@example.com")
			h.plantUser(ids.member, "Иван Иванов", "member@example.com")

			require.NoError(t, tc.notify(h, ids))
			require.Len(t, h.feed.inserted, 1)

			n := h.feed.inserted[0]
			assert.Equal(t, tc.wantEvent, n.EventType)
			assert.Equal(t, tc.wantUser(ids), n.UserID)
			assert.Equal(t, tc.wantTitle, n.Title)
			assert.Equal(t, tc.wantBody, n.Body)
			assert.Equal(t, domain.DedupKey(tc.wantKey+ids.membership.String()), n.DedupKey)
			require.NotNil(t, n.Payload.Actor)
			assert.Equal(t, tc.wantActor(ids), n.Payload.Actor.ID)
		})
	}
}

// TestNotifyViewFailure checks a view resolution failure fails the
// publication (an abnormal store error must not silently drop the copy's
// subject) and creates no rows.
func TestNotifyViewFailure(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	// No property planted: the view lookup errors.

	err := h.pub.NotifyAccessRevoked(
		context.Background(), ids.membership, ids.property, ids.member, ids.owner,
	)
	require.Error(t, err)
	assert.Empty(t, h.feed.inserted)

	err = h.pub.NotifyInvitationActivated(
		context.Background(), ids.membership, ids.property, ids.owner, ids.member,
		false, time.Now(),
	)
	require.Error(t, err)
	assert.Empty(t, h.feed.inserted)
}

// TestNotifyActorProfileFailure checks a failed actor lookup fails the
// publication the actor's copy depends on.
func TestNotifyActorProfileFailure(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Дом на Рублёвке", "")
	// Owner profile not planted.

	err := h.pub.NotifyAccessRevoked(
		context.Background(), ids.membership, ids.property, ids.member, ids.owner,
	)
	require.Error(t, err)
	assert.Empty(t, h.feed.inserted)
}

// TestNotifyMembershipGranted checks the instant landing's invitation row
// (issue #829, решение #737 тип №5): a registered user granted access right
// away learns about it with the catalog's verbatim copy — the inviter names
// the text and the actor card, the bare membership id keys the row (revoke →
// re-invite is a new membership, hence a new row).
func TestNotifyMembershipGranted(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Дом на Рублёвке", "ул. Рублёвское шоссе, 1")
	h.plantUser(ids.owner, "Пётр Петров", "inviter@example.com")

	err := h.pub.NotifyMembershipGranted(
		t.Context(), ids.membership, ids.property, ids.member, ids.owner,
	)
	require.NoError(t, err)
	require.Len(t, h.feed.inserted, 1, "the granted member's row is the only one — no «Приглашение принято» on an instant landing")

	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventPropertyInvitation, n.EventType)
	assert.Equal(t, ids.member, n.UserID)
	assert.Equal(t, "Приглашение в объект", n.Title)
	assert.Equal(t, "Пётр Петров пригласил вас в объект «Дом на Рублёвке». Теперь объект доступен вам совместно", n.Body)
	assert.Equal(t, "Дом на Рублёвке", n.ContextLabel)
	assert.Equal(t, domain.DedupKey("property_invitation:"+ids.membership.String()), n.DedupKey)
	assert.Equal(t, domain.CategorySharedAccess, n.Category)
	require.NotNil(t, n.Payload.Property)
	assert.Equal(t, ids.property, n.Payload.Property.ID)
	assert.Equal(t, "Дом на Рублёвке", n.Payload.Property.Name)
	assert.Equal(t, "ул. Рублёвское шоссе, 1", n.Payload.Property.Address)
	require.NotNil(t, n.Payload.Actor)
	assert.Equal(t, ids.owner, n.Payload.Actor.ID)
	assert.Equal(t, "Пётр Петров", n.Payload.Actor.Name)
	assert.Equal(t, "inviter@example.com", n.Payload.Actor.Email)
	require.NotNil(t, n.Payload.MembershipID)
	assert.Equal(t, ids.membership, *n.Payload.MembershipID)
}

// TestNotifyInvitationActivatedDedupStable checks the dedup pair of a
// repeated publication: the same activation inserts nothing new.
func TestNotifyInvitationActivatedDedupStable(t *testing.T) {
	t.Parallel()

	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Дом на Рублёвке", "")
	h.plantUser(ids.owner, "Пётр Петров", "inviter@example.com")
	h.plantUser(ids.member, "Иван Иванов", "invitee@example.com")

	for range 2 {
		err := h.pub.NotifyInvitationActivated(
			t.Context(), ids.membership, ids.property,
			ids.owner, ids.member, false, time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
	}
	assert.Len(t, h.feed.inserted, 2, "the second publication inserts nothing")
}

// TestNotifyRoleChanged checks the role-change row (карта #828, тикет #830):
// the member learns the manager's change with the catalog's verbatim copy —
// the display role names of the #692 chart canon («Редактирование»/«Просмотр»)
// — the changer the actor, the change instant keying the row.
func TestNotifyRoleChanged(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		role      string
		wantLabel string
	}{
		{"full_access", "Редактирование"},
		{"viewer", "Просмотр"},
	} {
		t.Run(tc.role, func(t *testing.T) {
			t.Parallel()
			h := newAccessPublisherHarness()
			ids := newAccessIDs()
			h.plantProperty(ids.property, "Дом на Рублёвке", "ул. Рублёвское шоссе, 1")
			h.plantUser(ids.owner, "Пётр Петров", "owner@example.com")
			changedAt := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)

			err := h.pub.NotifyRoleChanged(
				t.Context(), ids.membership, ids.property, ids.member, ids.owner,
				tc.role, changedAt,
			)
			require.NoError(t, err)
			require.Len(t, h.feed.inserted, 1, "the member's row is the only one")

			n := h.feed.inserted[0]
			assert.Equal(t, domain.EventAccessRoleChanged, n.EventType)
			assert.Equal(t, domain.CategorySharedAccess, n.Category)
			assert.Equal(t, ids.member, n.UserID)
			assert.Equal(t, "Роль изменена", n.Title)
			assert.Equal(t,
				fmt.Sprintf("Пётр Петров изменил вашу роль в объекте «Дом на Рублёвке» на «%s»", tc.wantLabel),
				n.Body)
			assert.Equal(t, "Дом на Рублёвке", n.ContextLabel)
			assert.Equal(t,
				domain.DedupKey("access_role_changed:"+ids.membership.String()+":"+strconv.FormatInt(changedAt.Unix(), 10)),
				n.DedupKey)
			require.NotNil(t, n.Payload.Property)
			assert.Equal(t, ids.property, n.Payload.Property.ID)
			assert.Equal(t, "Дом на Рублёвке", n.Payload.Property.Name)
			require.NotNil(t, n.Payload.Actor)
			assert.Equal(t, ids.owner, n.Payload.Actor.ID)
			assert.Equal(t, "Пётр Петров", n.Payload.Actor.Name)
			require.NotNil(t, n.Payload.MembershipID)
			assert.Equal(t, ids.membership, *n.Payload.MembershipID)
		})
	}
}

// TestNotifyRoleChangedDedupPerChange pins the every-change-notifies rule
// (тикет #830): each new change instant yields a fresh row; a repeated
// publication of the same change inserts nothing.
func TestNotifyRoleChangedDedupPerChange(t *testing.T) {
	t.Parallel()
	h := newAccessPublisherHarness()
	ids := newAccessIDs()
	h.plantProperty(ids.property, "Дом на Рублёвке", "")
	h.plantUser(ids.owner, "Пётр Петров", "owner@example.com")

	first := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	for _, at := range []time.Time{first, second, second} {
		err := h.pub.NotifyRoleChanged(
			t.Context(), ids.membership, ids.property, ids.member, ids.owner,
			"viewer", at,
		)
		require.NoError(t, err)
	}
	assert.Len(t, h.feed.inserted, 2, "each change inserts a row, the repeat inserts nothing")
}

// UnixDedupStamp lives in access_publisher.go — the tests assert the exact
// key shape with the same rendering the implementation commits to.
