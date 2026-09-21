package database

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	scanErr error
}

// testPingQuery is the SQL fixture shared by the instrumented-row tests.
const testPingQuery = "SELECT 1"

func (r *fakeRow) Scan(dest ...any) error { return r.scanErr }

func TestInstrumentedRowScanReleasesConnection(t *testing.T) {
	t.Parallel()

	var releases atomic.Int32
	row := &instrumentedRow{
		row:        &fakeRow{},
		inst:       newDBInstrumenter(slog.New(slog.NewTextHandler(os.Stdout, nil))),
		sql:        testPingQuery,
		queryStart: time.Now(),
		release:    func() { releases.Add(1) },
	}

	if err := row.Scan(new(int)); err != nil {
		t.Fatalf("Scan unexpected error: %v", err)
	}
	if got := releases.Load(); got != 1 {
		t.Fatalf("release called %d times, want 1", got)
	}

	// Subsequent Close/Scan calls must be idempotent.
	row.Close()
	if err := row.Scan(new(int)); err != nil {
		t.Errorf("idempotent Scan error: %v", err)
	}
	if got := releases.Load(); got != 1 {
		t.Fatalf("release called %d times after idempotent calls, want 1", got)
	}
}

func TestInstrumentedRowCloseReleasesConnection(t *testing.T) {
	t.Parallel()

	var releases atomic.Int32
	row := &instrumentedRow{
		row:        &fakeRow{scanErr: errors.New("scan failed")},
		inst:       newDBInstrumenter(slog.New(slog.NewTextHandler(os.Stdout, nil))),
		sql:        testPingQuery,
		queryStart: time.Now(),
		release:    func() { releases.Add(1) },
	}

	row.Close()
	if got := releases.Load(); got != 1 {
		t.Fatalf("release called %d times, want 1", got)
	}

	row.Close()
	if got := releases.Load(); got != 1 {
		t.Fatalf("release called %d times after second Close, want 1", got)
	}
}

func TestInstrumentedRowFinalizerReleasesDiscardedRow(t *testing.T) {
	t.Parallel()

	var releases atomic.Int32
	mkRow := func() *instrumentedRow {
		return &instrumentedRow{
			row:        &fakeRow{},
			inst:       newDBInstrumenter(slog.New(slog.NewTextHandler(os.Stdout, nil))),
			sql:        testPingQuery,
			queryStart: time.Now(),
			release:    func() { releases.Add(1) },
		}
	}

	// Create the row in a nested scope so it becomes unreachable when the
	// function returns, allowing the finalizer to run.
	func() {
		row := mkRow()
		runtime.SetFinalizer(row, (*instrumentedRow).Close)
	}()

	runtime.GC()
	runtime.GC()

	// Finalizers are best-effort; spin briefly to give the runtime a chance.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if releases.Load() == 1 {
			break
		}
		runtime.Gosched()
	}

	if got := releases.Load(); got != 1 {
		t.Fatalf("finalizer did not release connection; release called %d times", got)
	}
}

func TestSQLOperationUsesSQLCQueryName(t *testing.T) {
	t.Parallel()

	sql := `-- name: GetSessionByTokenHash :one
SELECT id, user_id
FROM sessions
WHERE token_hash = $1`

	if got, want := sqlOperation(sql), "get_session_by_token_hash"; got != want {
		t.Fatalf("sqlOperation() = %q, want %q", got, want)
	}
}

func TestSQLOperationFallsBackToVerbAndTable(t *testing.T) {
	t.Parallel()

	if got, want := sqlOperation(`SELECT id FROM users WHERE id = $1`), "select_users"; got != want {
		t.Fatalf("sqlOperation() = %q, want %q", got, want)
	}
}

var _ pgx.Row = (*fakeRow)(nil)

// stubPgxTx satisfies pgx.Tx by embedding the interface: the PgxTxOf tests
// only pass the value around, no method is ever invoked.
type stubPgxTx struct{ pgx.Tx }

// bareTx implements only the transaction port — the shape PgxTxOf must reject.
type bareTx struct{}

func (bareTx) Commit(context.Context) error   { return nil }
func (bareTx) Rollback(context.Context) error { return nil }

func TestPgxTxOfUnwrapsInstrumentedTx(t *testing.T) {
	t.Parallel()

	raw := &stubPgxTx{}
	instrumented := NewInstrumentedTx(raw, nil)

	got, err := PgxTxOf(instrumented)
	if err != nil {
		t.Fatalf("PgxTxOf(instrumented) error: %v", err)
	}
	if got != pgx.Tx(raw) {
		t.Fatalf("PgxTxOf(instrumented) = %v, want the raw pgx tx", got)
	}
}

func TestPgxTxOfPassesThroughPgxTx(t *testing.T) {
	t.Parallel()

	raw := &stubPgxTx{}
	got, err := PgxTxOf(raw)
	if err != nil {
		t.Fatalf("PgxTxOf(pgx.Tx) error: %v", err)
	}
	if got != pgx.Tx(raw) {
		t.Fatalf("PgxTxOf(pgx.Tx) = %v, want the same tx", got)
	}
}

func TestPgxTxOfRejectsNonPgxTx(t *testing.T) {
	t.Parallel()

	if _, err := PgxTxOf(bareTx{}); err == nil {
		t.Fatal("PgxTxOf(bare tx) = nil error, want rejection")
	}
}
