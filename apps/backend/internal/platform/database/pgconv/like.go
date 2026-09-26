package pgconv

import "strings"

// likePatternEscaper escapes the ILIKE metacharacters so the SQL predicate
// can match them literally with ESCAPE '\'.
var likePatternEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLikePattern trims and escapes a user-supplied substring so it can be
// used as an ILIKE/LIKE pattern with ESCAPE '\' (trigram and plain ILIKE
// search predicates). Trimming here is idempotent for callers that already
// trimmed in the application layer.
func EscapeLikePattern(q string) string {
	return likePatternEscaper.Replace(strings.TrimSpace(q))
}
