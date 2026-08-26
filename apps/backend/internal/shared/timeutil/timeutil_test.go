package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextBoundary(t *testing.T) {
	t.Parallel()

	at := func(clock string) time.Time {
		parsed, err := time.Parse(time.DateTime, "2026-08-25 "+clock)
		require.NoError(t, err)
		return parsed
	}

	cases := []struct {
		name     string
		now      time.Time
		interval time.Duration
		want     string
	}{
		// An hourly worker started mid-hour first fires at the top of the
		// next hour — the deploy no longer sets the phase.
		{"hourly from mid-hour", at("13:47:12"), time.Hour, "2026-08-25 14:00:00"},
		// A boundary instant belongs to the next slot: strictly after now.
		{"exactly on boundary rolls to next", at("14:00:00"), time.Hour, "2026-08-25 15:00:00"},
		// Sub-hour intervals align to their wall-clock grid.
		{"five-minute grid", at("13:47:12"), 5 * time.Minute, "2026-08-25 13:50:00"},
		// Intervals that do not divide the hour keep a stable wall-clock grid
		// anchored at the epoch (00:00, 01:30, 03:00…).
		{"ninety-minute grid", at("00:47:12"), 90 * time.Minute, "2026-08-25 01:30:00"},
		// The result stays on the UTC grid regardless of the wall clock's
		// location: schedules are absolute, not local.
		{
			"absolute not local", at("13:47:12").In(time.FixedZone("MSK", 3*3600)), time.Hour,
			"2026-08-25 14:00:00",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NextBoundary(tc.now, tc.interval)
			want, err := time.Parse(time.DateTime, tc.want)
			require.NoError(t, err)
			assert.True(t, got.Equal(want), "NextBoundary(%v, %v) = %v, want %v",
				tc.now, tc.interval, got, want)
		})
	}
}
