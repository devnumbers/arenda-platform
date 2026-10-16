package riverqueue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/riverqueue/river"
)

// letterRecipient is the tests' fixed change-time recipient address.
const letterRecipient = "owner@example.com"

// fakeUserSource serves one user (or a lookup error) to the worker.
type fakeUserSource struct {
	user domain.User
	err  error
}

func (f *fakeUserSource) GetByID(context.Context, uuid.UUID) (domain.User, error) {
	return f.user, f.err
}

// captureSender records every message handed to the mailer.
type captureSender struct {
	messages []mailer.Message
	err      error
}

func (s *captureSender) Send(_ context.Context, msg mailer.Message) error {
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, msg)
	return nil
}

func TestLetterFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind         application.ContactChangeKind
		wantTemplate string
		wantSubject  string
	}{
		{application.ContactChangedPhone, "phone_change_done", "Телефон в Рентли изменён"},
		{application.ContactChangedEmail, "email_change_done", "Email в Рентли изменён"},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()
			letter, err := letterFor(tc.kind)
			if err != nil {
				t.Fatalf("letterFor: %v", err)
			}
			if letter.template != tc.wantTemplate || letter.subject != tc.wantSubject {
				t.Fatalf("letter = (%q, %q), want (%q, %q)",
					letter.template, letter.subject, tc.wantTemplate, tc.wantSubject)
			}
		})
	}

	t.Run("unknown kind is a programming error", func(t *testing.T) {
		t.Parallel()
		if _, err := letterFor(application.ContactChangeKind("password_changed")); err == nil {
			t.Fatal("letterFor unknown kind: nil error, want one")
		}
	})
}

func TestFormatChangedAt(t *testing.T) {
	t.Parallel()

	// 2026-10-07 11:30 UTC == 14:30 in Moscow.
	instant := time.Date(2026, 10, 7, 11, 30, 0, 0, time.UTC)

	t.Run("moscow renders the approved abbreviation", func(t *testing.T) {
		t.Parallel()
		msk, err := domain.NewTimezone("Europe/Moscow")
		if err != nil {
			t.Fatalf("timezone: %v", err)
		}
		got, err := formatChangedAt(instant, msk)
		if err != nil {
			t.Fatalf("formatChangedAt: %v", err)
		}
		want := "7 октября 2026 г. в 14:30 (МСК)"
		if got != want {
			t.Fatalf("changed-at = %q, want %q", got, want)
		}
	})

	t.Run("half-hour offset keeps its minutes", func(t *testing.T) {
		t.Parallel()
		kolkata, err := domain.NewTimezone("Asia/Kolkata")
		if err != nil {
			t.Fatalf("timezone: %v", err)
		}
		got, err := formatChangedAt(instant, kolkata)
		if err != nil {
			t.Fatalf("formatChangedAt: %v", err)
		}
		want := "7 октября 2026 г. в 17:00 (UTC+5:30)"
		if got != want {
			t.Fatalf("changed-at = %q, want %q", got, want)
		}
	})

	t.Run("empty zone renders UTC (the defensive path)", func(t *testing.T) {
		t.Parallel()
		got, err := formatChangedAt(instant, domain.Timezone{})
		if err != nil {
			t.Fatalf("formatChangedAt: %v", err)
		}
		want := "7 октября 2026 г. в 11:30 (UTC)"
		if got != want {
			t.Fatalf("changed-at = %q, want %q", got, want)
		}
	})
}

// letterTestEnv bundles the pieces one worker test needs: a user in Moscow,
// a real renderer over the templates dir, a capturing sender, and the worker.
type letterTestEnv struct {
	users     *fakeUserSource
	sender    *captureSender
	worker    *ContactChangedWorker
	changedAt time.Time
}

func newLetterTestEnv(t *testing.T) *letterTestEnv {
	t.Helper()
	renderer, err := mailer.NewRenderer("../../../../templates/email")
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	user, err := domain.NewOwner(mustPhone(t))
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	tz, err := domain.NewTimezone("Europe/Moscow")
	if err != nil {
		t.Fatalf("timezone: %v", err)
	}
	user.Timezone = tz
	users := &fakeUserSource{user: user}
	sender := &captureSender{}
	// 2026-10-07 11:30 UTC == 14:30 Moscow.
	return &letterTestEnv{
		users:     users,
		sender:    sender,
		worker:    NewContactChangedWorker(users, sender, renderer, nil),
		changedAt: time.Date(2026, 10, 7, 11, 30, 0, 0, time.UTC),
	}
}

// assertSingleLetter checks the delivered message: one letter to the
// change-time recipient with the wanted subject and body blocks.
func assertSingleLetter(t *testing.T, sender *captureSender, recipient, subject string, bodyBlocks []string) {
	t.Helper()
	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}
	msg := sender.messages[0]
	if len(msg.To) != 1 || msg.To[0] != recipient {
		t.Fatalf("to = %v, want the change-time recipient %s", msg.To, recipient)
	}
	if msg.Subject != subject {
		t.Fatalf("subject = %q, want the fact-naming subject %q", msg.Subject, subject)
	}
	for _, want := range bodyBlocks {
		if !strings.Contains(msg.TextBody, want) {
			t.Fatalf("text body %q misses block %q", msg.TextBody, want)
		}
	}
	if msg.HTMLBody == "" {
		t.Fatal("html body is empty")
	}
}

func TestContactChangedWorker_PhoneLetterCarriesRequiredBlocks(t *testing.T) {
	t.Parallel()
	env := newLetterTestEnv(t)

	err := env.worker.Work(context.Background(), &river.Job[ContactChangedArgs]{
		Args: ContactChangedArgs{
			ChangeKind: string(application.ContactChangedPhone),
			UserID:     env.users.user.ID,
			Recipient:  letterRecipient,
			ChangedAt:  env.changedAt,
		},
	})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}
	assertSingleLetter(t, env.sender, letterRecipient, "Телефон в Рентли изменён", []string{
		"Телефон для входа в ваш аккаунт Рентли изменён 7 октября 2026 г. в 14:30 (МСК)",
		"Если это были вы — ничего делать не нужно.",
		"Если это были не вы: войти по прежнему номеру больше нельзя.",
		"hello@rentlee.ru",
	})
}

func TestContactChangedWorker_EmailLetterCarriesRequiredBlocks(t *testing.T) {
	t.Parallel()
	env := newLetterTestEnv(t)

	err := env.worker.Work(context.Background(), &river.Job[ContactChangedArgs]{
		Args: ContactChangedArgs{
			ChangeKind: string(application.ContactChangedEmail),
			UserID:     env.users.user.ID,
			Recipient:  "old@example.com",
			ChangedAt:  env.changedAt,
		},
	})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}
	assertSingleLetter(t, env.sender, "old@example.com", "Email в Рентли изменён", []string{
		"Email вашего аккаунта Рентли изменён 7 октября 2026 г. в 14:30 (МСК)",
		"Если это были вы — ничего делать не нужно.",
		"Если это были не вы: войдите в аккаунт и верните прежний адрес в настройках профиля.",
		"hello@rentlee.ru",
	})
}

func TestContactChangedWorker_UnknownKindCancelsJob(t *testing.T) {
	t.Parallel()
	env := newLetterTestEnv(t)

	err := env.worker.Work(context.Background(), &river.Job[ContactChangedArgs]{
		Args: ContactChangedArgs{ChangeKind: "password_changed", Recipient: letterRecipient},
	})
	var cancelErr *river.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Fatalf("error = %v, want a river.JobCancel", err)
	}
	if len(env.sender.messages) != 0 {
		t.Fatal("a letter went out for an unknown kind")
	}
}

func TestContactChangedWorker_VanishedUserCancelsJob(t *testing.T) {
	t.Parallel()
	env := newLetterTestEnv(t)
	env.users.err = application.ErrNotFound

	err := env.worker.Work(context.Background(), &river.Job[ContactChangedArgs]{
		Args: ContactChangedArgs{
			ChangeKind: string(application.ContactChangedPhone),
			UserID:     uuid.Must(uuid.NewV7()),
			Recipient:  letterRecipient,
			ChangedAt:  env.changedAt,
		},
	})
	var cancelErr *river.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Fatalf("error = %v, want a river.JobCancel", err)
	}
	if len(env.sender.messages) != 0 {
		t.Fatal("a letter went out for a vanished user")
	}
}

func TestContactChangedWorker_SMTPFailureReturnsErrorForRetry(t *testing.T) {
	t.Parallel()
	env := newLetterTestEnv(t)
	sendErr := errors.New("smtp down")
	env.sender.err = sendErr

	err := env.worker.Work(context.Background(), &river.Job[ContactChangedArgs]{
		Args: ContactChangedArgs{
			ChangeKind: string(application.ContactChangedPhone),
			UserID:     env.users.user.ID,
			Recipient:  letterRecipient,
			ChangedAt:  env.changedAt,
		},
	})
	if !errors.Is(err, sendErr) {
		t.Fatalf("error = %v, want the smtp failure wrapped for retry", err)
	}
}

// mustPhone parses the tests' fixed owner phone.
func mustPhone(t *testing.T) domain.Phone {
	t.Helper()
	phone, err := domain.NewPhone("+79160000700")
	if err != nil {
		t.Fatalf("parse phone: %v", err)
	}
	return phone
}
