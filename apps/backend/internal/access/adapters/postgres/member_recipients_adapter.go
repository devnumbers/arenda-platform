package postgres

import (
	"context"

	"github.com/google/uuid"
)

// MemberRecipientAdapter adapts the membership repository to the notifications
// application.PropertyRecipientLister port (issue #159): the reminder worker
// fans property reminders out to the owner and to every active member. The
// compile-time check against the notifications port lives in cmd/api/wire (the
// only layer allowed to depend on several bounded contexts at once), so this
// package does not import the notifications context.
type MemberRecipientAdapter struct {
	members *MembershipRepository
}

// NewMemberRecipientAdapter creates a MemberRecipientAdapter over the
// membership repository.
func NewMemberRecipientAdapter(members *MembershipRepository) *MemberRecipientAdapter {
	return &MemberRecipientAdapter{members: members}
}

// ListActiveRecipientIDs returns the user ids of the property's active
// members. Suspended memberships (hidden from the recipient due to a tariff
// slot shortage, issue #158) are excluded: they receive nothing.
func (a *MemberRecipientAdapter) ListActiveRecipientIDs(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	memberships, err := a.members.ListByProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(memberships))
	for _, m := range memberships {
		if m.IsSuspended() {
			continue
		}
		out = append(out, m.UserID)
	}
	return out, nil
}
