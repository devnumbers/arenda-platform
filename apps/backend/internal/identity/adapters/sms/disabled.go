package sms

import (
	"context"
	"errors"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

var ErrDisabled = errors.New("sms sender is disabled")

// DisabledSender rejects SMS sends without logging verification codes.
type DisabledSender struct{}

func NewDisabledSender() *DisabledSender {
	return &DisabledSender{}
}

func (s *DisabledSender) Send(ctx context.Context, phone domain.Phone, message string) error {
	return ErrDisabled
}
