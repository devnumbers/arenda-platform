package mailer

import "context"

// Sender sends email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
