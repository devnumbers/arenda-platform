package events

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	platformevents "github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
)

// fakeDispatcher captures the last Publish call so the Publisher test can assert
// the event type and payload are forwarded correctly.
type fakeDispatcher struct {
	lastEventType platformevents.EventType
	lastEvent     any
	err           error
}

func (d *fakeDispatcher) Subscribe(platformevents.EventType, platformevents.Handler) {}

func (d *fakeDispatcher) Publish(_ context.Context, eventType platformevents.EventType, event any) error {
	d.lastEventType = eventType
	d.lastEvent = event
	return d.err
}

func TestPublisher_PublishUserRegistered(t *testing.T) {
	t.Parallel()
	d := &fakeDispatcher{}
	pub := NewPublisher(d)

	userID := uuid.Must(uuid.NewV7())
	phone, err := domain.NewPhone("+79160004000")
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	email, err := domain.NewEmail("owner@example.com")
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	event := identityapp.UserRegistered{
		UserID: userID,
		Phone:  phone,
		Email:  email,
	}

	if err := pub.PublishUserRegistered(t.Context(), event); err != nil {
		t.Fatalf("PublishUserRegistered error = %v", err)
	}
	if d.lastEventType != platformevents.EventType("user_registered") {
		t.Fatalf("event type = %q, want user_registered", d.lastEventType)
	}
	got, ok := d.lastEvent.(identityapp.UserRegistered)
	if !ok {
		t.Fatalf("event type = %T, want identityapp.UserRegistered", d.lastEvent)
	}
	if got.UserID != userID {
		t.Fatalf("event UserID = %s, want %s", got.UserID, userID)
	}
	if got.Phone != phone {
		t.Fatalf("event Phone = %s, want %s", got.Phone, phone)
	}
	if got.Email != email {
		t.Fatalf("event Email = %s, want %s", got.Email, email)
	}
}

func TestPublisher_PublishUserRegistered_PropagatesError(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("dispatch failed")
	d := &fakeDispatcher{err: dbErr}
	pub := NewPublisher(d)

	err := pub.PublishUserRegistered(t.Context(), identityapp.UserRegistered{})
	if !errors.Is(err, dbErr) {
		t.Fatalf("PublishUserRegistered error = %v, want wrap of dbErr", err)
	}
}

// Compile-time guard that Publisher implements identityapp.EventPublisher.
var _ identityapp.EventPublisher = (*Publisher)(nil)
