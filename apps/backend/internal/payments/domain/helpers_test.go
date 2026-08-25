package domain

import (
	"github.com/google/uuid"
)

// uuidMustV7 mints a UUIDv7 for domain test fixtures (ADR 0019; the panicking
// form is sanctioned for fixtures).
func uuidMustV7() uuid.UUID { return uuid.Must(uuid.NewV7()) }
