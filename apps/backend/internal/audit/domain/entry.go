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

	ActionPropertyCreated      Action = "property.created"
	ActionPropertyUpdated      Action = "property.updated"
	ActionPropertyArchived     Action = "property.archived"
	ActionPropertyUnarchived   Action = "property.unarchived"
	ActionPropertyPhotoAdded   Action = "property.photo_added"
	ActionPropertyPhotoDeleted Action = "property.photo_deleted"
	ActionPropertyDeleted      Action = "property.deleted"

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
	EntityUser                EntityType = "user"
	EntityProperty            EntityType = "property"
	EntityPropertyPhoto       EntityType = "property_photo"
	EntityLease               EntityType = "lease"
	EntityTenantContact       EntityType = "tenant_contact"
	EntityOperation           EntityType = "operation"
	EntityRecurringOperation  EntityType = "recurring_operation"
	EntityOperationCategory   EntityType = "operation_category"
	EntitySubscription        EntityType = "subscription"
	EntityPaymentMethod       EntityType = "payment_method"
	EntitySubscriptionPayment EntityType = "subscription_payment"
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
