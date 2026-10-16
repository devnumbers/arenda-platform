//go:build integration

package scheduler

import "go.uber.org/goleak"

// goleakOpts exempts the testcontainers infrastructure this binary starts
// when TEST_DATABASE_URL is unset: the reaper connection goroutine
// ((*Reaper).connect.func1, testcontainers-go v0.44) blocks on its
// termination signal for the whole process lifetime by design, and no
// TerminateContainer call can end it — the handle is reused across
// containers via the package's own sync.Once. It is fixture machinery, not
// a scheduler worker, and in a TEST_DATABASE_URL run (or with
// TESTCONTAINERS_RYUK_DISABLED=true) it is never created, so the exemption
// simply matches nothing. The .func1 name is pinned to v0.44: a testcontainers
// upgrade that renames it fails this gate loudly — update the string then.
func goleakOpts() []goleak.Option {
	return []goleak.Option{
		goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"),
	}
}
