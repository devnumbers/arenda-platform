package database

import "testing"

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
