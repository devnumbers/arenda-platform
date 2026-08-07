// Package domain defines the audit log entry model and the action vocabulary.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ActorRole identifies who performed the action.
type ActorRole string

const (
	ActorRoleOwner     ActorRole = "owner"
	ActorRoleAdmin     ActorRole = "admin"
	ActorRoleSystem    ActorRole = "system"
	ActorRoleAnonymous ActorRole = "anonymous"
)

// Action is a dotted "<module>.<verb>" identifier of an auditable action.
type Action string

const (
	ActionAuthRegistered   Action = "auth.registered"
	ActionAuthLogin        Action = "auth.login"
	ActionAuthLoginFailed  Action = "auth.login_failed"
	ActionAuthLogout       Action = "auth.logout"
	ActionAuthLogoutAll    Action = "auth.logout_all"
	ActionAuthPhoneChanged Action = "auth.phone_changed"
	ActionProfileUpdated   Action = "profile.updated"

	ActionNotificationPreferencesUpdated Action = "notification_preferences.updated"

	ActionPropertyCreated        Action = "property.created"
	ActionPropertyUpdated        Action = "property.updated"
	ActionPropertyArchived       Action = "property.archived"
	ActionPropertyUnarchived     Action = "property.unarchived"
	ActionPropertyPhotoAdded     Action = "property.photo_added"
	ActionPropertyPhotoDeleted   Action = "property.photo_deleted"
	ActionPropertyContactCreated Action = "property_contact.created"
	ActionPropertyContactUpdated Action = "property_contact.updated"
	ActionPropertyContactDeleted Action = "property_contact.deleted"
	ActionPropertyDeleted        Action = "property.deleted"

	// ActionPropertyMemberAdded records a member granted shared access to a
	// property. Context never carries the member's email or phone (PII); only
	// ids and the role granted. See ADR 0020 and issue #156 (T3).
	ActionPropertyMemberAdded   Action = "property_member.added"
	ActionPropertyMemberUpdated Action = "property_member.updated"
	ActionPropertyMemberRemoved Action = "property_member.removed"
	ActionPropertyMemberLeft    Action = "property_member.left"
	// ActionPropertyMemberSuspended records a membership moved to the suspended
	// state by the slot coordinator (recipient tariff downgrade / grace expiry /
	// activation without a free slot). Recorded by the system actor; Context
	// never carries the member's PII. See issue #158 (T4).
	ActionPropertyMemberSuspended Action = "property_member.suspended"
	// ActionPropertyMemberReactivated records a suspended membership recovered
	// FIFO by the slot coordinator when a recipient slot frees up (revoke,
	// self-exit, owner archive/delete, recipient upgrade). Recorded by the
	// system actor. See issue #158 (T4).
	ActionPropertyMemberReactivated Action = "property_member.reactivated"

	// ActionPropertyMemberInvitationInvited records a pending email invitation
	// created for a property (issue #161, T5). Invitation lifecycle context
	// never carries the invitee's email (PII, ADR 0020); only ids and the role.
	ActionPropertyMemberInvitationInvited Action = "property_member_invitation.invited"
	// ActionPropertyMemberInvitationResent records a manual invite email resend.
	ActionPropertyMemberInvitationResent Action = "property_member_invitation.resent"
	// ActionPropertyMemberInvitationRoleChanged records a role change on a
	// pending invitation (no new email is sent).
	ActionPropertyMemberInvitationRoleChanged Action = "property_member_invitation.role_changed"
	// ActionPropertyMemberInvitationCancelled records a pending invitation
	// cancelled silently by a manager.
	ActionPropertyMemberInvitationCancelled Action = "property_member_invitation.cancelled"
	// ActionPropertyMemberInvitationActivated records a pending invitation
	// turned into a membership when the invitee registered with the matching
	// email. Recorded by the system actor.
	ActionPropertyMemberInvitationActivated Action = "property_member_invitation.activated"

	ActionLeaseCreated   Action = "lease.created"
	ActionLeaseUpdated   Action = "lease.updated"
	ActionLeaseCompleted Action = "lease.completed"

	ActionTenantContactCreated Action = "tenant_contact.created"
	ActionTenantContactUpdated Action = "tenant_contact.updated"

	ActionOperationCreated          Action = "operation.created"
	ActionOperationUpdated          Action = "operation.updated"
	ActionOperationDeleted          Action = "operation.deleted"
	ActionOperationCompleted        Action = "operation.completed"
	ActionOperationMarkedIncomplete Action = "operation.marked_incomplete"
	// ActionOperationMoved records an operation moved to another property of the
	// same data owner (issue #166). Context carries from_property_id and
	// to_property_id.
	ActionOperationMoved Action = "operation.moved"

	ActionRecurringOperationCreated Action = "recurring_operation.created"
	ActionRecurringOperationUpdated Action = "recurring_operation.updated"
	ActionRecurringOperationDeleted Action = "recurring_operation.deleted"
	ActionRecurringOperationPaused  Action = "recurring_operation.paused"
	ActionRecurringOperationResumed Action = "recurring_operation.resumed"

	ActionOperationCategoryCreated Action = "operation_category.created"

	ActionSubscriptionTariffChanged    Action = "subscription.tariff_changed"
	ActionSubscriptionCancelled        Action = "subscription.cancelled"
	ActionSubscriptionAutoRenewToggled Action = "subscription.auto_renew_toggled"

	ActionPaymentMethodAdded     Action = "payment_method.added"
	ActionPaymentMethodActivated Action = "payment_method.activated"
	ActionPaymentMethodDeleted   Action = "payment_method.deleted"

	ActionSubscriptionPaymentSucceeded Action = "subscription_payment.succeeded"
	ActionSubscriptionPaymentFailed    Action = "subscription_payment.failed"
	ActionSubscriptionPaymentRefunded  Action = "subscription_payment.refunded"
	ActionSubscriptionPaymentSynced    Action = "subscription_payment.synced"
)

// EntityType identifies the kind of entity the action targets.
type EntityType string

const (
	EntityUser            EntityType = "user"
	EntityProperty        EntityType = "property"
	EntityPropertyPhoto   EntityType = "property_photo"
	EntityPropertyContact EntityType = "property_contact"
	EntityPropertyMember  EntityType = "property_member"
	// EntityPropertyMemberInvitation is a pending email invitation to shared
	// access (issue #161, T5).
	EntityPropertyMemberInvitation EntityType = "property_member_invitation"
	EntityLease                    EntityType = "lease"
	EntityTenantContact            EntityType = "tenant_contact"
	EntityOperation                EntityType = "operation"
	EntityRecurringOperation       EntityType = "recurring_operation"
	EntityOperationCategory        EntityType = "operation_category"
	EntitySubscription             EntityType = "subscription"
	EntityPaymentMethod            EntityType = "payment_method"
	EntitySubscriptionPayment      EntityType = "subscription_payment"
)

// Entry is a single audit log record.
type Entry struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	ActorID    *uuid.UUID // nil = system/anonymous
	ActorRole  ActorRole
	Action     Action
	EntityType EntityType     // "" = none
	EntityID   *uuid.UUID     // nil = none
	Context    map[string]any // whitelist fields only, never PII/secrets
	RequestID  string
	IP         string
}
