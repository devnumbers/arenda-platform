package clock

import "time"

// Clock abstracts time source for testability and deterministic time-based logic.
type Clock interface {
	Now() time.Time
}
