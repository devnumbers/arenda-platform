//go:build !integration

package scheduler

import "go.uber.org/goleak"

// goleakOpts returns no exemptions in the unit build: without the integration
// tag this binary runs no testcontainers infrastructure, so any leaked
// goroutine is a worker regression and must fail the run.
func goleakOpts() []goleak.Option { return nil }
