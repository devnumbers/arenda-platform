//go:build integration

package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
)

// latestLoginCodeQuery is the SQL produced by sqlc for
// GetLatestLoginCodeByEmailAndPurpose (see identity.sql.go), with line breaks
// added for lll — SQL is whitespace-insensitive. It is duplicated here as a
// string because sqlc emits it as an unexported const and EXPLAIN needs a
// literal. If the query text drifts, the matching sqlc const
// (getLatestLoginCodeByEmailAndPurpose) must be updated too.
const latestLoginCodeQuery = `SELECT id, user_id, email, code_hash, expires_at, used, created_at, purpose FROM login_codes
WHERE email = $1 AND purpose = $2 AND used = false AND expires_at > $3
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`

// TestLoginCodeQueryPlan_IndexUsageForGetLatest is a diagnostic test (issue #224)
// that runs EXPLAIN ANALYZE against the GetLatestLoginCodeByEmailAndPurpose
// query to verify whether PostgreSQL uses the partial unique index
// idx_login_codes_unique_unused for the read path.
//
// The index was recreated by migration 000162 as (email, purpose) WHERE
// used=false — the email-only identity shape (ADR 0067) — primarily for
// CreateLoginCode's ON CONFLICT upsert. Both index columns are raw table
// columns, so the read query's email and purpose equalities match them
// directly; only the TTL predicate (expires_at > $3) falls outside the index.
//
// The test seeds a production-realistic volume of login_codes (a large used historical
// trail plus a few hundred unused active codes) and asserts the default planner
// chooses an index scan over idx_login_codes_unique_unused. A regression to seq scan
// at this volume would mean the index no longer serves the read path.
func TestLoginCodeQueryPlan_IndexUsageForGetLatest(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := t.Context()

	const (
		targetEmail   = "target@example.com"
		targetPurpose = "login"
	)

	now := time.Now().UTC()

	// Seed the target row: an unused, non-expired code for the target pair.
	seedLoginCodeRow(t, pool, uuid.Must(uuid.NewV7()), targetEmail, targetPurpose,
		false, now.Add(5*time.Minute), now.Add(-1*time.Minute))

	// Same email, different purpose — should not match the target query but lives
	// in the same partial-index partition (the (email, purpose) pair stays unique).
	seedLoginCodeRow(t, pool, uuid.Must(uuid.NewV7()), targetEmail, "email_change",
		false, now.Add(5*time.Minute), now.Add(-30*time.Second))
	// Same email+purpose, but already used — excluded by the partial predicate.
	// A used+expired row cannot coexist with an unused one under the partial unique
	// index, so this also exercises the expires_at filter indirectly.
	seedLoginCodeRow(t, pool, uuid.Must(uuid.NewV7()), targetEmail, targetPurpose,
		true, now.Add(-1*time.Hour), now.Add(-2*time.Hour))

	// Seed enough rows so the planner treats the table as non-trivial and prefers
	// an index over a sequential scan. Used rows bloat the heap (audit trail); unused
	// rows populate the partial index. Distinct emails avoid unique collisions.
	bulkSeedLoginCodes(t, pool, 10000, true, now)
	bulkSeedLoginCodes(t, pool, 500, false, now)

	// Run ANALYZE so the planner has up-to-date statistics.
	if _, err := pool.Exec(ctx, "ANALYZE login_codes"); err != nil {
		t.Fatalf("ANALYZE login_codes: %v", err)
	}

	args := []any{targetEmail, targetPurpose, now}

	// Default plan: what the planner actually chooses at this volume.
	plan := explainPlanRows(t, pool, latestLoginCodeQuery, args)
	t.Logf("default plan:\n%s", plan)

	if !planUsesIndex(plan, "idx_login_codes_unique_unused") {
		t.Errorf("default plan does not use idx_login_codes_unique_unused — "+
			"the read query regressed to a non-index scan:\n%s", plan)
	}

	// Forced index (enable_seqscan=off): confirm the index cond covers the full
	// (email, purpose) equality pair — both are raw index columns — while the
	// TTL predicate stays a post-scan Filter. This documents *why* the index
	// serves the read path: only expires_at falls outside it.
	t.Run("forced_index_reveals_expires_at_filter", func(t *testing.T) {
		t.Parallel()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire conn: %v", err)
		}
		defer conn.Release()
		if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
			t.Fatalf("SET enable_seqscan=off: %v", err)
		}

		forced := explainPlanConn(ctx, t, conn.Conn(), latestLoginCodeQuery, args)
		t.Logf("forced-index plan (enable_seqscan=off):\n%s", forced)

		if !planUsesIndex(forced, "idx_login_codes_unique_unused") {
			t.Errorf("forced plan does not use idx_login_codes_unique_unused:\n%s", forced)
		}
		indexCond := extractIndexCond(forced)
		// Both equality predicates must be served by the index: email and
		// purpose are raw columns of idx_login_codes_unique_unused.
		for _, col := range []string{"email", "purpose"} {
			if !strings.Contains(indexCond, col) {
				t.Errorf("index cond = %q, want %s matched by the index:\n%s", indexCond, col, forced)
			}
		}
		// The TTL predicate must remain a Filter, never an Index Cond: the
		// index carries no expires_at column.
		if strings.Contains(indexCond, "expires_at") {
			t.Errorf("expires_at appeared in Index Cond — unexpected; the index is "+
				"(email, purpose) so the TTL predicate should only be a Filter:\n%s", forced)
		}
	})
}

// planUsesIndex reports whether the EXPLAIN plan text references the named index
// in a scan node. PostgreSQL prints scan nodes as indented lines like
// "->  Index Scan using <index> on <table>", so we search for the index name
// alongside a scan operator anywhere in the line (not just as a prefix).
func planUsesIndex(plan, indexName string) bool {
	for line := range strings.SplitSeq(plan, "\n") {
		switch {
		case strings.Contains(line, "Index Scan"),
			strings.Contains(line, "Index Only Scan"),
			strings.Contains(line, "Bitmap Index Scan"):
			if strings.Contains(line, indexName) {
				return true
			}
		}
	}
	return false
}

// extractIndexCond returns the joined "Index Cond:" lines from an EXPLAIN plan, so
// a caller can assert which columns the index actually matched.
func extractIndexCond(plan string) string {
	var conds []string
	for line := range strings.SplitSeq(plan, "\n") {
		trimmed := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(trimmed, "Index Cond:"); ok {
			conds = append(conds, strings.TrimSpace(rest))
		}
	}
	return strings.Join(conds, " ")
}

// seedLoginCodeRow inserts a raw login_codes row for diagnostic purposes. Unlike
// the repository Save path, this writes the columns directly so EXPLAIN sees
// real column values — which is exactly what index matching depends on.
func seedLoginCodeRow(
	t *testing.T,
	pool *pgxpool.Pool,
	id uuid.UUID,
	email, purpose string,
	used bool,
	expiresAt, createdAt time.Time,
) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
INSERT INTO login_codes (id, email, code_hash, expires_at, used, created_at, purpose)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, id, email, "diaghash", expiresAt, used, createdAt, purpose)
	if err != nil {
		t.Fatalf("seed login_codes row: %v", err)
	}
}

// bulkSeedLoginCodes inserts n login_codes rows in batches. When usedRows is true,
// it seeds used+expired rows (historical noise outside the partial index); when
// false, it seeds unused+non-expired rows with distinct emails (inside the partial
// index). Distinct emails avoid collisions on the partial unique index.
func bulkSeedLoginCodes(t *testing.T, pool *pgxpool.Pool, n int, usedRows bool, now time.Time) {
	t.Helper()
	ctx := t.Context()

	// Prefix encodes row kind to avoid email collisions between the used and
	// unused batches (both iterate from 0).
	emailPrefix := "bulk-used"
	if !usedRows {
		emailPrefix = "bulk-unused"
	}

	const batchSize = 500
	for offset := range n / batchSize {
		var sb strings.Builder
		sb.WriteString("INSERT INTO login_codes (id, email, code_hash, expires_at, used, created_at, purpose) VALUES ")
		args := make([]any, 0, batchSize*7)
		for j := range batchSize {
			global := offset*batchSize + j
			email := fmt.Sprintf("%s%07d@example.com", emailPrefix, global)
			expiresAt := now.Add(5 * time.Minute)
			createdAt := now.Add(-time.Duration(global+1) * time.Second)
			if usedRows {
				// Used rows are expired historical noise; the lifecycle must
				// still hold — chk_login_codes_expires_after_created forbids
				// a code created already expired.
				expiresAt = now.Add(-time.Duration(global+1) * time.Minute)
				createdAt = now.Add(-time.Duration(global+1) * time.Hour)
			}
			args = append(args,
				uuid.Must(uuid.NewV7()), email, "bulkhash",
				expiresAt, usedRows, createdAt, "login",
			)
			if j > 0 {
				sb.WriteByte(',')
			}
			base := j * 7
			if _, err := fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7); err != nil {
				t.Fatalf("append placeholder row %d: %v", j, err)
			}
		}
		if _, err := pool.Exec(ctx, sb.String(), args...); err != nil {
			t.Fatalf("bulk seed login_codes (used=%v batch %d): %v", usedRows, offset, err)
		}
	}
}

// explainPlanRows runs EXPLAIN (ANALYZE) via a pool and returns the plan as text.
func explainPlanRows(t *testing.T, pool *pgxpool.Pool, query string, args []any) string {
	t.Helper()
	rows, err := pool.Query(t.Context(), "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	return scanExplainLines(t, rows)
}

// explainPlanConn runs EXPLAIN (ANALYZE) on a specific connection so session-level
// GUCs like enable_seqscan=off apply, and returns the plan as text.
func explainPlanConn(ctx context.Context, t *testing.T, conn *pgx.Conn, query string, args []any) string {
	t.Helper()
	rows, err := conn.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	return scanExplainLines(t, rows)
}

// scanExplainLines collects the single-column text rows of an EXPLAIN result.
func scanExplainLines(t *testing.T, rows pgx.Rows) string {
	t.Helper()
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan EXPLAIN line: %v", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate EXPLAIN: %v", err)
	}
	return strings.Join(lines, "\n")
}
