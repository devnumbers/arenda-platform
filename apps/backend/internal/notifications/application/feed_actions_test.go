package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLiveState is an in-memory FeedLiveState: the tests set exactly the
// facts the scenario needs.
type fakeLiveState struct {
	rental      RentalActionState
	rentalErr   error
	paymentOpen bool
	taskOpen    bool
	propertyAcc PropertyAccess
	propertyErr error
}

func (f *fakeLiveState) RentalActionState(_ context.Context, _ uuid.UUID) (RentalActionState, error) {
	return f.rental, f.rentalErr
}

func (f *fakeLiveState) PaymentOpen(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.paymentOpen, nil
}

func (f *fakeLiveState) TaskOpen(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.taskOpen, nil
}

func (f *fakeLiveState) PropertyAccess(_ context.Context, _, _ uuid.UUID) (PropertyAccess, error) {
	return f.propertyAcc, f.propertyErr
}

// actionNotification builds a feed row of the event type carrying the given
// payload.
func actionNotification(t *testing.T, eventType domain.EventType, payload domain.Payload) domain.Notification {
	t.Helper()
	n, err := domain.NewNotification(
		uuid.Must(uuid.NewV7()), feedTestUser, eventType,
		"Заголовок", "Тело", "Объект", payload,
		domain.DedupKey(string(eventType)+":actions-test"),
	)
	require.NoError(t, err)
	return *n
}

// rental_completed offers both buttons only while the rental still awaits
// action and the reader may act on it (решение #737).
func TestAvailableActions_RentalCompleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	n := actionNotification(t, domain.EventRentalCompleted,
		domain.Payload{RentalID: new(uuid.Must(uuid.NewV7())), Property: &domain.EntityRef{}})

	awaiting := &fakeLiveState{
		rental:      RentalActionState{Exists: true, AwaitingAction: true, PropertyID: uuid.Must(uuid.NewV7())},
		propertyAcc: PropertyAccess{Exists: true, Manageable: true},
	}
	actions, err := availableActions(ctx, awaiting, feedTestUser, n)
	require.NoError(t, err)
	assert.Equal(t, []domain.ActionKind{domain.ActionRentalExtend, domain.ActionRentalComplete}, actions,
		"both buttons in catalog order")

	// The state moved on: the rental was extended (active again) or completed.
	movedOn := &fakeLiveState{
		rental:      RentalActionState{Exists: true, AwaitingAction: false, PropertyID: awaiting.rental.PropertyID},
		propertyAcc: PropertyAccess{Exists: true, Manageable: true},
	}
	actions, err = availableActions(ctx, movedOn, feedTestUser, n)
	require.NoError(t, err)
	assert.Empty(t, actions, "состояние ушло — кнопок нет")

	// The reader is a «Просмотр» member: notified, but without mutation buttons.
	viewer := &fakeLiveState{
		rental:      awaiting.rental,
		propertyAcc: PropertyAccess{Exists: true, Manageable: false},
	}
	actions, err = availableActions(ctx, viewer, feedTestUser, n)
	require.NoError(t, err)
	assert.Empty(t, actions, "роли «Просмотр» кнопки-мутации не полагаются")

	// The rental was deleted.
	gone := &fakeLiveState{
		rental:      RentalActionState{},
		propertyAcc: PropertyAccess{Exists: true, Manageable: true},
	}
	actions, err = availableActions(ctx, gone, feedTestUser, n)
	require.NoError(t, err)
	assert.Empty(t, actions)
}

// The open_* navigation actions stand while their entity stands.
func TestAvailableActions_EntityGates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	propertyID := uuid.Must(uuid.NewV7())

	payment := actionNotification(t, domain.EventPaymentDue, domain.Payload{PaymentID: new(uuid.Must(uuid.NewV7()))})
	paymentLive := &fakeLiveState{paymentOpen: true}
	actions, err := availableActions(ctx, paymentLive, feedTestUser, payment)
	require.NoError(t, err)
	assert.Equal(t, []domain.ActionKind{domain.ActionOpenPayment}, actions)

	paymentLive.paymentOpen = false
	actions, err = availableActions(ctx, paymentLive, feedTestUser, payment)
	require.NoError(t, err)
	assert.Empty(t, actions, "платёж закрыт — кнопки нет")

	task := actionNotification(t, domain.EventTaskOverdue, domain.Payload{TaskID: new(uuid.Must(uuid.NewV7()))})
	taskLive := &fakeLiveState{taskOpen: true}
	actions, err = availableActions(ctx, taskLive, feedTestUser, task)
	require.NoError(t, err)
	assert.Equal(t, []domain.ActionKind{domain.ActionOpenTask}, actions)

	taskLive.taskOpen = false
	actions, err = availableActions(ctx, taskLive, feedTestUser, task)
	require.NoError(t, err)
	assert.Empty(t, actions)

	invitation := actionNotification(t, domain.EventPropertyInvitation,
		domain.Payload{Property: &domain.EntityRef{ID: propertyID}})
	accessLive := &fakeLiveState{propertyAcc: PropertyAccess{Exists: true, Manageable: false}}
	actions, err = availableActions(ctx, accessLive, feedTestUser, invitation)
	require.NoError(t, err)
	assert.Equal(t, []domain.ActionKind{domain.ActionOpenProperty}, actions, "просмотр тоже открывает объект")

	accessLive.propertyAcc = PropertyAccess{}
	actions, err = availableActions(ctx, accessLive, feedTestUser, invitation)
	require.NoError(t, err)
	assert.Empty(t, actions, "объект удалён — уведомление остаётся без действий")
}

// The service categories' navigation targets are global screens — no entity
// to live-check, the buttons always stand; a system notification renders
// without buttons.
func TestAvailableActions_ServiceCategories(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	grace := actionNotification(t, domain.EventSubscriptionGraceEntered, domain.Payload{})
	actions, err := availableActions(ctx, &fakeLiveState{}, feedTestUser, grace)
	require.NoError(t, err)
	assert.Equal(t, []domain.ActionKind{domain.ActionOpenPaymentMethods}, actions)

	maintenance := actionNotification(t, domain.EventSystemMaintenance, domain.Payload{})
	actions, err = availableActions(ctx, &fakeLiveState{}, feedTestUser, maintenance)
	require.NoError(t, err)
	assert.Empty(t, actions)
}

// Get returns the row with live actions; foreign and deleted rows do not
// exist for the reader.
func TestFeedService_Get(t *testing.T) {
	t.Parallel()

	repo := newFakeFeedQuery()
	propertyID := uuid.Must(uuid.NewV7())
	live := &fakeLiveState{
		rental:      RentalActionState{Exists: true, AwaitingAction: true, PropertyID: propertyID},
		propertyAcc: PropertyAccess{Exists: true, Manageable: true},
	}
	svc := NewFeedService(repo, live)

	n, err := domain.NewNotification(
		uuid.Must(uuid.NewV7()), feedTestUser, domain.EventRentalCompleted,
		"Аренда завершена", "Срок аренды истёк. Продлите или завершите аренду.", "Объект",
		domain.Payload{RentalID: new(uuid.Must(uuid.NewV7())), Property: &domain.EntityRef{ID: propertyID}},
		domain.DedupKey("rental_completed:actions-get"),
	)
	require.NoError(t, err)
	_, err = repo.Insert(context.Background(), *n)
	require.NoError(t, err)

	detail, err := svc.Get(context.Background(), feedTestUser, n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.ID, detail.Notification.ID)
	assert.Equal(t, []domain.ActionKind{domain.ActionRentalExtend, domain.ActionRentalComplete}, detail.Actions)

	// A foreign row does not exist for the reader.
	stranger := uuid.Must(uuid.NewV7())
	_, err = svc.Get(context.Background(), stranger, n.ID)
	require.ErrorIs(t, err, ErrNotFound)

	// A deleted row does not exist either.
	require.NoError(t, svc.Delete(context.Background(), feedTestUser, n.ID))
	_, err = svc.Get(context.Background(), feedTestUser, n.ID)
	require.ErrorIs(t, err, ErrNotFound)
}
