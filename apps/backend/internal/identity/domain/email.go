package domain

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Email length limits per RFC 5321 §4.5.3.1. These are pragmatic guards, not a
// full RFC 5322 parser: they reject obviously corrupt or abusive addresses
// without the complexity (and false-negative risk) of a grammar parser. The
// local-part ceiling matters most for the trusted DB path (EmailFrom), which
// must not crash on oversized data imported from another system.
const (
	maxEmailTotalLen = 254
	maxEmailLocalLen = 64
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type Email struct {
	value string
}

func NewEmail(raw string) (Email, error) {
	normalized, err := NormalizeEmail(raw)
	if err != nil {
		return Email{}, err
	}
	return Email{value: normalized}, nil
}

// EmailFrom creates an Email from an already-normalized address.
// It is intended for trusted sources such as the database. An empty value or
// an invalid address returns an error so callers do not silently propagate
// corrupt or missing data.
func EmailFrom(normalized string) (Email, error) {
	if normalized == "" {
		return Email{}, ErrInvalidEmail
	}
	return NewEmail(normalized)
}

// NormalizeEmail trims and lowercases the raw address and validates its shape
// and length. Lowercasing is the only normalization applied: NFKC is
// deliberately omitted because it can change the meaning of a local-part (the
// portion before @ is case-sensitive for many providers), so unicode
// normalization would risk producing an address that no longer matches the
// recipient's mailbox. IDN/punycode decoding is likewise skipped — the platform
// delivers to whatever address the user entered; the mail transport, not the
// domain, is responsible for ACE encoding if a provider requires it.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || !isValidEmail(email) {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func (e Email) String() string {
	return e.value
}

func isValidEmail(email string) bool {
	if len(email) > maxEmailTotalLen {
		return false
	}
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return false
	}
	// The local-part limit is a character (rune) count per RFC 5321 §4.5.3.1,
	// so count runes, not bytes — a 64-rune local-part with multibyte chars
	// (e.g. Cyrillic) must pass even though its byte length exceeds 64.
	if utf8.RuneCountInString(email[:at]) > maxEmailLocalLen {
		return false
	}
	return emailRegex.MatchString(email)
}
