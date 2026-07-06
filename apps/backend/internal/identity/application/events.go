package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// UserRegistered is emitted after a new user is created during authentication.
type UserRegistered struct {
	UserID uuid.UUID
	Phone  domain.Phone
	Email  domain.Email
	At     time.Time
}

// EventPublisher publishes domain events.
// The initial implementation is an in-process dispatcher; a production deployment
// should replace it with an outbox-backed publisher for durability.
type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, event UserRegistered) error
}
