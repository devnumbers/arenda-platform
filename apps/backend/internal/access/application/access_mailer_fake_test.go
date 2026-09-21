package application

import (
	"context"
	"slices"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
)

// fakeAccessMailer records the direct emails that remain at the AccessMailer
// port (the invite email, the object-deleted notice): the lifecycle
// correspondence itself left for the notifications feed (карта #734, #751),
// so the fake shrank with the port.
type fakeAccessMailer struct {
	sent []sentMail
	err  error
}

// sentMail records one email passed through the AccessMailer port. Kind is the
// template name (invite, property_deleted).
type sentMail struct {
	kind  string
	to    string
	title string
	role  domain.Role
}

func (f *fakeAccessMailer) record(m sentMail) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeAccessMailer) SendInvite(_ context.Context, to, title string, role domain.Role) error {
	return f.record(sentMail{kind: "invite", to: to, title: title, role: role})
}

func (f *fakeAccessMailer) SendPropertyDeleted(_ context.Context, to, title string) error {
	return f.record(sentMail{kind: "property_deleted", to: to, title: title})
}

var _ AccessMailer = (*fakeAccessMailer)(nil)

// fakeEventPublisher records the lifecycle events the services publish
// (карта #734, #751); errFor fails a kind on demand — the publications are
// best-effort, so a failure must never surface from the transition itself.
type fakeEventPublisher struct {
	events []recordedEvent
	errFor map[string]error
}

type recordedEvent struct {
	kind  string
	email *InvitationActivated
	pause *MembershipSuspended
	resum *MembershipResumed
	revo  *MembershipRevoked
	left  *MemberLeft
}

func (f *fakeEventPublisher) PublishInvitationActivated(_ context.Context, e InvitationActivated) error {
	if err := f.errFor["invitation_activated"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "invitation_activated", email: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipSuspended(_ context.Context, e MembershipSuspended) error {
	if err := f.errFor["membership_suspended"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "membership_suspended", pause: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipResumed(_ context.Context, e MembershipResumed) error {
	if err := f.errFor["membership_resumed"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "membership_resumed", resum: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipRevoked(_ context.Context, e MembershipRevoked) error {
	if err := f.errFor["membership_revoked"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "membership_revoked", revo: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMemberLeft(_ context.Context, e MemberLeft) error {
	if err := f.errFor["member_left"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "member_left", left: &e})
	return nil
}

var _ AccessEventPublisher = (*fakeEventPublisher)(nil)

// count returns how many events of the given kind were published.
func (f *fakeEventPublisher) count(kind string) int {
	n := 0
	for _, e := range f.events {
		if e.kind == kind {
			n++
		}
	}
	return n
}

// last returns the last recorded event of the given kind, failing the test
// otherwise.
func (f *fakeEventPublisher) last(t *testing.T, kind string) recordedEvent {
	t.Helper()
	if f.count(kind) == 0 {
		t.Fatalf("expected at least one %s event, got none (all: %+v)", kind, f.events)
	}
	for _, v := range slices.Backward(f.events) {
		if v.kind == kind {
			return v
		}
	}
	panic("unreachable")
}
