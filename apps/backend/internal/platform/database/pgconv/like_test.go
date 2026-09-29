package pgconv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscapeLikePattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text is kept", in: "Иван", want: "Иван"},
		{name: "percent is escaped", in: "50% скидка", want: `50\% скидка`},
		{name: "underscore is escaped", in: "a_b", want: `a\_b`},
		{name: "backslash is escaped", in: `a\b`, want: `a\\b`},
		{name: "all metacharacters", in: `%_\`, want: `\%\_\\`},
		{name: "edges are trimmed", in: "  foo  ", want: "foo"},
		{name: "trim and escape together", in: " %50 ", want: `\%50`},
		{name: "empty stays empty", in: "", want: ""},
		{name: "whitespace-only folds to empty", in: "   ", want: ""},
		{name: "non-escaped runes survive", in: "идёт «дождь»?", want: "идёт «дождь»?"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, EscapeLikePattern(tc.in))
		})
	}
}
