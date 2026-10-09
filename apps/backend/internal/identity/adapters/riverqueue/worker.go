package riverqueue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/riverqueue/river"
)

// Compile-time check that the worker satisfies River's worker contract.
var _ river.Worker[ContactChangedArgs] = (*ContactChangedWorker)(nil)

// Subjects of the two change letters. Like the code letters' subjects
// (issue #1204), the subject names what happened — and, being a
// notification-only letter, carries no code and no link to act through
// (research #1201 §5).
const (
	phoneChangeDoneSubject = "Телефон в Рентли изменён"
	emailChangeDoneSubject = "Email в Рентли изменён"
)

// changeLetter is the presentation shape of one change letter: the template
// rendered through the shared layout and the fact-naming subject.
type changeLetter struct {
	template string
	subject  string
}

// letterFor resolves the unique letter for the change kind. A kind without a
// letter is a programming error and cancels the job loudly.
func letterFor(kind application.ContactChangeKind) (changeLetter, error) {
	switch kind {
	case application.ContactChangedPhone:
		return changeLetter{template: "phone_change_done", subject: phoneChangeDoneSubject}, nil
	case application.ContactChangedEmail:
		return changeLetter{template: "email_change_done", subject: emailChangeDoneSubject}, nil
	default:
		return changeLetter{}, fmt.Errorf("no letter for contact change %q", kind)
	}
}

// changedAtMonths are the Russian month names in the genitive case, shared by
// both letters' change instants ("7 октября").
var changedAtMonths = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// moscowZone is the platform's default user zone (users.timezone default);
// the approved letter copy names it with the Russian abbreviation.
const moscowZone = "Europe/Moscow"

// formatChangedAt renders the change instant in the user's timezone as a
// Russian phrase — day, genitive month, year, clock time and a parenthesised
// zone label: the platform's default zone renders its abbreviation «МСК»,
// every other zone its UTC offset, taken from the instant itself so any IANA
// zone is correct without a backend copy of the frontend's zone dictionary
// (карта #591); non-whole-hour offsets keep their minutes ("UTC+3:30"). The
// exact phrasing is pinned by the tests and by the letter-content contract.
func formatChangedAt(t time.Time, timezone domain.Timezone) (string, error) {
	loc, err := time.LoadLocation(timezone.String())
	if err != nil {
		return "", fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	local := t.In(loc)
	label := formatUTCOffset(local)
	if timezone.String() == moscowZone {
		label = "МСК"
	}
	return renderChangedAt(local, label), nil
}

// renderChangedAt is the single formatting of a change instant: the local
// wall clock plus a zone label in parentheses.
func renderChangedAt(local time.Time, label string) string {
	return fmt.Sprintf("%d %s %d г. в %02d:%02d (%s)",
		local.Day(), changedAtMonths[int(local.Month())-1], local.Year(),
		local.Hour(), local.Minute(), label)
}

// formatUTCOffset renders the zone offset the Russian way: "UTC" for the
// zero offset, "UTC+3" for whole hours, "UTC+3:30" otherwise.
func formatUTCOffset(t time.Time) string {
	_, sec := t.Zone()
	if sec == 0 {
		return "UTC"
	}
	sign, abs := "+", sec
	if sec < 0 {
		sign, abs = "-", -sec
	}
	h, m := abs/3600, (abs%3600)/60
	if m == 0 {
		return fmt.Sprintf("UTC%s%d", sign, h)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, h, m)
}

// userSource reloads the changed user for the letter's timezone
// (consumer-declared port; the identity user repository satisfies it).
type userSource interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// renderer is the consumer-side port for email template rendering (ADR 0035),
// the same seam the identity code-letter sender uses.
type renderer interface {
	Render(name string, data any) (plain, html string, err error)
}

// ContactChangedWorker delivers one contact-change letter: it reloads the
// user for the timezone, renders the letter, and sends it to the recipient
// captured at change time. Retry classification: a vanished user or a kind
// without a letter cancels the job — there is nothing to retry; database and
// SMTP failures return the error and follow River's backoff ladder.
type ContactChangedWorker struct {
	river.WorkerDefaults[ContactChangedArgs]
	users    userSource
	sender   mailer.Sender
	renderer renderer
	log      *slog.Logger
}

// NewContactChangedWorker builds the change-letter worker.
func NewContactChangedWorker(users userSource, sender mailer.Sender, renderer renderer, log *slog.Logger) *ContactChangedWorker {
	if log == nil {
		log = slog.Default()
	}
	return &ContactChangedWorker{users: users, sender: sender, renderer: renderer, log: log}
}

// Work renders and sends one change letter.
func (w *ContactChangedWorker) Work(ctx context.Context, job *river.Job[ContactChangedArgs]) error {
	kind := application.ContactChangeKind(job.Args.ChangeKind)
	letter, err := letterFor(kind)
	if err != nil {
		return river.JobCancel(err)
	}

	user, err := w.users.GetByID(ctx, job.Args.UserID)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			w.log.WarnContext(ctx, "contact-change letter skipped: user gone",
				slog.String("user_id", job.Args.UserID.String()))
			return river.JobCancel(fmt.Errorf("user %s not found", job.Args.UserID))
		}
		return fmt.Errorf("load user: %w", err)
	}

	when, err := formatChangedAt(job.Args.ChangedAt, user.Timezone)
	if err != nil {
		// The stored zone passed validation at write time, so this is
		// defensive: a slightly off clock never costs the letter.
		w.log.WarnContext(ctx, "contact-change letter falls back to UTC",
			slog.String("user_id", job.Args.UserID.String()),
			slog.String("error", err.Error()))
		when = fallbackChangedAt(job.Args.ChangedAt)
	}

	plain, html, err := w.renderer.Render(letter.template, map[string]any{"When": when})
	if err != nil {
		return fmt.Errorf("render %s email: %w", letter.template, err)
	}

	msg := mailer.Message{
		To:       []string{job.Args.Recipient},
		Subject:  letter.subject,
		TextBody: plain,
		HTMLBody: html,
	}
	if err := w.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send %s email: %w", letter.template, err)
	}
	return nil
}

// fallbackChangedAt renders the instant in UTC for the defensive path.
func fallbackChangedAt(t time.Time) string {
	local := t.UTC()
	return renderChangedAt(local, formatUTCOffset(local))
}
