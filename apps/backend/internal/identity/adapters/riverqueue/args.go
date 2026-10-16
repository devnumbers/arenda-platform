// Package riverqueue is the delivery-queue adapter of the identity context
// for the contact-change security letters (решение #1207): the transactional
// enqueuer the change services schedule letters through, and the River worker
// that renders and sends them. Jobs carry the full fact of the change —
// kind, user, recipient, instant — because the letter must be deliverable
// from its args alone: for an email change the old address is gone from the
// user row the moment the change commits.
package riverqueue

import (
	"time"

	"github.com/google/uuid"
)

// QueueContactChanged carries the change letters. A separate queue keeps the
// security letters from starving behind the notifications feed's email queue,
// which owns its own SMTP ceiling.
const QueueContactChanged = "identity_contact_changed"

// contactChangedMaxAttempts budgets the River retries: a letter is worth a
// backoff ladder, not an infinite one — the account owner who made the change
// does not need it, only a hijack victim does, and five attempts over River's
// ladder is enough for any transient SMTP outage. A fixed domain decision, no
// env knob (the scan cadence canon).
const contactChangedMaxAttempts = 5

// ContactChangedArgs delivers one contact-change letter. Recipient and
// instant are captured at change time: for an email change the recipient is
// the OLD address, and ChangedAt is what the letter announces.
type ContactChangedArgs struct {
	// ChangeKind is the application.ContactChangeKind value ("email_changed"
	// or "phone_changed") — the letter's template and subject selector.
	// Named not Kind: the river.JobArgs interface claims Kind() for the job
	// kind itself.
	ChangeKind string    `json:"change_kind"`
	UserID     uuid.UUID `json:"user_id"`
	Recipient  string    `json:"recipient"`
	ChangedAt  time.Time `json:"changed_at"`
}

// Kind identifies the job kind to River.
func (ContactChangedArgs) Kind() string { return "identity:contact_changed" }
