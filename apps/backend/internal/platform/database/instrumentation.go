package database

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const (
	defaultSlowAcquireThreshold = 25 * time.Millisecond
	defaultSlowQueryThreshold   = 50 * time.Millisecond
)

type queryExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type transactionExecutor interface {
	queryExecutor
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type dbInstrumenter struct {
	logger               *slog.Logger
	slowAcquireThreshold time.Duration
	slowQueryThreshold   time.Duration
}

// InstrumentedPool wraps a pgxpool.Pool with slow acquire/query diagnostics.
type InstrumentedPool struct {
	pool *pgxpool.Pool
	inst dbInstrumenter
}

// NewInstrumentedPool returns a DBTX-compatible wrapper around pool.
func NewInstrumentedPool(pool *pgxpool.Pool, logger *slog.Logger) *InstrumentedPool {
	return &InstrumentedPool{
		pool: pool,
		inst: newDBInstrumenter(logger),
	}
}

// InstrumentedTx wraps a pgx transaction with slow query diagnostics.
type InstrumentedTx struct {
	tx   transactionExecutor
	inst dbInstrumenter
}

// NewInstrumentedTx returns a DBTX-compatible transaction wrapper.
func NewInstrumentedTx(tx transactionExecutor, logger *slog.Logger) *InstrumentedTx {
	return &InstrumentedTx{
		tx:   tx,
		inst: newDBInstrumenter(logger),
	}
}

func newDBInstrumenter(logger *slog.Logger) dbInstrumenter {
	if logger == nil {
		logger = slog.Default()
	}
	return dbInstrumenter{
		logger:               logger,
		slowAcquireThreshold: defaultSlowAcquireThreshold,
		slowQueryThreshold:   defaultSlowQueryThreshold,
	}
}

func (db *InstrumentedPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	acquireStart := time.Now()
	conn, err := db.pool.Acquire(ctx)
	acquireDuration := time.Since(acquireStart)
	if err != nil {
		db.inst.log(ctx, sql, acquireDuration, 0, err)
		return pgconn.CommandTag{}, err
	}
	defer conn.Release()

	queryStart := time.Now()
	tag, err := conn.Exec(ctx, sql, args...)
	db.inst.log(ctx, sql, acquireDuration, time.Since(queryStart), err)
	return tag, err
}

func (db *InstrumentedPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	acquireStart := time.Now()
	conn, err := db.pool.Acquire(ctx)
	acquireDuration := time.Since(acquireStart)
	if err != nil {
		db.inst.log(ctx, sql, acquireDuration, 0, err)
		return nil, err
	}

	queryStart := time.Now()
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		conn.Release()
		db.inst.log(ctx, sql, acquireDuration, time.Since(queryStart), err)
		return nil, err
	}

	return newInstrumentedRows(rows, db.inst, ctx, sql, acquireDuration, queryStart, conn.Release), nil
}

func (db *InstrumentedPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	acquireStart := time.Now()
	conn, err := db.pool.Acquire(ctx)
	acquireDuration := time.Since(acquireStart)
	if err != nil {
		db.inst.log(ctx, sql, acquireDuration, 0, err)
		return errRow{err: err}
	}

	queryStart := time.Now()
	row := conn.QueryRow(ctx, sql, args...)
	// instrumentedRow acquires a connection from the pool. Callers must either
	// Scan the row or Close it to release the connection. A runtime finalizer
	// provides a best-effort fallback if the row is discarded without Scan/Close.
	ir := &instrumentedRow{
		row:             row,
		inst:            db.inst,
		requestID:       requestctx.RequestIDFromContext(ctx),
		traceID:         requestctx.TraceIDFromContext(ctx),
		sql:             sql,
		acquireDuration: acquireDuration,
		queryStart:      queryStart,
		release:         conn.Release,
	}
	runtime.SetFinalizer(ir, (*instrumentedRow).Close)
	return ir
}

func (db *InstrumentedPool) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return db.pool.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

// Begin starts a new transaction on the underlying pool.
func (db *InstrumentedPool) Begin(ctx context.Context) (transaction.Tx, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return NewInstrumentedTx(tx, db.inst.logger), nil
}

func (tx *InstrumentedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	queryStart := time.Now()
	tag, err := tx.tx.Exec(ctx, sql, args...)
	tx.inst.log(ctx, sql, 0, time.Since(queryStart), err)
	return tag, err
}

func (tx *InstrumentedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	queryStart := time.Now()
	rows, err := tx.tx.Query(ctx, sql, args...)
	if err != nil {
		tx.inst.log(ctx, sql, 0, time.Since(queryStart), err)
		return nil, err
	}

	return newInstrumentedRows(rows, tx.inst, ctx, sql, 0, queryStart, nil), nil
}

func (tx *InstrumentedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	queryStart := time.Now()
	row := tx.tx.QueryRow(ctx, sql, args...)
	return &instrumentedRow{
		row:        row,
		inst:       tx.inst,
		requestID:  requestctx.RequestIDFromContext(ctx),
		traceID:    requestctx.TraceIDFromContext(ctx),
		sql:        sql,
		queryStart: queryStart,
	}
}

func (tx *InstrumentedTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return tx.tx.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

func (tx *InstrumentedTx) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

func (tx *InstrumentedTx) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}

type instrumentedRows struct {
	rows            pgx.Rows
	inst            dbInstrumenter
	requestID       string
	traceID         string
	sql             string
	acquireDuration time.Duration
	queryStart      time.Time
	release         func()
	closeOnce       sync.Once
}

func newInstrumentedRows(rows pgx.Rows, inst dbInstrumenter, ctx context.Context, sql string, acquireDuration time.Duration, queryStart time.Time, release func()) *instrumentedRows {
	return &instrumentedRows{
		rows:            rows,
		inst:            inst,
		requestID:       requestctx.RequestIDFromContext(ctx),
		traceID:         requestctx.TraceIDFromContext(ctx),
		sql:             sql,
		acquireDuration: acquireDuration,
		queryStart:      queryStart,
		release:         release,
	}
}

func (r *instrumentedRows) Close() {
	r.closeOnce.Do(func() {
		r.rows.Close()
		err := r.rows.Err()
		r.inst.logWithIDs(context.Background(), r.sql, r.acquireDuration, time.Since(r.queryStart), err, r.requestID, r.traceID)
		if r.release != nil {
			r.release()
		}
	})
}

func (r *instrumentedRows) Err() error {
	return r.rows.Err()
}

func (r *instrumentedRows) CommandTag() pgconn.CommandTag {
	return r.rows.CommandTag()
}

func (r *instrumentedRows) FieldDescriptions() []pgconn.FieldDescription {
	return r.rows.FieldDescriptions()
}

func (r *instrumentedRows) Next() bool {
	if r.rows.Next() {
		return true
	}
	r.Close()
	return false
}

func (r *instrumentedRows) Scan(dest ...any) error {
	err := r.rows.Scan(dest...)
	if err != nil {
		r.Close()
	}
	return err
}

func (r *instrumentedRows) Values() ([]any, error) {
	values, err := r.rows.Values()
	if err != nil {
		r.Close()
	}
	return values, err
}

func (r *instrumentedRows) RawValues() [][]byte {
	return r.rows.RawValues()
}

func (r *instrumentedRows) Conn() *pgx.Conn {
	return r.rows.Conn()
}

type instrumentedRow struct {
	row             pgx.Row
	inst            dbInstrumenter
	requestID       string
	traceID         string
	sql             string
	acquireDuration time.Duration
	queryStart      time.Time
	release         func()
	closeOnce       sync.Once
}

// Scan copies column values into dest and releases the acquired connection.
// It is safe to call multiple times, but the connection is released on the
// first call.
func (r *instrumentedRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	r.inst.logWithIDs(context.Background(), r.sql, r.acquireDuration, time.Since(r.queryStart), err, r.requestID, r.traceID)
	r.Close()
	return err
}

// Close releases the connection acquired for this row without reading its
// values. It is idempotent and safe to call after Scan. Callers must invoke
// either Scan or Close; a runtime finalizer provides a best-effort fallback
// when the row is discarded.
func (r *instrumentedRow) Close() {
	r.closeOnce.Do(func() {
		if r.release != nil {
			r.release()
		}
	})
}

type errRow struct {
	err error
}

func (r errRow) Scan(dest ...any) error {
	return r.err
}

func (i dbInstrumenter) log(ctx context.Context, sql string, acquireDuration, queryDuration time.Duration, err error) {
	i.logWithIDs(ctx, sql, acquireDuration, queryDuration, err, requestctx.RequestIDFromContext(ctx), requestctx.TraceIDFromContext(ctx))
}

// logWithIDs is log with the request-scoped correlation IDs already extracted.
// The rows wrappers use it because pgx.Rows/pgx.Row callbacks have no access to
// the original query context; they capture the IDs at construction time.
func (i dbInstrumenter) logWithIDs(ctx context.Context, sql string, acquireDuration, queryDuration time.Duration, err error, requestID, traceID string) {
	if !i.shouldLog(acquireDuration, queryDuration, err) {
		return
	}

	attrs := []slog.Attr{
		slog.String("db_system", "postgresql"),
		slog.String("db_operation", sqlOperation(sql)),
		slog.String("sql_hash", sqlHash(sql)),
		slog.Duration("pool_acquire_duration", acquireDuration),
		slog.Duration("query_duration", queryDuration),
	}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", sanitize.Error(err)))
	}

	// Static messages at the call sites (sloglint static-msg): the error/slow
	// distinction lives in the level, not in an interpolated message.
	if isLoggableQueryError(err) {
		i.logger.LogAttrs(ctx, slog.LevelError, "database query failed", attrs...)
		return
	}
	i.logger.LogAttrs(ctx, slog.LevelWarn, "database query slow", attrs...)
}

func (i dbInstrumenter) shouldLog(acquireDuration, queryDuration time.Duration, err error) bool {
	return isLoggableQueryError(err) ||
		(i.slowAcquireThreshold > 0 && acquireDuration >= i.slowAcquireThreshold) ||
		(i.slowQueryThreshold > 0 && queryDuration >= i.slowQueryThreshold)
}

func isLoggableQueryError(err error) bool {
	return err != nil && !errors.Is(err, pgx.ErrNoRows)
}

func sqlHash(sql string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(normalizeSQL(sql)))
	return fmt.Sprintf("%016x", h.Sum64())
}

func sqlOperation(sql string) string {
	if name := sqlcQueryName(sql); name != "" {
		return sanitizeSQLOperationName(name)
	}

	fields := strings.Fields(strings.ToLower(sql))
	if len(fields) == 0 {
		return "unknown"
	}

	verb := fields[0]
	table := ""
	switch verb {
	case "select":
		table = wordAfter(fields, "from")
	case "insert":
		table = wordAfter(fields, "into")
	case "update":
		if len(fields) > 1 {
			table = fields[1]
		}
	case "delete":
		table = wordAfter(fields, "from")
	}
	if table == "" {
		return sanitizeSQLIdent(verb)
	}
	return sanitizeSQLIdent(verb) + "_" + sanitizeSQLIdent(table)
}

func wordAfter(fields []string, word string) string {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == word {
			return fields[i+1]
		}
	}
	return ""
}

func sqlcQueryName(sql string) string {
	for line := range strings.SplitSeq(sql, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "--") {
			return ""
		}

		header := strings.TrimSpace(strings.TrimPrefix(line, "--"))
		if !strings.HasPrefix(header, "name:") {
			continue
		}

		parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(header, "name:")))
		if len(parts) == 0 {
			return ""
		}
		return parts[0]
	}
	return ""
}

func sanitizeSQLOperationName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	var b strings.Builder
	var lastUnderscore bool
	var prevWasLowerOrDigit bool
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			if b.Len() > 0 && prevWasLowerOrDigit && !lastUnderscore {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			lastUnderscore = false
			prevWasLowerOrDigit = false
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
			prevWasLowerOrDigit = true
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
			prevWasLowerOrDigit = true
		case r == '_':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
			prevWasLowerOrDigit = false
		default:
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
			prevWasLowerOrDigit = false
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "unknown"
	}
	return result
}

func sanitizeSQLIdent(value string) string {
	value = strings.Trim(value, `"(),;`)
	value = strings.TrimPrefix(value, "public.")
	if value == "" {
		return "unknown"
	}

	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
