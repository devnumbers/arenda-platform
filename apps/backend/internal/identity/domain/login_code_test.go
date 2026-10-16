package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// validHexHash hashes s with sha256 and returns a hex string that
// LoginCode.Verify can hex-decode, mirroring encryption.hashToken.
func validHexHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func newValidCode(t *testing.T, codeHash string, now time.Time) LoginCode {
	t.Helper()
	phone, err := NewPhone("+79001234567")
	if err != nil {
		t.Fatalf("NewPhone: %v", err)
	}
	email, err := NewEmail("owner@example.com")
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}
	c, err := NewLoginCode(
		phone,
		email,
		codeHash,
		LoginCodePurposeLogin,
		nil,
		now,
	)
	if err != nil {
		t.Fatalf("NewLoginCode: %v", err)
	}
	return c
}

func TestLoginCode_Verify(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	hash := validHexHash("123456")
	otherHash := validHexHash("999999") // Different value, same length.

	tests := []struct {
		name     string
		mutate   func(c *LoginCode)
		input    string
		now      time.Time
		wantErr  error
		wantUsed bool
	}{
		{
			name:    "valid code within TTL",
			mutate:  func(c *LoginCode) {},
			input:   hash,
			now:     now.Add(LoginCodeTTL - time.Second),
			wantErr: nil,
		},
		{
			name:    "expired after TTL",
			mutate:  func(c *LoginCode) {},
			input:   hash,
			now:     now.Add(LoginCodeTTL + time.Second),
			wantErr: ErrLoginCodeInvalid,
		},
		{
			name:    "already used",
			mutate:  func(c *LoginCode) { c.Used = true },
			input:   hash,
			now:     now,
			wantErr: ErrLoginCodeInvalid,
		},
		{
			name:    "wrong hash value (constant-time compare)",
			mutate:  func(c *LoginCode) {},
			input:   otherHash,
			now:     now,
			wantErr: ErrLoginCodeInvalid,
		},
		{
			name:    "malformed hex input (odd length)",
			mutate:  func(c *LoginCode) {},
			input:   "abc",
			now:     now,
			wantErr: ErrLoginCodeInvalid,
		},
		{
			name:    "stored code hash is malformed hex",
			mutate:  func(c *LoginCode) { c.CodeHash = "zz" },
			input:   hash,
			now:     now,
			wantErr: ErrLoginCodeInvalid,
		},
		{
			name:    "hash length mismatch (short input decodes, wrong length)",
			mutate:  func(c *LoginCode) {},
			input:   validHexHash("x")[:8], // 4 bytes vs 32 bytes expected.
			now:     now,
			wantErr: ErrLoginCodeInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newValidCode(t, hash, now)
			tc.mutate(&c)
			usedBefore := c.Used

			err := c.Verify(tc.input, tc.now)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Verify error = %v, want nil", err)
				}
			} else {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Verify error = %v, want %v", err, tc.wantErr)
				}
			}
			// Verify must never flip the Used flag.
			if c.Used != usedBefore {
				t.Fatalf("Verify changed Used: was %v, now %v (Verify must be side-effect free)", usedBefore, c.Used)
			}
		})
	}
}
