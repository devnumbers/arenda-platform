package application

// The «На оплату» section's read side (ticket #1346): the rules with an
// enabled reminder (reminder_offset_days 1/3/7) and no auto-pay — the
// owner's decision — whose unpaid planned operation with date D stands in
// the reminder window D−N ≤ today ≤ D (today — the property owner's
// calendar, ADR 0048). Overdue operations (D < today) are not here — they
// live in «Все просроченные». The section is a derivation on read (the
// research's option C): no second state machine, the window follows the
// stored operations for free — an open pause generates no occurrences, a
// timezone shift moves the boundaries, the tick's materialization keeps at
// most today's and one future operation per rule in the window.

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// PaymentDueItem is one row of the «На оплату» section (ticket #1346): the
// (rule, operation) pair in the reminder window — the section page's card
// data. The bucket is the client's rendering of DaysLeft (the server builds
// no labels): 0 → «Сегодня», 1 → «Завтра», 2..3 → «Через 3 дня», 4..7 →
// «Через 7 дней».
type PaymentDueItem struct {
	// ID is the payment rule's id — the card's link target (the payment
	// screen), not the operation's own page.
	ID uuid.UUID
	// OperationID is the window operation's id — the row's identity (a rule
	// can carry two window rows: today's and the next future one).
	OperationID uuid.UUID
	PropertyID  uuid.UUID
	// PropertyName is the bound property's display name — the section card's
	// object label.
	PropertyName  string
	Type          domain.PaymentType
	Title         string
	AmountKopecks int64
	Category      domain.CategoryRef
	// Today is the property owner's calendar date (ADR 0048) the window math
	// ran against — the client renders the bucket labels against DaysLeft,
	// the date travels for the display.
	Today   time.Time
	DueDate time.Time
	// DaysLeft is DueDate − Today (0..7 by the window's construction) — the
	// bucket key and the hub card's «Через N дней» source.
	DaysLeft int
	// ReminderOffsetDays is the rule's lead time (1/3/7) — the window's own
	// N.
	ReminderOffsetDays int
}

// PaymentDueList is the «На оплату» section's response (ticket #1346): the
// whole window in one read — the section is small by construction (the
// materialization keeps at most two window operations per rule, the
// reminder is armed on a fraction of the rules) — ordered by (bucket,
// dueDate desc, id desc): the buckets in their fixed order (0, 1, 2..3,
// 4..7), «Сначала новые» inside one. DueOperationsCount is the rows' own
// count — the hub card's number, the same figure the feed's counters carry.
type PaymentDueList struct {
	Items              []PaymentDueItem
	DueOperationsCount int64
}

// ListPaymentDue returns the actor's visible «На оплату» section (ticket
// #1346): the window rows of the own book plus active-membership
// properties, archived excluded, completed rules excluded (the feed's
// inherited gate), auto-pay rules excluded (the owner's decision — the
// reminder may fire for them, the section does not contain them). The
// window predicate and the ordering live in the store's SQL; the reads
// never tick and publish no realtime frames (D18) — the section re-reads.
func (s *GlobalPaymentService) ListPaymentDue(ctx context.Context, actor uuid.UUID) (PaymentDueList, error) {
	todays, err := s.ownerTodays(ctx, actor)
	if err != nil {
		return PaymentDueList{}, err
	}
	if len(todays) == 0 {
		return PaymentDueList{Items: []PaymentDueItem{}}, nil
	}
	rows, err := s.reader.ListPaymentDue(ctx, actor, todays)
	if err != nil {
		return PaymentDueList{}, fmt.Errorf("list payment due: %w", err)
	}
	return PaymentDueList{
		Items:              rows,
		DueOperationsCount: int64(len(rows)),
	}, nil
}
