package postgres

// Unit tests of the store's query folding helpers (ticket #476): the amount
// side of the extended operations search, which decides client-side-looking
// semantics — when a query counts as an amount query and which digits travel
// to the SQL LIKE — before any database is involved.

import "testing"

func TestSearchAmountDigits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"empty query", "", ""},
		{"blank query", "   ", ""},
		{"plain digits", "2500", "2500"},
		{"thousand separator", "2 500", "2500"},
		{"decimal comma", "2500,50", "250050"},
		{"decimal point", "2500.50", "250050"},
		{"minus sign and nbsp", "\u22122\u00a0500", "2500"},
		{"leading zeros kept", "0250", "0250"},
		{"letters switch amount matching off", "клининг 500", ""},
		{"single letter", "о", ""},
		{"separators without digits", " ..,-- ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := searchAmountDigits(tc.query); got != tc.want {
				t.Errorf("searchAmountDigits(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}
