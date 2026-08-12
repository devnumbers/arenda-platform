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
	email, err := NewEmail("owner@example.com")
	if err != nil {
		t.Fatalf("NewEmail: %v", err)
	}
	c, err := NewLoginCode(
		PhoneFrom("+79001234567"),
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
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	hash := validHexHash("123456")
	otherHash := validHexHash("999999") // different value, same length

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
			input:   validHexHash("x")[:8], // 4 bytes vs 32 bytes expected
			now:     now,
			wantErr: ErrLoginCodeInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

func TestLoginCode_VerifyAndUse(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	hash := validHexHash("123456")

	t.Run("marks used only on success", func(t *testing.T) {
		c := newValidCode(t, hash, now)

		if err := c.VerifyAndUse(hash, now); err != nil {
			t.Fatalf("VerifyAndUse error = %v, want nil", err)
		}
		if !c.Used {
			t.Fatal("Used = false, want true after successful VerifyAndUse")
		}
	})

	t.Run("does not mark used on wrong code", func(t *testing.T) {
		c := newValidCode(t, hash, now)

		err := c.VerifyAndUse(validHexHash("999999"), now)
		if !errors.Is(err, ErrLoginCodeInvalid) {
			t.Fatalf("VerifyAndUse error = %v, want ErrLoginCodeInvalid", err)
		}
		if c.Used {
			t.Fatal("Used = true, want false after failed VerifyAndUse")
		}
	})

	t.Run("does not mark used on expired code", func(t *testing.T) {
		c := newValidCode(t, hash, now)

		err := c.VerifyAndUse(hash, now.Add(LoginCodeTTL+time.Second))
		if !errors.Is(err, ErrLoginCodeInvalid) {
			t.Fatalf("VerifyAndUse error = %v, want ErrLoginCodeInvalid", err)
		}
		if c.Used {
			t.Fatal("Used = true, want false after failed VerifyAndUse")
		}
	})
}
