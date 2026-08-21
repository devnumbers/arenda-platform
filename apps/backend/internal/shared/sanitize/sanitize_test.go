package sanitize

import (
	"errors"
	"strings"
	"testing"
)

func TestError_RedactsSensitiveSubstrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		forbidden   []string
		mustContain []string
	}{
		{
			name:        "token",
			input:       "provider rejected request: api_token=abc123secret",
			forbidden:   []string{"abc123secret"},
			mustContain: []string{"api_token=[REDACTED]"},
		},
		{
			name:        "password",
			input:       "auth failed: password='super_secret_123'",
			forbidden:   []string{"super_secret_123"},
			mustContain: []string{"password=[REDACTED]"},
		},
		{
			name:        "hex token",
			input:       "upstream error: authorization deadbeefcafebabe0011223344556677",
			forbidden:   []string{"deadbeefcafebabe0011223344556677"},
			mustContain: []string{"authorization=[REDACTED]"},
		},
		{
			name:        "base64 token",
			input:       "upstream error: bearer dGhpcyBpcyBhIHNlY3JldCB0b2tlbiB2YWx1ZSB0aGF0IG11c3QgYmUgcmVkYWN0ZWQ=",
			forbidden:   []string{"dGhpcyBpcyBhIHNlY3JldCB0b2tlbiB2YWx1ZSB0aGF0IG11c3QgYmUgcmVkYWN0ZWQ="},
			mustContain: []string{"bearer=[REDACTED]"},
		},
		{
			name:        "full pan",
			input:       "charge failed: card 1234-5678-9012-3456 declined",
			forbidden:   []string{"1234-5678-9012-3456", "1234567890123456"},
			mustContain: []string{"card [REDACTED] declined"},
		},
		{
			name:        "masked pan",
			input:       "charge failed: card ****-****-****-3456 declined",
			forbidden:   []string{"****-****-****-3456", "3456"},
			mustContain: []string{"card [REDACTED] declined"},
		},
		{
			name:        "phone formatted",
			input:       "login failed for +7 (999) 123-45-67: rate limit exceeded",
			forbidden:   []string{"+7 (999) 123-45-67", "9991234567"},
			mustContain: []string{"login failed for [REDACTED]: rate limit exceeded"},
		},
		{
			name:        "phone compact",
			input:       "login failed for +79991234567: rate limit exceeded",
			forbidden:   []string{"+79991234567"},
			mustContain: []string{"login failed for [REDACTED]: rate limit exceeded"},
		},
		{
			name:        "multiple secrets",
			input:       "error token=abc password=def card 1111-2222-3333-4444 phone +71112223344",
			forbidden:   []string{"token=abc", "password=def", "1111-2222-3333-4444", "+71112223344"},
			mustContain: []string{"token=[REDACTED]", "password=[REDACTED]", "card [REDACTED]", "phone [REDACTED]"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Error(errors.New(tc.input))
			for _, s := range tc.forbidden {
				if strings.Contains(got, s) {
					t.Errorf("sanitized output contains forbidden substring %q:\n%s", s, got)
				}
			}
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("sanitized output missing expected substring %q:\n%s", s, got)
				}
			}
		})
	}
}

func TestError_TruncatesLongErrors(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("something went wrong; ", maxSanitizedErrorLength/20+100)
	got := Error(errors.New(long))
	if len(got) > maxSanitizedErrorLength+20 {
		t.Errorf("sanitized output too long: got %d bytes, want at most %d", len(got), maxSanitizedErrorLength+20)
	}
	if !strings.HasSuffix(got, " [truncated]") {
		t.Errorf("sanitized output missing truncation marker: %q", got)
	}
}

func TestError_Nil(t *testing.T) {
	t.Parallel()

	if got := Error(nil); got != "" {
		t.Errorf("Error(nil) = %q, want empty string", got)
	}
}

func TestWrap_PreservesUnwrap(t *testing.T) {
	t.Parallel()

	original := errors.New("init failed: token=secret123")
	wrapped := Wrap(original, "init payment")
	if wrapped == nil {
		t.Fatal("Wrap non-nil error returned nil")
	}
	if !errors.Is(wrapped, original) {
		t.Error("errors.Is(wrapped, original) = false, want true")
	}
	if strings.Contains(wrapped.Error(), "secret123") {
		t.Errorf("wrapped error message contains sensitive data: %q", wrapped.Error())
	}
	if !strings.Contains(wrapped.Error(), "init payment:") {
		t.Errorf("wrapped error message missing context: %q", wrapped.Error())
	}
}

func TestWrap_Nil(t *testing.T) {
	t.Parallel()

	if got := Wrap(nil, "init payment"); got != nil {
		t.Errorf("Wrap(nil) = %v, want nil", got)
	}
}

func TestError_DoesNotRedactLegitimateIdentifiers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "uuid without dashes",
			input: "entity 1234567890abcdef1234567890abcdef not found",
		},
		{
			name:  "git sha",
			input: "commit a1b2c3d4e5f6789012345678901234567890abcd failed checks",
		},
		{
			name:  "long base64 opaque id",
			input: "id dGhpcyBpcyBhIHZlcnkgbG9uZyBiYXNlNjQgc3RyaW5nIHRoYXQgaXMgbm90IGEgc2VjcmV0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Error(errors.New(tc.input))
			if got != tc.input {
				t.Errorf("sanitized output unexpectedly changed:\ninput:  %s\noutput: %s", tc.input, got)
			}
		})
	}
}
