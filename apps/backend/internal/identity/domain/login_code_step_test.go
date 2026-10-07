package domain

import "testing"

func TestLoginCodeStep_CanonicalValues(t *testing.T) {
	t.Parallel()
	// The canonical strings surface in delivery logs and select the letter in
	// the email adapter; renaming them silently changes observable behaviour.
	if got := string(LoginCodeStepCurrentEmail); got != "current_email" {
		t.Fatalf("LoginCodeStepCurrentEmail = %q, want current_email", got)
	}
	if got := string(LoginCodeStepNewEmail); got != "new_email" {
		t.Fatalf("LoginCodeStepNewEmail = %q, want new_email", got)
	}
	if got := LoginCodeStepCurrentEmail.String(); got != "current_email" {
		t.Fatalf("LoginCodeStepCurrentEmail.String() = %q, want current_email", got)
	}
	if got := LoginCodeStepNewEmail.String(); got != "new_email" {
		t.Fatalf("LoginCodeStepNewEmail.String() = %q, want new_email", got)
	}
}
