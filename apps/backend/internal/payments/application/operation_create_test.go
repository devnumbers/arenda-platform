package application

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// The manual operation's validator at its own seam (ticket #569), mirroring
// the payment_rule_test.go style: the rule's create vocabulary minus the
// rule-only parts — a manual fact carries no recurrence and no payment form.

// validManualOperation is the base fixture every case mutates: a minimal
// manual fact that passes validateManualOperation.
func validManualOperation() domain.Operation {
	slug := "utilities"
	return domain.Operation{
		Type:          domain.TypeExpense,
		Title:         "ЖКУ",
		AmountKopecks: 500000,
		Origin:        domain.OriginManual,
		Status:        domain.StatusPaid,
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
		{"amount over 10^9", func(op *domain.Operation) { op.AmountKopecks = 1_000_000_001 }, true},
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
			err := validateManualOperation(op)
			if tc.wantErr && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

// A user-category reference is not part of the manual creation contract yet:
// the command carries the default-catalog slug only, and a CategoryRef built
// around a user id would fail — the slug is the only resolvable reference.
func TestValidateManualOperation_UserCategorySlugOnly(t *testing.T) {
	t.Parallel()
	op := validManualOperation()
	op.CategorySlug = new(uuid.Must(uuid.NewV7()).String())
	if err := validateManualOperation(op); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput — a uuid is not a catalog slug", err)
	}
}
