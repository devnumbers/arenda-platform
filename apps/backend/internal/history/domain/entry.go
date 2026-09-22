// Package domain defines the action journal model and the action vocabulary
// of the «История» context (ADR 0061): the product feed of manual user
// actions, deliberately separate from the admin audit log.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kind is the action kind — one of the seven mockup filter groups
// (ADR 0061 §4). It doubles as the link kind of a segment: the frontend
// routes a linked run by the same vocabulary.
type Kind string

const (
	KindProperty  Kind = "property"
	KindRental    Kind = "rental"
	KindPayment   Kind = "payment"
	KindOperation Kind = "operation"
	KindContact   Kind = "contact"
	KindTask      Kind = "task"
	KindMember    Kind = "member"
)

// BaseAction is the coarse action group — one of the four filter groups of
// the feed, each with its own icon and color (ADR 0061 §4).
type BaseAction string

const (
	BaseAdded     BaseAction = "added"
	BaseChanged   BaseAction = "changed"
	BaseCompleted BaseAction = "completed"
	BaseDeleted   BaseAction = "deleted"
)

// ActorRole is the role the actor held on the property at action time
// (the shared/policy vocabulary: owner | full_access | viewer, the audit
// pattern). Recorded verbatim; never re-resolved.
type ActorRole string

const (
	ActorRoleOwner      ActorRole = "owner"
	ActorRoleFullAccess ActorRole = "full_access"
	ActorRoleViewer     ActorRole = "viewer"
)

// Action is a stable dotted id of a recorded action (ADR 0061 §4). The ids
// are API surface: the frontend maps them to icons and filters, so a renamed
// action id is a breaking change, not a copy tweak.
type Action string

const (
	ActionPropertyCreated            Action = "property.created"
	ActionPropertyRenamed            Action = "property.renamed"
	ActionPropertyAddressChanged     Action = "property.address_changed"
	ActionPropertyDescriptionChanged Action = "property.description_changed"
	ActionPropertyAttributesChanged  Action = "property.attributes_changed"
	// ActionPropertyUpdated records a mutation that touched several field
	// groups at once (rename + address in one PATCH); single-group edits get
	// their precise ids above.
	ActionPropertyUpdated Action = "property.updated"

	ActionPropertyPhotoAdded   Action = "property.photo_added"
	ActionPropertyPhotoDeleted Action = "property.photo_deleted"
	ActionPropertyPinned       Action = "property.pinned"
	ActionPropertyUnpinned     Action = "property.unpinned"
	ActionPropertyArchived     Action = "property.archived"
	ActionPropertyUnarchived   Action = "property.unarchived"

	ActionRentalCreated   Action = "rental.created"
	ActionRentalUpdated   Action = "rental.updated"
	ActionRentalCompleted Action = "rental.completed"
	ActionRentalDeleted   Action = "rental.deleted"

	ActionPaymentCreated Action = "payment.created"
	ActionPaymentUpdated Action = "payment.updated"
	ActionPaymentDeleted Action = "payment.deleted"
	ActionPaymentPaused  Action = "payment.paused"
	ActionPaymentResumed Action = "payment.resumed"

	ActionOperationCreated Action = "operation.created"
	ActionOperationPaid    Action = "operation.paid"
	ActionOperationDeleted Action = "operation.deleted"

	ActionContactCreated Action = "contact.created"
	ActionContactUpdated Action = "contact.updated"
	ActionContactDeleted Action = "contact.deleted"

	ActionTaskRuleCreated Action = "task_rule.created"
	ActionTaskRuleUpdated Action = "task_rule.updated"
	ActionTaskRuleDeleted Action = "task_rule.deleted"
	ActionTaskCompleted   Action = "task.completed"
	ActionTaskUncompleted Action = "task.uncompleted"
	// ActionTaskCompletedCleared records «Удалить все выполненные» — one row
	// per property, the removed count in the text (ADR 0061 §3 bulk canon).
	ActionTaskCompletedCleared Action = "task.completed_cleared"

	ActionMemberInvited             Action = "member.invited"
	ActionMemberAdded               Action = "member.added"
	ActionMemberRoleChanged         Action = "member.role_changed"
	ActionMemberRemoved             Action = "member.removed"
	ActionMemberInvitationCancelled Action = "member.invitation_cancelled"
	ActionMemberLeft                Action = "member.left"
	// ActionMemberParticipantRemoved records one property leg of the bulk
	// participant removal («Отозвать и удалить», #694): the multi-object
	// action writes one row per affected property (ADR 0061 §3).
	ActionMemberParticipantRemoved Action = "member.participant_removed"
)

// Link points a segment run at an entity page (ticket #713). A deleted
// entity leaves a row without a link — the builders simply omit it.
type Link struct {
	Kind Kind      `json:"kind"`
	ID   uuid.UUID `json:"id"`
}

// Segment is one run of the server-built row text (ADR 0061 §6): the server
// is the single source of the row copy; the frontend renders the runs
// verbatim and wraps the linked ones. Runs carry their own spacing — the
// visible sentence is the exact concatenation of the texts.
type Segment struct {
	Text string `json:"text"`
	Link *Link  `json:"link,omitempty"`
}

// Segments is the ordered row text.
type Segments []Segment

// PlainText is the searchable concatenation of the runs — what the row
// visually says is exactly what the search matches (ADR 0061 §6).
func (s Segments) PlainText() string {
	var b strings.Builder
	for _, seg := range s {
		b.WriteString(seg.Text)
	}
	return b.String()
}

// Entry is a single action journal record — append-only, never re-resolved
// (ADR 0061). ActorName/ActorEmail are the action-time snapshots; the
// recorder service fills them from the actor snapshot source when empty.
// Searchable is materialized at write time by the recorder service; the
// search_tsv column is generated by the database.
type Entry struct {
	ID         uuid.UUID
	PropertyID uuid.UUID
	ActorID    *uuid.UUID
	ActorRole  ActorRole
	ActorName  string
	ActorEmail string
	Kind       Kind
	Action     Action
	BaseAction BaseAction
	Segments   Segments
	// Searchable is the row's plain text plus the actor name and email —
	// computed by the recorder service, never handed in by callers.
	Searchable string
	// Context carries the structured extras (amounts in kopecks, dates,
	// old/new values) for future use; never rendered as the row text.
	Context   map[string]any
	CreatedAt time.Time
}

// ActorSnapshot is the actor's display name and email at action time —
// the PII the journal is allowed to keep (ADR 0061 §5).
type ActorSnapshot struct {
	Name  string
	Email string
}

// The context jsonb keys the calling modules set on top of the catalog
// builders (ADR 0061 §5: the structured extras live in the context, never
// in the row text).
const (
	// CtxKeyRole is the role granted on a membership/invitation row.
	CtxKeyRole = "role"
	// CtxKeyUserID is the target user of a membership action.
	CtxKeyUserID = "user_id"
	// CtxKeyAmountKopecks is an amount in kopecks (money is BIGINT kopecks
	// everywhere; the context carries it for the future read model).
	CtxKeyAmountKopecks = "amount_kopecks"
)
