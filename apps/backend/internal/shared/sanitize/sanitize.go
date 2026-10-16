// Package sanitize provides helpers for removing sensitive data from strings
// before they are logged or returned to users.
package sanitize

import (
	"regexp"
	"strings"
)

var (
	// Matches words that suggest the adjacent value is a secret.
	credentialKeyword = `token|password|secret|key|auth|credential|bearer|authorization|apikey|access_key`

	// Redact common credential patterns (case-insensitive, optional surrounding quotes).
	tokenPattern = regexp.MustCompile(`(?i)(token|password|secret|key)\s*[:=]\s*["']?[^\s"'&]+["']?`)
	// Redact hexadecimal strings that follow a credential-like keyword.
	hexTokenPattern = regexp.MustCompile(`(?i)\b(` + credentialKeyword + `)\b[\s"'=:]*([0-9a-fA-F]{32,})["']?`)
	// Redact base64 strings that follow a credential-like keyword.
	b64TokenPattern = regexp.MustCompile(`(?i)\b(` + credentialKeyword + `)\b[\s"'=:]*([A-Za-z0-9+/]{40,}={0,2})["']?`)
	// Redact Russian/international phone numbers in common formats.
	phonePattern = regexp.MustCompile(`\+7[-\s]?\(?\d{3}\)?[-\s]?\d{3}[-\s]?\d{2}[-\s]?\d{2}`)
	// Redact compact phone numbers such as +79991234567.
	phoneCompactPattern = regexp.MustCompile(`\+7\d{10}`)
	// Redact likely payment card numbers (13-19 digits with optional separators).
	panPattern = regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{1,4}(?:[-\s]?\d{1,3})?\b`)
	// Redact masked PANs such as ****-****-****-1234.
	maskedPanPattern = regexp.MustCompile(`(?:\*{4}[-\s]?){2,3}\d{4}`)
)

const maxSanitizedErrorLength = 1024

// Error redacts likely secrets, PCI data, and PII from an error before
// logging. It keeps enough detail for debugging while reducing the risk of
// leaking credentials, raw upstream responses, phone numbers, or card data.
// A nil error returns an empty string.
func Error(err error) string {
	if err == nil {
		return ""
	}
	s := redact(err.Error())
	if len(s) > maxSanitizedErrorLength {
		s = s[:maxSanitizedErrorLength] + " [truncated]"
	}
	return s
}

// String redacts likely secrets, PCI data, and PII from an arbitrary string
// before logging. Unlike Error, it applies no length cap, so callers keep
// control over truncation.
func String(s string) string {
	return redact(s)
}

// redact applies all sensitive-data patterns to s.
func redact(s string) string {
	s = tokenPattern.ReplaceAllString(s, "${1}=[REDACTED]")
	s = hexTokenPattern.ReplaceAllString(s, "${1}=[REDACTED]")
	s = b64TokenPattern.ReplaceAllString(s, "${1}=[REDACTED]")
	s = phoneCompactPattern.ReplaceAllString(s, "[REDACTED]")
	s = phonePattern.ReplaceAllString(s, "[REDACTED]")
	s = panPattern.ReplaceAllString(s, "[REDACTED]")
	s = maskedPanPattern.ReplaceAllString(s, "[REDACTED]")
	return strings.TrimSpace(s)
}

// wrappedError preserves the original error chain for errors.Is/errors.As
// while exposing a sanitized Error() string.
type wrappedError struct {
	sanitized string
	original  error
}

func (e *wrappedError) Error() string { return e.sanitized }
func (e *wrappedError) Unwrap() error { return e.original }

// Wrap returns an error whose message is sanitized but which still unwraps to
// the original error. This is useful when an error is returned from a service
// and may be logged by callers: the log text stays safe while error identity
// checks continue to work.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &wrappedError{sanitized: msg + ": " + Error(err), original: err}
}
