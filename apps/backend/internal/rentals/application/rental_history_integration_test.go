//go:build integration

package application_test

// The fail-safe atomicity of the rentals conveyor over the action journal
// (карта #704, тикет #707, ADR 0061): a failing journal insert rolls the
// whole rental back — the rental row, its managed payment and the journal
// row share the transaction's fate.

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	rentalspg "github.com/nambers/arenda-planform/apps/backend/internal/rentals/adapters/postgres"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// failingJournal is the recorder double whose every insert fails.
type failingJournal struct{}

var errJournalDown = errors.New("journal store down")

func (failingJournal) Record(context.Context, historydomain.Entry) error { return errJournalDown }

func (failingJournal) WithTx(transaction.Tx) historyapp.Recorder { return failingJournal{} }

func TestHistory_FailingJournalRollsRentalBack(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	gateway := &fakeSeamGateway{pool: pool, day: rentalsdomain.MustPaymentDay(15)}
	factory := rentalsapp.NewTxStoreFactory(
		rentalspg.NewRentalStore(pool),
		paymentspg.NewPropertyStore(pool),
		gateway,
		nil,
		auditapp.NewService(auditpg.NewWriter(pool), nil),
		failingJournal{},
		pgdb.NewUoW(pool, slog.New(slog.DiscardHandler)),
	)
	h := &rentalsHarness{
		t: t, pool: pool, gateway: gateway,
		svc: rentalsapp.NewRentalService(factory, intClock{}, intPolicy{role: sharedpolicy.RoleOwner}),
	}
	h.seedOwner()

	_, err := h.svc.CreateRental(context.Background(), h.owner, h.propID, h.createCmd())
	if !errors.Is(err, errJournalDown) {
		t.Fatalf("CreateRental error = %v, want errJournalDown", err)
	}

	var rentals, payments, journal int
	if err := pool.QueryRow(context.Background(),
		`SELECT
		    (SELECT count(*) FROM rentals WHERE property_id = $1),
		    (SELECT count(*) FROM payments WHERE property_id = $1),
		    (SELECT count(*) FROM action_journal WHERE property_id = $1)`,
		h.propID,
	).Scan(&rentals, &payments, &journal); err != nil {
		t.Fatalf("counts: %v", err)
	}
	if rentals != 0 || payments != 0 || journal != 0 {
		t.Errorf("rows after the failed create: rentals=%d payments=%d journal=%d, want 0/0/0 (rolled back)",
			rentals, payments, journal)
	}
}
