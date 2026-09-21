package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyDeleteMailer implements the properties SharedMembersDeleteMailer port
// (issue #162, T6): it collects the former shared members' emails inside the
// property delete transaction (before the memberships are dropped) and sends
// the "object deleted" email after commit. The email stays direct — the
// object's deletion is not in the notifications catalog (карта #734, #751),
// and the members' own feed rows could not outlive the deleted property's
// context anyway. Pending invitations are rows of a different table and never
// collected here, so unregistered invitees receive nothing.
type PropertyDeleteMailer struct {
	members MembershipRepository
	emails  UserEmailResolver
	mailer  AccessMailer
	logger  *slog.Logger
}

// NewPropertyDeleteMailer creates a PropertyDeleteMailer.
func NewPropertyDeleteMailer(
	members MembershipRepository,
	emails UserEmailResolver,
	mailer AccessMailer,
	logger *slog.Logger,
) *PropertyDeleteMailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &PropertyDeleteMailer{members: members, emails: emails, mailer: mailer, logger: logger}
}

// CollectFormerMemberEmails returns the email addresses of the property's
// members with an active or suspended membership. Members without an email are
// skipped. It must be called inside the delete transaction, before the slot
// coordinator drops the memberships.
func (m *PropertyDeleteMailer) CollectFormerMemberEmails(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) ([]string, error) {
	memberships, err := m.members.WithTx(tx).ListByProperty(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list memberships by property: %w", err)
	}
	emails := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		email, err := m.emails.GetEmail(ctx, membership.UserID)
		if err != nil {
			m.logger.WarnContext(ctx, "access: member email lookup for property deleted email failed",
				slog.String(auditKeyUserID, membership.UserID.String()),
				slog.String(auditKeyPropertyID, propertyID.String()),
				slog.String("error", err.Error()))
			continue
		}
		if email != "" {
			emails = append(emails, email)
		}
	}
	return emails, nil
}

// SendPropertyDeleted sends one "object deleted" email. The caller (the
// properties service) invokes it after the delete commits and logs a failure;
// it must not roll anything back.
func (m *PropertyDeleteMailer) SendPropertyDeleted(ctx context.Context, to, propertyTitle string) error {
	return m.mailer.SendPropertyDeleted(ctx, to, propertyTitle)
}
