package domain

import "time"

// Shared fixtures of the domain tests. All dates are strings in the
// YYYY-MM-DD form the date-only assertions compare against; September 2026
// anchors every case on the Thursday the 10th.
const (
	aug27 = "2026-08-27"
	sep03 = "2026-09-03"
	sep10 = "2026-09-10"
	sep17 = "2026-09-17"
	sep24 = "2026-09-24"
)

func mustLocation(name string) *time.Location {
	tz, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return tz
}
