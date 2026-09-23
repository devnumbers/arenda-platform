package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogCases pins the catalog v1 (решение #737): every feed event type
// carries its category and its possible actions — the knowledge the reading
// side (#743) computes live button availability on top of.
var catalogCases = []struct {
	eventType    EventType
	wantCategory Category
	wantActions  []ActionKind
}{
	{EventRentalCompleted, CategoryRental, []ActionKind{ActionRentalExtend, ActionRentalComplete}},
	{EventPaymentDue, CategoryPaymentsOperations, []ActionKind{ActionOpenPayment}},
	{EventPaymentOverdue, CategoryPaymentsOperations, []ActionKind{ActionOpenPayment}},
	{EventTaskOverdue, CategoryTasks, []ActionKind{ActionOpenTask}},
	{EventPropertyInvitation, CategorySharedAccess, []ActionKind{ActionOpenProperty}},
	{EventInvitationAccepted, CategorySharedAccess, []ActionKind{ActionOpenPropertyMembers}},
	{EventAccessRevoked, CategorySharedAccess, nil},
	{EventAccessPaused, CategorySharedAccess, nil},
	{EventAccessResumed, CategorySharedAccess, nil},
	{EventMemberLeft, CategorySharedAccess, nil},
	{EventAccessRoleChanged, CategorySharedAccess, []ActionKind{ActionOpenPropertyMembers}},
	{EventSubscriptionPaymentFailed, CategoryTariff, []ActionKind{ActionOpenTariffs}},
	{EventSubscriptionPaymentReminder, CategoryTariff, []ActionKind{ActionOpenTariffs}},
	{EventSubscriptionPaymentSucceeded, CategoryTariff, nil},
	{EventSubscriptionPlanChanged, CategoryTariff, []ActionKind{ActionOpenTariffs}},
	{EventSubscriptionGraceEntered, CategoryTariff, []ActionKind{ActionOpenPaymentMethods}},
	{EventSubscriptionGraceExpiring, CategoryTariff, []ActionKind{ActionOpenPaymentMethods}},
	{EventSystemMaintenance, CategorySystem, nil},
}

func TestNewNotification_AssignsCatalogCategory(t *testing.T) {
	t.Parallel()

	for _, tc := range catalogCases {
		n, err := NewNotification(
			newID(), newID(),
			tc.eventType,
			"Заголовок", "Тело", "Контекст",
			Payload{}, DedupKey("key"),
		)
		require.NoError(t, err, tc.eventType)
		assert.Equal(t, tc.wantCategory, n.Category, "category of %s", tc.eventType)
		assert.Equal(t, tc.eventType, n.EventType)
	}
}

func TestActionsForEvent(t *testing.T) {
	t.Parallel()

	for _, tc := range catalogCases {
		actions := ActionsForEvent(tc.eventType)
		if tc.wantActions == nil {
			assert.Empty(t, actions, "actions of %s", tc.eventType)
			continue
		}
		assert.Equal(t, tc.wantActions, actions, "actions of %s", tc.eventType)
	}

	// Free_reminder is a dead PostgreSQL enum value (#277): the database
	// keeps it forever, the catalog does not know it.
	assert.Empty(t, ActionsForEvent(EventType("free_reminder")))
}

func TestNewNotification_RejectsInvalid(t *testing.T) {
	t.Parallel()

	id := newID()
	userID := newID()
	payload := Payload{}
	build := func(nid, nuserID uuid.UUID, eventType EventType, title, body string, dedup DedupKey) error {
		_, err := NewNotification(nid, nuserID, eventType, title, body, "", payload, dedup)
		return err
	}

	require.ErrorIs(t, build(uuid.Nil, userID, EventPaymentDue, "З", "Т", "k"), ErrInvalidNotification, "zero notification id")
	require.ErrorIs(t, build(id, uuid.Nil, EventPaymentDue, "З", "Т", "k"), ErrInvalidNotification, "zero recipient")
	require.ErrorIs(t, build(id, userID, EventType("free_reminder"), "З", "Т", "k"), ErrInvalidNotification,
		"a dead enum value outside the catalog makes no feed row")
	require.ErrorIs(t, build(id, userID, EventType("bogus"), "З", "Т", "k"), ErrInvalidNotification)
	require.ErrorIs(t, build(id, userID, EventPaymentDue, "", "Т", "k"), ErrInvalidNotification, "empty title")
	require.ErrorIs(t, build(id, userID, EventPaymentDue, "З", "", "k"), ErrInvalidNotification, "empty body")
	require.ErrorIs(t, build(id, userID, EventPaymentDue, "З", "Т", ""), ErrInvalidNotification, "empty dedup key")
}

func TestNotification_PayloadJSONRoundTrip(t *testing.T) {
	t.Parallel()

	until := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	propertyID := newID()
	actorID := newID()
	full := Payload{
		Property: &EntityRef{ID: propertyID, Name: "Объект"},
		Actor:    &EntityRef{ID: actorID, Name: "Иван"},
		RentalID: new(newID()),
		TaskID:   new(newID()),
		Tariff: &TariffRef{
			Slug:          "pro",
			Period:        "monthly",
			AmountKopecks: 29900,
			ActiveUntil:   &until,
		},
	}

	data, err := json.Marshal(full)
	require.NoError(t, err)

	var decoded Payload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, full, decoded)
	assert.Equal(t, "Объект", decoded.Property.Name)
	require.NotNil(t, decoded.Tariff)
	assert.Equal(t, int64(29900), decoded.Tariff.AmountKopecks)

	// A payload without refs marshals to an empty object — the snapshot
	// lives in title/body, the payload only carries links.
	empty, err := json.Marshal(Payload{})
	require.NoError(t, err)
	assert.JSONEq(t, "{}", string(empty))
}

func TestNewDedupKey(t *testing.T) {
	t.Parallel()

	key, err := NewDedupKey("payment_due:8f0c…:2026-10-01")
	require.NoError(t, err)
	assert.Equal(t, DedupKey("payment_due:8f0c…:2026-10-01"), key)

	_, err = NewDedupKey("   ")
	assert.ErrorIs(t, err, ErrInvalidNotification, "blank dedup key")
}

// newID returns a fresh app-generated UUIDv7 (ADR 0019).
func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }
