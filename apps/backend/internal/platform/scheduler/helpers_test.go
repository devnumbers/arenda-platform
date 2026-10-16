package scheduler

import "time"

// testSensitivePayload is a fixed string with credential-shaped fragments
// shared by the worker tests that assert log sanitization.
const (
	testSensitivePayload = "token=secret123 card 1234-5678-9012-3456 phone +79991234567"
	testSecretToken      = "token=secret123"
	testCardNumber       = "1234-5678-9012-3456"
	testUserPhone        = "+79991234567"
)

// fakeClockForWorker is a fixed clock shared by the worker tests.
type fakeClockForWorker struct{ now time.Time }

func (c fakeClockForWorker) Now() time.Time { return c.now }
