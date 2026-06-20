package database

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
)

const (
	defaultSlowAcquireThreshold = 25 * time.Millisecond
	defaultSlowQueryThreshold   = 50 * time.Millisecond
)

type queryExecutor interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type transactionExecutor interface {
	queryExecutor
	Commit(context.Context) error
	Rollback(context.Context) error
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

func (db *InstrumentedPool) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
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

func (db *InstrumentedPool) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
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

func (db *InstrumentedPool) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	acquireStart := time.Now()
	conn, err := db.pool.Acquire(ctx)
	acquireDuration := time.Since(acquireStart)
	if err != nil {
		db.inst.log(ctx, sql, acquireDuration, 0, err)
		return errRow{err: err}
	}

	queryStart := time.Now()
	row := conn.QueryRow(ctx, sql, args...)
	return instrumentedRow{
		row:             row,
		inst:            db.inst,
		ctx:             ctx,
		sql:             sql,
		acquireDuration: acquireDuration,
		queryStart:      queryStart,
		release:         conn.Release,
	}
}

func (tx *InstrumentedTx) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	queryStart := time.Now()
	tag, err := tx.tx.Exec(ctx, sql, args...)
	tx.inst.log(ctx, sql, 0, time.Since(queryStart), err)
	return tag, err
}

func (tx *InstrumentedTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	queryStart := time.Now()
	rows, err := tx.tx.Query(ctx, sql, args...)
	if err != nil {
		tx.inst.log(ctx, sql, 0, time.Since(queryStart), err)
		return nil, err
	}

	return newInstrumentedRows(rows, tx.inst, ctx, sql, 0, queryStart, nil), nil
}

func (tx *InstrumentedTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	queryStart := time.Now()
	row := tx.tx.QueryRow(ctx, sql, args...)
	return instrumentedRow{
		row:        row,
		inst:       tx.inst,
		ctx:        ctx,
		sql:        sql,
		queryStart: queryStart,
	}
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
	ctx             context.Context
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
		ctx:             ctx,
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
		r.inst.log(r.ctx, r.sql, r.acquireDuration, time.Since(r.queryStart), err)
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
	ctx             context.Context
	sql             string
	acquireDuration time.Duration
	queryStart      time.Time
	release         func()
}

func (r instrumentedRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	r.inst.log(r.ctx, r.sql, r.acquireDuration, time.Since(r.queryStart), err)
	if r.release != nil {
		r.release()
	}
	return err
}

type errRow struct {
	err error
}

func (r errRow) Scan(dest ...any) error {
	return r.err
}

func (i dbInstrumenter) log(ctx context.Context, sql string, acquireDuration, queryDuration time.Duration, err error) {
	if !i.shouldLog(acquireDuration, queryDuration, err) {
		return
	}

	level := slog.LevelWarn
	message := "database query slow"
	if isLoggableQueryError(err) {
		level = slog.LevelError
		message = "database query failed"
	}

	attrs := []slog.Attr{
		slog.String("db_system", "postgresql"),
		slog.String("db_operation", sqlOperation(sql)),
		slog.String("sql_hash", sqlHash(sql)),
		slog.Duration("pool_acquire_duration", acquireDuration),
		slog.Duration("query_duration", queryDuration),
	}
	if requestID := requestctx.RequestIDFromContext(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if traceID := requestctx.TraceIDFromContext(ctx); traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", sanitizeError(err)))
	}

	i.logger.LogAttrs(ctx, level, message, attrs...)
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
	for _, line := range strings.Split(sql, "\n") {
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

var (
	// Redact common credential patterns (case-insensitive, optional surrounding quotes).
	instTokenPattern    = regexp.MustCompile(`(?i)(token|password|secret|key)\s*[:=]\s*["']?[^\s"'&]+["']?`)
	instHexTokenPattern = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
	instB64TokenPattern = regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`)
)

const maxSanitizedErrorLength = 1024

// sanitizeError redacts likely secrets and truncates an error string before
// logging. It is defined locally in the database package to avoid an import
// cycle with the httpapi package.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	s = instTokenPattern.ReplaceAllString(s, "${1}=[REDACTED]")
	s = instHexTokenPattern.ReplaceAllString(s, "[REDACTED]")
	s = instB64TokenPattern.ReplaceAllString(s, "[REDACTED]")
	if len(s) > maxSanitizedErrorLength {
		s = s[:maxSanitizedErrorLength] + " [truncated]"
	}
	return strings.TrimSpace(s)
}
