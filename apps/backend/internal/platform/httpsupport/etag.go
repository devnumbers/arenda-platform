package httpsupport

import "strings"

// ETagMatches reports whether an If-None-Match header value matches the
// etag (RFC 9110 §8.8.3.2): `*` matches any existing representation, the
// list is comma-separated, and weak prefixes (W/) compare equal — the
// photo's bytes are immutable per key (ADR 0065), so a weak match is
// sufficient for the 304.
func ETagMatches(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "" {
		return false
	}
	if strings.TrimSpace(ifNoneMatch) == "*" {
		return true
	}
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		candidate = strings.Trim(candidate, `"`)
		if candidate == strings.Trim(etag, `"`) {
			return true
		}
	}
	return false
}
