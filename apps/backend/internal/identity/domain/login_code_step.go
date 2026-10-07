package domain

// LoginCodeStep marks which letter of an operation carries the code — the
// address role the code is delivered to. Login and phone-change codes always
// travel to the user's current email; the email-change protocol adds a second
// letter to the pending new address. Together with LoginCodePurpose the step
// selects the letter's unique template and subject in the delivery adapter
// (issue #1204); it never takes part in the code hash — the hash already
// binds purpose, phone, and email.
type LoginCodeStep string

const (
	// LoginCodeStepCurrentEmail is the letter to the user's current email:
	// the only letter of login and phone change, and the "confirm it is you"
	// letter of email change.
	LoginCodeStepCurrentEmail LoginCodeStep = "current_email"
	// LoginCodeStepNewEmail is the letter to the pending new address: the
	// "confirm ownership of the address" letter of email change.
	LoginCodeStepNewEmail LoginCodeStep = "new_email"
)

// String returns the string representation of the step.
func (s LoginCodeStep) String() string {
	return string(s)
}
