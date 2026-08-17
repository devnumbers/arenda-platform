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

// latestLoginCodeQuery is the exact SQL produced by sqlc for
// GetLatestLoginCodeByPhoneAndEmailAndPurpose (see identity.sql.go). It is
// duplicated here as a string because sqlc emits it as an unexported const and
// EXPLAIN needs a literal. If the query text drifts, the matching sqlc const
// (getLatestLoginCodeByPhoneAndEmailAndPurpose) must be updated too.
const latestLoginCodeQuery = `SELECT id, user_id, phone, email, code_hash, expires_at, used, created_at, purpose, phone_encrypted FROM login_codes
WHERE phone = $1 AND email = $2 AND purpose = $3 AND used = false AND expires_at > $4
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`

// TestLoginCodeQueryPlan_IndexUsageForGetLatest is a diagnostic test (issue #224)
// that runs EXPLAIN ANALYZE against the GetLatestLoginCodeByPhoneAndEmailAndPurpose
// query to verify whether PostgreSQL uses the partial unique index
// idx_login_codes_unique_unused for the read path.
//
// The index was created by migration 000063 (phone, COALESCE(email, empty), purpose)
// WHERE used=false, primarily for CreateLoginCode's ON CONFLICT upsert. The read query
// filters on the raw email column (email = $2), and PostgreSQL cannot match a
// raw-column equality to an expression-index column — so email falls into Filter, not
// Index Cond. But the phone+purpose prefix still narrows the scan efficiently.
//
// The test seeds a production-realistic volume of login_codes (a large used historical
// trail plus a few hundred unused active codes) and asserts the default planner
// chooses an index scan over idx_login_codes_unique_unused. A regression to seq scan
// at this volume would mean the index no longer serves the read path.
func TestLoginCodeQueryPlan_IndexUsageForGetLatest(t *testing.T) {
	pool := testdb.Setup(t)
	ctx := t.Context()

	const (
		targetPhone   = "+79160000200"
		targetEmail   = "target@example.com"
		targetPurpose = "login"
	)

	now := time.Now().UTC()

	// Seed the target row: an unused, non-expired code for the target tuple.
	seedLoginCodeRow(t, pool, uuid.Must(uuid.NewV7()), targetPhone, targetEmail, targetPurpose,
		false, now.Add(5*time.Minute), now.Add(-1*time.Minute))

	// Same phone, different purpose — should not match the target query but lives
	// in the same partial-index partition.
	seedLoginCodeRow(t, pool, uuid.Must(uuid.NewV7()), targetPhone, targetEmail, "phone_change",
		false, now.Add(5*time.Minute), now.Add(-30*time.Second))
	// Same phone+email+purpose, but already used — excluded by the partial predicate.
	// A used+expired row cannot coexist with an unused one under the partial unique
	// index, so this also exercises the expires_at filter indirectly.
	seedLoginCodeRow(t, pool, uuid.Must(uuid.NewV7()), targetPhone, targetEmail, targetPurpose,
		true, now.Add(-1*time.Hour), now.Add(-2*time.Hour))

	// Seed enough rows so the planner treats the table as non-trivial and prefers
	// an index over a sequential scan. Used rows bloat the heap (audit trail); unused
	// rows populate the partial index. Distinct phones avoid unique collisions.
	bulkSeedLoginCodes(t, pool, 10000, true, now)
	bulkSeedLoginCodes(t, pool, 500, false, now)

	// Run ANALYZE so the planner has up-to-date statistics.
	if _, err := pool.Exec(ctx, "ANALYZE login_codes"); err != nil {
		t.Fatalf("ANALYZE login_codes: %v", err)
	}

	args := []any{targetPhone, targetEmail, targetPurpose, now}

	// Default plan: what the planner actually chooses at this volume.
	plan := explainPlanRows(t, pool, latestLoginCodeQuery, args)
	t.Logf("default plan:\n%s", plan)

	if !planUsesIndex(plan, "idx_login_codes_unique_unused") {
		t.Errorf("default plan does not use idx_login_codes_unique_unused — "+
			"the read query regressed to a non-index scan:\n%s", plan)
	}

	// Forced index (enable_seqscan=off): confirm the expression-index column
	// (COALESCE(email, ...)) cannot serve the email predicate — email appears in
	// Filter, not Index Cond. This documents *why* the prefix is phone+purpose only.
	t.Run("forced_index_reveals_email_filter", func(t *testing.T) {
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
		// email must be a post-scan Filter, never an Index Cond, because the index
		// column is COALESCE(email, ...) and cannot match a raw email = $2 equality.
		if strings.Contains(forced, "Index Cond:") &&
			strings.Contains(extractIndexCond(forced), "email") {
			t.Errorf("email appeared in Index Cond — unexpected; the index column is "+
				"COALESCE(email, ...) so email should only be a Filter:\n%s", forced)
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
// the repository Save path, this bypasses encryption so EXPLAIN sees real column
// values — which is exactly what index matching depends on.
func seedLoginCodeRow(
	t *testing.T,
	pool *pgxpool.Pool,
	id uuid.UUID,
	phone, email, purpose string,
	used bool,
	expiresAt, createdAt time.Time,
) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
INSERT INTO login_codes (id, phone, email, code_hash, expires_at, used, created_at, purpose, phone_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false)
`, id, phone, email, "diaghash", expiresAt, used, createdAt, purpose)
	if err != nil {
		t.Fatalf("seed login_codes row: %v", err)
	}
}

// bulkSeedLoginCodes inserts n login_codes rows in batches. When usedRows is true,
// it seeds used+expired rows (historical noise outside the partial index); when
// false, it seeds unused+non-expired rows with distinct phones (inside the partial
// index). Distinct phones avoid collisions on the partial unique index.
func bulkSeedLoginCodes(t *testing.T, pool *pgxpool.Pool, n int, usedRows bool, now time.Time) {
	t.Helper()
	ctx := t.Context()

	// Prefix encodes row kind to avoid phone collisions between the used and unused
	// batches (both iterate from 0).
	phonePrefix := 10
	if !usedRows {
		phonePrefix = 30
	}

	const batchSize = 500
	for offset := range n / batchSize {
		var sb strings.Builder
		sb.WriteString("INSERT INTO login_codes (id, phone, email, code_hash, expires_at, used, created_at, purpose, phone_encrypted) VALUES ")
		args := make([]any, 0, batchSize*9)
		for j := range batchSize {
			global := offset*batchSize + j
			phone := fmt.Sprintf("+7916%02d%07d", phonePrefix, global)
			expiresAt := now.Add(5 * time.Minute)
			if usedRows {
				expiresAt = now.Add(-time.Duration(global+1) * time.Minute)
			}
			args = append(args,
				uuid.Must(uuid.NewV7()), phone, "bulk@example.com", "bulkhash",
				expiresAt, usedRows, now.Add(-time.Duration(global+1)*time.Second), "login", false,
			)
			if j > 0 {
				sb.WriteByte(',')
			}
			base := j * 9
			fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9)
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
