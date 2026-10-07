package httpsupport

import "testing"

func TestETagMatches(t *testing.T) {
	t.Parallel()

	const etag = `"b34d1e0f6a2c9d7714ee5a0b3c8f1a2d"`

	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"empty header", "", false},
		{"exact", `"b34d1e0f6a2c9d7714ee5a0b3c8f1a2d"`, true},
		{"unquoted", `b34d1e0f6a2c9d7714ee5a0b3c8f1a2d`, true},
		{"weak prefix", `W/"b34d1e0f6a2c9d7714ee5a0b3c8f1a2d"`, true},
		{"star", `*`, true},
		{"list with a match", `"aaaa", "b34d1e0f6a2c9d7714ee5a0b3c8f1a2d"`, true},
		{"list without a match", `"aaaa", "bbbb"`, false},
		{"different", `"aaaa"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ETagMatches(tc.header, etag); got != tc.want {
				t.Errorf("ETagMatches(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
