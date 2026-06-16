package timeutil

import "time"

// Date returns the UTC midnight date for the given time.
func Date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
