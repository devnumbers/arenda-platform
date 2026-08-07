package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LifecycleMailer bundles the dependencies every sharing lifecycle email needs
// (issue #162, T6): the mailer port, the user email resolver and the property
// title resolver. All methods are fire-and-forget: a missing recipient email
// (the user has none) skips the send, a title lookup failure degrades to a
// generic text, and a send failure is logged, never returned — the emails are
// post-commit (or in-transaction for the slot coordinator paths, whose callers
// own the commit) and must not roll back the business operation.
type LifecycleMailer struct {
	mailer AccessMailer
	emails UserEmailResolver
	titles PropertyTitleResolver
	logger *slog.Logger
}

// NewLifecycleMailer creates a LifecycleMailer. Any dependency may be nil: a
// nil mailer (or a nil LifecycleMailer passed to a service) disables all
// lifecycle emails, a nil resolver reads as "no email".
func NewLifecycleMailer(mailer AccessMailer, emails UserEmailResolver, titles PropertyTitleResolver, logger *slog.Logger) *LifecycleMailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &LifecycleMailer{mailer: mailer, emails: emails, titles: titles, logger: logger}
}

// SendAccessRevoked emails the former member that their access was revoked.
// Sent only when the revoked membership was active.
func (m *LifecycleMailer) SendAccessRevoked(ctx context.Context, userID, propertyID uuid.UUID) {
	m.sendToUser(ctx, userID, propertyID, "access_revoked", func(to, title string) error {
		return m.mailer.SendAccessRevoked(ctx, to, title)
	})
}

// SendAccessSuspended emails the member that their access waits for a free
// tariff slot (activation / unarchive without one).
func (m *LifecycleMailer) SendAccessSuspended(ctx context.Context, userID, propertyID uuid.UUID) {
	m.sendToUser(ctx, userID, propertyID, "access_suspended", func(to, title string) error {
		return m.mailer.SendAccessSuspended(ctx, to, title)
	})
}

// SendAccessRestored emails the member that a suspended membership became
// active again.
func (m *LifecycleMailer) SendAccessRestored(ctx context.Context, userID, propertyID uuid.UUID) {
	m.sendToUser(ctx, userID, propertyID, "access_restored", func(to, title string) error {
		return m.mailer.SendAccessRestored(ctx, to, title)
	})
}

// SendDowngradeSummary emails the recipient the single summary of the
// memberships suspended by one enforcement call (a tariff downgrade / grace
// expiry). It replaces the per-membership waiting email on this path.
func (m *LifecycleMailer) SendDowngradeSummary(ctx context.Context, userID uuid.UUID, propertyIDs []uuid.UUID) {
	if !m.enabled() || len(propertyIDs) == 0 {
		return
	}
	to := m.userEmail(ctx, userID, "downgrade_summary")
	if to == "" {
		return
	}
	titles := make([]string, 0, len(propertyIDs))
	for _, propertyID := range propertyIDs {
		if title := m.propertyTitle(ctx, propertyID, "downgrade_summary"); title != "" {
			titles = append(titles, title)
		}
	}
	if err := m.mailer.SendDowngradeSummary(ctx, to, titles); err != nil {
		m.logSendFailure(ctx, "downgrade_summary", userID, uuid.Nil, err)
	}
}

// SendInvitationActivated emails the property owner that an invited member
// activated their access at registration. memberEmail is the activated
// invitation's address — the owner already sees it in the member list while
// the invitation is pending.
func (m *LifecycleMailer) SendInvitationActivated(ctx context.Context, ownerID, propertyID uuid.UUID, memberEmail string) {
	if !m.enabled() {
		return
	}
	to := m.userEmail(ctx, ownerID, "invitation_activated")
	if to == "" {
		return
	}
	title := m.propertyTitle(ctx, propertyID, "invitation_activated")
	if err := m.mailer.SendInvitationActivated(ctx, to, title, memberEmail); err != nil {
		m.logSendFailure(ctx, "invitation_activated", ownerID, propertyID, err)
	}
}

// SendMemberLeft emails the property owner that a member left the object.
func (m *LifecycleMailer) SendMemberLeft(ctx context.Context, ownerID, propertyID uuid.UUID, memberName string) {
	if !m.enabled() {
		return
	}
	to := m.userEmail(ctx, ownerID, "member_left")
	if to == "" {
		return
	}
	title := m.propertyTitle(ctx, propertyID, "member_left")
	if err := m.mailer.SendMemberLeft(ctx, to, title, memberName); err != nil {
		m.logSendFailure(ctx, "member_left", ownerID, propertyID, err)
	}
}

// enabled reports whether lifecycle emails are active. A nil receiver disables
// them, which lets services treat the dependency as optional.
func (m *LifecycleMailer) enabled() bool {
	return m != nil && m.mailer != nil
}

// sendToUser is the shared body of the member-addressed lifecycle emails:
// resolve the recipient address and the property title, then send.
func (m *LifecycleMailer) sendToUser(ctx context.Context, userID, propertyID uuid.UUID, kind string, send func(to, title string) error) {
	if !m.enabled() {
		return
	}
	to := m.userEmail(ctx, userID, kind)
	if to == "" {
		return
	}
	title := m.propertyTitle(ctx, propertyID, kind)
	if err := send(to, title); err != nil {
		m.logSendFailure(ctx, kind, userID, propertyID, err)
	}
}

// userEmail resolves the recipient address; "" means "no email, skip".
func (m *LifecycleMailer) userEmail(ctx context.Context, userID uuid.UUID, kind string) string {
	if m.emails == nil {
		return ""
	}
	email, err := m.emails.GetEmail(ctx, userID)
	if err != nil {
		m.logger.WarnContext(ctx, "access: user email lookup for lifecycle email failed",
			slog.String("kind", kind),
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()))
		return ""
	}
	return email
}

// propertyTitle resolves the property display title; "" degrades to a generic
// text in the template rather than failing the send.
func (m *LifecycleMailer) propertyTitle(ctx context.Context, propertyID uuid.UUID, kind string) string {
	if m.titles == nil {
		return ""
	}
	title, err := m.titles.GetTitle(ctx, propertyID)
	if err != nil {
		m.logger.WarnContext(ctx, "access: property title lookup for lifecycle email failed",
			slog.String("kind", kind),
			slog.String("property_id", propertyID.String()),
			slog.String("error", err.Error()))
		return ""
	}
	return title
}

func (m *LifecycleMailer) logSendFailure(ctx context.Context, kind string, userID, propertyID uuid.UUID, err error) {
	attrs := []any{
		slog.String("kind", kind),
		slog.String("user_id", userID.String()),
		slog.String("error", err.Error()),
	}
	if propertyID != uuid.Nil {
		attrs = append(attrs, slog.String("property_id", propertyID.String()))
	}
	m.logger.ErrorContext(ctx, "access: lifecycle email send failed", attrs...)
}

// PropertyDeleteMailer implements the properties SharedMembersDeleteMailer port
// (issue #162, T6): it collects the former shared members' emails inside the
// property delete transaction (before the memberships are dropped) and sends
// the "object deleted" email after commit. Pending invitations are rows of a
// different table and never collected here, so unregistered invitees receive
// nothing.
type PropertyDeleteMailer struct {
	members MembershipRepository
	emails  UserEmailResolver
	mailer  AccessMailer
	logger  *slog.Logger
}

// NewPropertyDeleteMailer creates a PropertyDeleteMailer.
func NewPropertyDeleteMailer(members MembershipRepository, emails UserEmailResolver, mailer AccessMailer, logger *slog.Logger) *PropertyDeleteMailer {
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
				slog.String("user_id", membership.UserID.String()),
				slog.String("property_id", propertyID.String()),
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
