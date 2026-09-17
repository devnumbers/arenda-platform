package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
)

// Fakes for the invite email path, shared by the invitation and participant
// mutation tests. The sharing lifecycle emails (issue #162, T6) are cut
// (issue #695): the invite email is the only email of the access context.

// sentMail records one email passed through the AccessMailer port.
type sentMail struct {
	kind   string
	to     string
	titles []string
	role   domain.Role
}

// fakeAccessMailer records sent invite emails; err simulates a send failure.
type fakeAccessMailer struct {
	sent []sentMail
	err  error
}

func (f *fakeAccessMailer) record(m sentMail) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeAccessMailer) SendInvite(_ context.Context, to string, titles []string, role domain.Role) error {
	return f.record(sentMail{kind: "invite", to: to, titles: titles, role: role})
}

// count returns how many emails of the given kind were sent.
func (f *fakeAccessMailer) count(kind string) int {
	n := 0
	for _, m := range f.sent {
		if m.kind == kind {
			n++
		}
	}
	return n
}

// only returns the single email of the given kind, failing the test otherwise.
func (f *fakeAccessMailer) only(t *testing.T, kind string) sentMail {
	t.Helper()
	if got := f.count(kind); got != 1 {
		t.Fatalf("expected exactly one %s email, got %d (all: %+v)", kind, got, f.sent)
	}
	for _, m := range f.sent {
		if m.kind == kind {
			return m
		}
	}
	panic("unreachable")
}

// fakeEmailResolver is an in-memory UserEmailResolver keyed by user id; an
// absent user reads as "no email" and the send is skipped.
type fakeEmailResolver map[uuid.UUID]string

func (f fakeEmailResolver) GetEmail(_ context.Context, id uuid.UUID) (string, error) {
	return f[id], nil
}

// fakePropertyTitles resolves per-property titles for the invite email text.
type fakePropertyTitles map[uuid.UUID]string

func (f fakePropertyTitles) GetTitle(_ context.Context, id uuid.UUID) (string, error) {
	if title, ok := f[id]; ok {
		return title, nil
	}
	return "", domain.ErrMemberNotFound
}

var (
	_ AccessMailer          = (*fakeAccessMailer)(nil)
	_ UserEmailResolver     = fakeEmailResolver(nil)
	_ PropertyTitleResolver = fakePropertyTitles(nil)
)
