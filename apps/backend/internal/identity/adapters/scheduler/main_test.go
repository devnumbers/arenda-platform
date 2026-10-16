package scheduler

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the binary when a goroutine outlives its tests: the cleaner
// under test is a long-lived component (ticker loop held until context
// cancellation, issue #352), so a leaked clean goroutine means the loop
// ignored its owner's cancellation — exactly the regression this package
// must not ship.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
