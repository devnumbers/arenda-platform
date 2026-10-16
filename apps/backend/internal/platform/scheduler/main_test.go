package scheduler

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the binary when a goroutine outlives its tests: the workers
// under test here are the platform's long-lived components (ticker loops held
// until context cancellation, issue #352), so a leaked tick goroutine means a
// worker ignored its owner's cancellation — exactly the regression this
// package must not ship. Infrastructure goroutines that the integration build
// starts (testcontainers) are exempted in main_integration_test.go.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleakOpts()...)
}
