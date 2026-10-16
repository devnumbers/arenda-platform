// Package mailer defines the mailing port: the message model, template renderer and the Sender interface.
package mailer

import "context"

// Sender sends email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
