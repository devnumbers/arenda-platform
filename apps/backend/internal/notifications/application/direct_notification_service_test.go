package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// The direct-notification delivery of issue #253: both channels honour the
// user's per-channel preferences (ADR 0030), every channel failure is
// best-effort — logged and never failing the other channel — and a nil push
// sender keeps delivery email-only.

// testOwnerEmail is the shared contact fixture of the direct-notification tests.
const testOwnerEmail = "owner@example.com"

// channelPrefsRepo embeds the policy-test fake and answers IsChannelAllowed
// from a per-channel map.
type channelPrefsRepo struct {
	fakePreferenceRepo
	allowed map[domain.NotificationChannel]bool
}

func (r *channelPrefsRepo) IsChannelAllowed(
	_ context.Context, _ uuid.UUID, _ domain.EventType, ch domain.NotificationChannel,
) (bool, error) {
	return r.allowed[ch], nil
}

// fakeResolver resolves every user to the scripted contact.
type fakeResolver struct {
	contact Contact
	err     error
}

func (r fakeResolver) Resolve(context.Context, uuid.UUID) (Contact, error) {
	return r.contact, r.err
}

// captureEmailer records SendDirect calls.
type captureEmailer struct {
	calls []captureEmail
	err   error
}

type captureEmail struct {
	to, subject, template string
}

func (e *captureEmailer) SendDirect(_ context.Context, to, subject, template string, _ map[string]any) error {
	if e.err != nil {
		return e.err
	}
	e.calls = append(e.calls, captureEmail{to: to, subject: subject, template: template})
	return nil
}

// fakePushSender scripts one outcome per endpoint and records the sends.
type fakePushSender struct {
	errByEndpoint map[string]error
	sent          []domain.PushSubscription
}

func (s *fakePushSender) Send(_ context.Context, sub domain.PushSubscription, _ PushPayload) error {
	if err, ok := s.errByEndpoint[sub.Endpoint]; ok {
		return err
	}
	s.sent = append(s.sent, sub)
	return nil
}

// fakePushSubRepo is an in-memory PushSubscriptionRepository.
type fakePushSubRepo struct {
	subs    []domain.PushSubscription
	deleted []string
}

func (r *fakePushSubRepo) Upsert(_ context.Context, _ domain.PushSubscription) (domain.PushSubscription, error) {
	return domain.PushSubscription{}, nil
}

func (r *fakePushSubRepo) Delete(_ context.Context, _ uuid.UUID, endpoint string) error {
	r.deleted = append(r.deleted, endpoint)
	return nil
}

func (r *fakePushSubRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.PushSubscription, error) {
	var subs []domain.PushSubscription
	for _, sub := range r.subs {
		if sub.UserID == userID {
			subs = append(subs, sub)
		}
	}
	return subs, nil
}

// directHarness wires the DirectNotificationService over the fakes with both
// channels allowed, one push subscription and a resolvable email contact.
type directHarness struct {
	svc     *DirectNotificationService
	prefs   *channelPrefsRepo
	email   *captureEmailer
	push    *fakePushSender
	pushSub *fakePushSubRepo
}

func newDirectHarness(t *testing.T) *directHarness {
	t.Helper()
	h := &directHarness{
		prefs: &channelPrefsRepo{allowed: map[domain.NotificationChannel]bool{
			domain.ChannelEmail: true,
			domain.ChannelPush:  true,
		}},
		email:   &captureEmailer{},
		push:    &fakePushSender{},
		pushSub: &fakePushSubRepo{},
	}
	h.svc = NewDirectNotificationService(
		h.prefs,
		fakeResolver{contact: Contact{Channel: ChannelEmail, Email: testOwnerEmail}},
		h.email,
		h.push,
		h.pushSub,
		"https://app.example",
		nil,
	)
	return h
}

func (h *directHarness) withSubscriptions(t *testing.T, userID uuid.UUID, endpoints ...string) {
	t.Helper()
	for _, endpoint := range endpoints {
		h.pushSub.subs = append(h.pushSub.subs, domain.PushSubscription{UserID: userID, Endpoint: endpoint})
	}
}

var directRecipient = uuid.Must(uuid.NewV7())

// TestDirectNotification_BothChannelsDelivered proves the happy path: with
// both channels allowed, a subscribed user with a verified email gets the
// email and one push per device subscription (issue #253).
func TestDirectNotification_BothChannelsDelivered(t *testing.T) {
	t.Parallel()

	h := newDirectHarness(t)
	h.withSubscriptions(t, directRecipient, "https://push.example/a", "https://push.example/b")

	if err := h.svc.NotifyGraceEntered(t.Context(), directRecipient, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("NotifyGraceEntered() error = %v", err)
	}

	if len(h.email.calls) != 1 {
		t.Fatalf("emails sent = %d, want 1", len(h.email.calls))
	}
	if h.email.calls[0].to != testOwnerEmail || h.email.calls[0].template != "notification" {
		t.Errorf("email = %+v, want to owner with the notification template", h.email.calls[0])
	}
	if len(h.push.sent) != 2 {
		t.Fatalf("pushes sent = %d, want 2 (one per device)", len(h.push.sent))
	}
}

// TestDirectNotification_PerChannelPreferences proves the ADR 0030 contract:
// each channel is gated by its own preference — an email opt-out silences
// email while push still delivers, and vice versa (issue #253).
func TestDirectNotification_PerChannelPreferences(t *testing.T) {
	t.Parallel()

	pushOnly := newDirectHarness(t)
	pushOnly.prefs.allowed[domain.ChannelEmail] = false
	pushOnly.withSubscriptions(t, directRecipient, "https://push.example/a")
	if err := pushOnly.svc.NotifyGraceExpiring(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceExpiring() error = %v", err)
	}
	if len(pushOnly.email.calls) != 0 {
		t.Errorf("emails sent = %d, want 0 (email not allowed)", len(pushOnly.email.calls))
	}
	if len(pushOnly.push.sent) != 1 {
		t.Errorf("pushes sent = %d, want 1 (push still allowed)", len(pushOnly.push.sent))
	}

	emailOnly := newDirectHarness(t)
	emailOnly.prefs.allowed[domain.ChannelPush] = false
	emailOnly.withSubscriptions(t, directRecipient, "https://push.example/a")
	if err := emailOnly.svc.NotifyGraceExpiring(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceExpiring() error = %v", err)
	}
	if len(emailOnly.email.calls) != 1 {
		t.Errorf("emails sent = %d, want 1 (email still allowed)", len(emailOnly.email.calls))
	}
	if len(emailOnly.push.sent) != 0 {
		t.Errorf("pushes sent = %d, want 0 (push not allowed)", len(emailOnly.push.sent))
	}

	neither := newDirectHarness(t)
	neither.prefs.allowed[domain.ChannelEmail] = false
	neither.prefs.allowed[domain.ChannelPush] = false
	if err := neither.svc.NotifyGraceEntered(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceEntered() error = %v", err)
	}
	if len(neither.email.calls) != 0 || len(neither.push.sent) != 0 {
		t.Errorf("delivered email=%d push=%d, want nothing on a full opt-out", len(neither.email.calls), len(neither.push.sent))
	}
}

// TestDirectNotification_MissingContactAndSubscriptions proves the graceful
// skips: a user without a verified email still gets the push, and a user
// without push subscriptions still gets the email (issue #253).
func TestDirectNotification_MissingContactAndSubscriptions(t *testing.T) {
	t.Parallel()

	noContact := newDirectHarness(t)
	noContact.withSubscriptions(t, directRecipient, "https://push.example/a")
	noContact.svc = NewDirectNotificationService(
		noContact.prefs,
		fakeResolver{err: ErrNoContact},
		noContact.email,
		noContact.push,
		noContact.pushSub,
		"https://app.example",
		nil,
	)
	if err := noContact.svc.NotifyGraceEntered(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceEntered() error = %v", err)
	}
	if len(noContact.email.calls) != 0 {
		t.Errorf("emails sent = %d, want 0 (no contact)", len(noContact.email.calls))
	}
	if len(noContact.push.sent) != 1 {
		t.Errorf("pushes sent = %d, want 1 (push needs no contact)", len(noContact.push.sent))
	}

	noSubs := newDirectHarness(t)
	if err := noSubs.svc.NotifyGraceEntered(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceEntered() error = %v", err)
	}
	if len(noSubs.email.calls) != 1 {
		t.Errorf("emails sent = %d, want 1 (email needs no subscription)", len(noSubs.email.calls))
	}
}

// TestDirectNotification_ChannelFailuresAreBestEffort proves a failing channel
// never fails the other and never surfaces an error: a dead push subscription
// is deleted, a rate-limited push stops the device fan-out, and a failing
// email send leaves push delivered (issue #253).
func TestDirectNotification_ChannelFailuresAreBestEffort(t *testing.T) {
	t.Parallel()

	gone := newDirectHarness(t)
	gone.withSubscriptions(t, directRecipient, "https://push.example/dead")
	gone.push.errByEndpoint = map[string]error{"https://push.example/dead": ErrSubscriptionGone}
	if err := gone.svc.NotifyGraceEntered(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceEntered() error = %v", err)
	}
	if len(gone.pushSub.deleted) != 1 || gone.pushSub.deleted[0] != "https://push.example/dead" {
		t.Errorf("deleted subscriptions = %v, want the gone endpoint", gone.pushSub.deleted)
	}
	if len(gone.email.calls) != 1 {
		t.Errorf("emails sent = %d, want 1 (email unaffected by push failures)", len(gone.email.calls))
	}

	rateLimited := newDirectHarness(t)
	rateLimited.withSubscriptions(t, directRecipient, "https://push.example/a", "https://push.example/b")
	rateLimited.push.errByEndpoint = map[string]error{"https://push.example/a": ErrRateLimited}
	if err := rateLimited.svc.NotifyGraceExpiring(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceExpiring() error = %v", err)
	}
	if len(rateLimited.push.sent) != 0 {
		t.Errorf("pushes sent = %d, want 0 (rate limit stops the fan-out)", len(rateLimited.push.sent))
	}
	if len(rateLimited.email.calls) != 1 {
		t.Errorf("emails sent = %d, want 1 (email unaffected)", len(rateLimited.email.calls))
	}

	emailFails := newDirectHarness(t)
	emailFails.email.err = errors.New("smtp down")
	emailFails.withSubscriptions(t, directRecipient, "https://push.example/a")
	if err := emailFails.svc.NotifyGraceEntered(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceEntered() error = %v (an email failure must not surface)", err)
	}
	if len(emailFails.push.sent) != 1 {
		t.Errorf("pushes sent = %d, want 1 (push unaffected by the email failure)", len(emailFails.push.sent))
	}
}

// TestDirectNotification_NilPushSenderIsEmailOnly proves the nil-sender guard:
// without a wired push sender (local dev without VAPID keys) delivery is
// email-only and never panics (issue #253).
func TestDirectNotification_NilPushSenderIsEmailOnly(t *testing.T) {
	t.Parallel()

	h := newDirectHarness(t)
	h.withSubscriptions(t, directRecipient, "https://push.example/a")
	h.svc = NewDirectNotificationService(
		h.prefs,
		fakeResolver{contact: Contact{Channel: ChannelEmail, Email: testOwnerEmail}},
		h.email,
		nil,
		h.pushSub,
		"https://app.example",
		nil,
	)
	if err := h.svc.NotifyGraceExpiring(t.Context(), directRecipient, time.Time{}); err != nil {
		t.Fatalf("NotifyGraceExpiring() error = %v", err)
	}
	if len(h.email.calls) != 1 {
		t.Errorf("emails sent = %d, want 1", len(h.email.calls))
	}
}
