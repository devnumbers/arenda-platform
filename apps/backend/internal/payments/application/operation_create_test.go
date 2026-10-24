package application

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// The manual operation's validator at its own seam (ticket #569), mirroring
// the payment_rule_test.go style: the rule's create vocabulary minus the
// rule-only parts — a manual fact carries no recurrence and no payment form.
// The client-chosen date (backdating, ticket #1392) is checked against the
// owner's today on the same seam: the future is the domain's own sentinel.

// validateToday is the owner's today the validator cases share: the UTC
// midnight shape the conveyor hands the mutation (ADR 0048).
var validateToday = time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)

// validManualOperation is the base fixture every case mutates: a minimal
// manual fact that passes validateManualOperation — born paid on the owner's
// today like the conveyor's draft.
func validManualOperation() domain.Operation {
	slug := "utilities"
	return domain.Operation{
		Type:          domain.TypeExpense,
		Title:         "ЖКУ",
		AmountKopecks: 500000,
		Origin:        domain.OriginManual,
		Status:        domain.StatusPaid,
		Date:          validateToday,
		CategorySlug:  &slug,
	}
}

func TestValidateManualOperation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(op *domain.Operation)
		wantErr bool
	}{
		{"valid fact as-is", nil, false},
		{"bad type enum", func(op *domain.Operation) { op.Type = domain.PaymentType("profit") }, true},
		{"blank title", func(op *domain.Operation) { op.Title = "   " }, true},
		{"title over 255 characters", func(op *domain.Operation) { op.Title = strings.Repeat("а", 256) }, true},
		{
			// The limit counts characters, not bytes — 130 Cyrillic characters
			// are 260 bytes and a valid title.
			"130-character Cyrillic title is valid", func(op *domain.Operation) { op.Title = strings.Repeat("а", 130) },
			false,
		},
		{"zero amount", func(op *domain.Operation) { op.AmountKopecks = 0 }, true},
		// Потолка сумм больше нет (решение владельца 23.10.2026): за прежним
		// потолком 10⁹ — произвольные суммы; минимум остаётся.
		{"amount over 10^9", func(op *domain.Operation) { op.AmountKopecks = 1_000_000_001 }, false},
		{"huge amount", func(op *domain.Operation) { op.AmountKopecks = 9_000_000_000_000_000 }, false},
		{"negative amount", func(op *domain.Operation) { op.AmountKopecks = -5 }, true},
		{"unknown category slug", func(op *domain.Operation) { op.CategorySlug = new("not-a-catalog-slug") }, true},
		{"nil category slug", func(op *domain.Operation) { op.CategorySlug = nil }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op := validManualOperation()
			if tc.mutate != nil {
				tc.mutate(&op)
			}
			err := validateManualOperation(op, validateToday)
			if tc.wantErr && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

// The client-chosen date (backdating, ticket #1392): the past is unlimited —
// no lower floor — and the boundary is the owner's today, inclusive; only the
// strictly-after day is the future, and it speaks with its own sentinel so
// the wire can answer «Дата операции не может быть в будущем».
func TestValidateManualOperation_ClientDate(t *testing.T) {
	t.Parallel()

	yesterday := validateToday.AddDate(0, 0, -1)
	tomorrow := validateToday.AddDate(0, 0, 1)
	yearAhead := validateToday.AddDate(1, 0, 0)

	t.Run("today itself is valid", func(t *testing.T) {
		t.Parallel()
		if err := validateManualOperation(validManualOperation(), validateToday); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
	t.Run("a year in the past is valid — no lower bound", func(t *testing.T) {
		t.Parallel()
		op := validManualOperation()
		op.Date = yearAhead.AddDate(-2, 0, 0)
		if err := validateManualOperation(op, validateToday); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
	t.Run("tomorrow is the future sentinel", func(t *testing.T) {
		t.Parallel()
		op := validManualOperation()
		op.Date = tomorrow
		if err := validateManualOperation(op, validateToday); !errors.Is(err, ErrOperationDateInFuture) {
			t.Errorf("err = %v, want ErrOperationDateInFuture", err)
		}
	})
	t.Run("a year ahead is the future sentinel", func(t *testing.T) {
		t.Parallel()
		op := validManualOperation()
		op.Date = yearAhead
		if err := validateManualOperation(op, validateToday); !errors.Is(err, ErrOperationDateInFuture) {
			t.Errorf("err = %v, want ErrOperationDateInFuture", err)
		}
	})
	t.Run("yesterday passes the date check", func(t *testing.T) {
		t.Parallel()
		op := validManualOperation()
		op.Date = yesterday
		if err := validateManualOperation(op, validateToday); err != nil {
			t.Errorf("err = %v, want nil — backdating is the point", err)
		}
	})
}

// A user-category reference is not part of the manual creation contract yet:
// the command carries the default-catalog slug only, and a CategoryRef built
// around a user id would fail — the slug is the only resolvable reference.
func TestValidateManualOperation_UserCategorySlugOnly(t *testing.T) {
	t.Parallel()
	op := validManualOperation()
	op.CategorySlug = new(uuid.Must(uuid.NewV7()).String())
	if err := validateManualOperation(op, validateToday); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput — a uuid is not a catalog slug", err)
	}
}
