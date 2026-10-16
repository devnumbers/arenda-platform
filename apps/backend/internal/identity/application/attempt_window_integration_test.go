//go:build integration

package application_test

import (
	"errors"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestAttemptWindowIntegration_BlocksAfterMaxFailures proves the real PostgreSQL
// attempt-window rows enforce the 15-failure block: after MaxLoginFailures
// invalid verifications, a subsequent code send is rejected with ErrUserBlocked
// and a verify surfaces ErrTooManyAttempts. The failures are recorded across
// separate transactions that survive the rolled-back success paths (ADR 0033).
func TestAttemptWindowIntegration_BlocksAfterMaxFailures(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000100")
	email := mustEmail(t, "blocked@example.com")
	h.seedVerifiedUser(t, phone, email)
	ctx := h.ctx()

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	// Drive MaxLoginFailures invalid verifications. The last one surfaces
	// ErrTooManyAttempts once the threshold is reached.
	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.auth.VerifyCode(ctx, phone, &email, "000000", nil, application.DeviceContext{})
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last VerifyCode error = %v, want ErrTooManyAttempts", lastErr)
	}

	// The attempt window row in PostgreSQL records the threshold.
	if n := h.countFailedAttempts(t, phone); n != domain.MaxLoginFailures {
		t.Fatalf("failed attempts = %d, want %d", n, domain.MaxLoginFailures)
	}

	// A fresh code send is now blocked by the persisted window.
	err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin)
	if !errors.Is(err, application.ErrUserBlocked) {
		t.Fatalf("SendCode after block = %v, want ErrUserBlocked", err)
	}
}

// TestAttemptWindowIntegration_UnblocksAfterTTL proves the block clears once the
// 30-minute window expires: advancing the fake clock past LoginAttemptWindowTTL
// lifts the block, and a new send+verify cycle succeeds.
func TestAttemptWindowIntegration_UnblocksAfterTTL(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000101")
	email := mustEmail(t, "ttl@example.com")
	h.seedVerifiedUser(t, phone, email)
	ctx := h.ctx()

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	var lastErr error
	for range domain.MaxLoginFailures {
		_, _, lastErr = h.auth.VerifyCode(ctx, phone, &email, "000000", nil, application.DeviceContext{})
	}
	if !errors.Is(lastErr, domain.ErrTooManyAttempts) {
		t.Fatalf("last VerifyCode error = %v, want ErrTooManyAttempts", lastErr)
	}
	// Confirm the block is active.
	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); !errors.Is(err, application.ErrUserBlocked) {
		t.Fatalf("SendCode before TTL = %v, want ErrUserBlocked", err)
	}

	// Advance the clock past the window TTL. The persisted window is no longer
	// blocked at the new now, so a new code can be issued.
	h.clock.advance(domain.LoginAttemptWindowTTL + 1)

	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode after TTL = %v, want nil", err)
	}
	code := h.sender.lastCode(t)

	raw, _, err := h.auth.VerifyCode(ctx, phone, &email, code, nil, application.DeviceContext{})
	if err != nil {
		t.Fatalf("VerifyCode after TTL = %v", err)
	}
	if raw.Token == "" {
		t.Fatal("VerifyCode after TTL returned empty token")
	}

	// A successful verify resets the attempt window (DeleteByPhone), so no
	// failures remain for the phone.
	if n := h.countFailedAttempts(t, phone); n != 0 {
		t.Fatalf("failed attempts after successful verify = %d, want 0", n)
	}
}
