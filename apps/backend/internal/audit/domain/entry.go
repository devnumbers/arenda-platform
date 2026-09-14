// Package domain defines the audit log entry model and the action vocabulary.
package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// ActorRole identifies who performed the action.
type ActorRole string

const (
	ActorRoleOwner     ActorRole = "owner"
	ActorRoleAdmin     ActorRole = "admin"
	ActorRoleSystem    ActorRole = "system"
	ActorRoleAnonymous ActorRole = "anonymous"
	// ActorRoleFullAccess and ActorRoleViewer attribute property-scoped actions
	// to a shared-access member instead of masking them as the owner's own
	// (Property Sharing follow-up, issue #166). The strings are identical to
	// the shared/policy Role values; the mapping lives in each calling module
	// because audit must not depend on shared/policy. Suspended/none actors
	// never reach a write path, so they have no ActorRole.
	ActorRoleFullAccess ActorRole = "full_access"
	ActorRoleViewer     ActorRole = "viewer"
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
	// ActionAuthPhoneChangeFailed records a failed phone-change verification
	// attempt, mirroring ActionAuthLoginFailed for the login flow. Closes the
	// audit gap where phone-change failures left no trail while the success
	// path was fully audited.
	ActionAuthPhoneChangeFailed Action = "auth.phone_change_failed"
	ActionProfileUpdated        Action = "profile.updated"

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

	// ActionPaymentCreated and its neighbours below record the Payments
	// context user mutations (ADR 0049 §5, ticket #457): in-tx fail-safe; the
	// bulk tick never writes audit (the extended ADR 0020 gap). The old
	// recurring_operation.* vocabulary above belongs to the removed finance
	// context; payment.* is the Payments context of ADR 0047.
	ActionPaymentCreated Action = "payment.created"
	ActionPaymentUpdated Action = "payment.updated"
	ActionPaymentDeleted Action = "payment.deleted"
	ActionPaymentPaused  Action = "payment.paused"
	ActionPaymentResumed Action = "payment.resumed"
	// ActionOperationPaid records the manual «Оплатить сейчас» of an
	// operation (ticket #461): planned → paid with paid_date = today in the
	// owner's timezone. The auto-pay day payment is tick bulk, never audited
	// (the extended ADR 0020 gap); it has no action here.
	ActionOperationPaid Action = "operation.paid"

	// ActionTaskRuleCreated and its neighbours record the Tasks context user
	// mutations (ADR 0051, tickets #496/#498): in-tx fail-safe; the bulk tick
	// never writes audit (the extended ADR 0020 gap). Rule deletion is hard —
	// the completed journal survives with snapshots (resolution #496).
	ActionTaskRuleCreated Action = "task_rule.created"
	ActionTaskRuleUpdated Action = "task_rule.updated"
	ActionTaskRuleDeleted Action = "task_rule.deleted"
	// ActionTaskCompleted and ActionTaskUncompleted record the manual
	// completion toggle: completed_date = today in the owner's timezone, and
	// its revert — possible only while the rule lives (resolution #496).
	ActionTaskCompleted   Action = "task.completed"
	ActionTaskUncompleted Action = "task.uncompleted"
	// ActionTaskCompletedCleared records «Удалить все выполненные»
	// (resolution #497): the completed tasks of the property's deleted rules
	// removed forever; Context carries the removed count, never titles.
	ActionTaskCompletedCleared Action = "task.completed_cleared"

	// ActionContactCreated and its neighbours record the Contacts context
	// user mutations (ADR 0054, ticket #506): in-tx fail-safe. The context
	// never carries the contact's PII (names, phone, email) — ids and field
	// names only. The property_contact.* vocabulary above belongs to the
	// demolished ADR 0026 surface and stays for the historical audit rows.
	ActionContactCreated Action = "contact.created"
	ActionContactUpdated Action = "contact.updated"
	ActionContactDeleted Action = "contact.deleted"

	// ActionRentalCreated and its neighbours record the Rentals context user
	// mutations (ADR 0053 §3, ticket #529): in-tx fail-safe. There is no
	// rental.extended — «продление» is a UI scenario of the planned-end edit
	// (rental.updated). The sync of the managed payment is audited by the
	// rental action, not twice as payment.*.
	ActionRentalCreated   Action = "rental.created"
	ActionRentalUpdated   Action = "rental.updated"
	ActionRentalCompleted Action = "rental.completed"
	ActionRentalDeleted   Action = "rental.deleted"

	ActionSubscriptionTariffChanged    Action = "subscription.tariff_changed"
	ActionSubscriptionCancelled        Action = "subscription.cancelled"
	ActionSubscriptionResumed          Action = "subscription.resumed"
	ActionSubscriptionAutoRenewToggled Action = "subscription.auto_renew_toggled"
	// ActionSubscriptionServiceAssigned and its neighbours below are the
	// admin subscription operations of issue #255: service assignment, force
	// tariff change and grace extension. The admin cancel on the user's
	// behalf reuses ActionSubscriptionCancelled with the admin actor.
	ActionSubscriptionServiceAssigned Action = "subscription.service_assigned"
	ActionSubscriptionTariffForced    Action = "subscription.tariff_forced"
	ActionSubscriptionGraceExtended   Action = "subscription.grace_extended"
	// ActionSubscriptionTimeShifted is the stand-only time-travel shift of
	// the subscription lifecycle (issue #665): the rig moves the temporal
	// boundaries for the acceptance scenarios and audits every move.
	ActionSubscriptionTimeShifted Action = "subscription.time_shifted"

	ActionPaymentMethodAdded     Action = "payment_method.added"
	ActionPaymentMethodActivated Action = "payment_method.activated"
	ActionPaymentMethodDeleted   Action = "payment_method.deleted"

	ActionSubscriptionPaymentSucceeded Action = "subscription_payment.succeeded"
	ActionSubscriptionPaymentFailed    Action = "subscription_payment.failed"
	ActionSubscriptionPaymentRefunded  Action = "subscription_payment.refunded"
	ActionSubscriptionPaymentSynced    Action = "subscription_payment.synced"

	// ActionTariffCreated and ActionTariffUpdated are the admin tariff
	// management operations (issue #256): creating a plan and editing its
	// prices, property limit or activity (hiding included). Context carries
	// the resulting field values, never user data.
	ActionTariffCreated Action = "tariff.created"
	ActionTariffUpdated Action = "tariff.updated"
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
	// EntityPayment is a payment rule of the Payments context (ADR 0047):
	// the rule itself, not its materialized operations.
	EntityPayment EntityType = "payment"
	// EntityTaskRule is a task rule of the Tasks context (ADR 0051); the
	// EntityTask covers both its materialized occurrences and the completed
	// journal (the journal clear has no single entity id — Context.count).
	EntityTaskRule EntityType = "task_rule"
	EntityTask     EntityType = "task"
	// EntityContact is a contact card of the Contacts context (ADR 0054).
	EntityContact EntityType = "contact"
	// EntityRental is a rental of the Rentals context (ADR 0053): the
	// occupancy period with its terms; its managed payment audits through
	// the rental actions, not as a second payment.* entry.
	EntityRental              EntityType = "rental"
	EntitySubscription        EntityType = "subscription"
	EntityPaymentMethod       EntityType = "payment_method"
	EntitySubscriptionPayment EntityType = "subscription_payment"
	EntityTariff              EntityType = "tariff"
)

// Entry is a single audit log record.
type Entry struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	ActorID    *uuid.UUID // System or anonymous when nil.
	ActorRole  ActorRole
	Action     Action
	EntityType EntityType     // None when empty.
	EntityID   *uuid.UUID     // None when nil.
	Context    map[string]any // Whitelist fields only, never PII/secrets.
	RequestID  string
	IP         string
}

// Actor identifies who performed an action, for audit recording. It carries
// just enough identity (user ID + role) for the application layer to record
// audit without depending on transport context or the full identity aggregate.
type Actor struct {
	ID   uuid.UUID
	Role ActorRole
}

// ActorRoleFromRole maps a shared-kernel actor.Role to an audit ActorRole.
// Unknown or future roles map to Anonymous rather than masking as Owner,
// so a corrupt or unexpected value stays visible in the audit trail.
func ActorRoleFromRole(role actor.Role) ActorRole {
	switch role {
	case actor.RoleOwner:
		return ActorRoleOwner
	case actor.RoleAdmin:
		return ActorRoleAdmin
	default:
		return ActorRoleAnonymous
	}
}
