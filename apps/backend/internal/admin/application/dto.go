package application

import (
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	propertiesdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// AdminUserView is a cross-user read-only view of an account.
type AdminUserView struct {
	ID                 uuid.UUID
	Phone              string
	Role               string
	Name               *string
	Surname            *string
	Patronymic         *string
	Email              *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	SubscriptionStatus string
}

// AdminUserStats aggregates counts for the user detail view.
type AdminUserStats struct {
	ActivePropertiesCount   int64
	ArchivedPropertiesCount int64
	LeasesCount             int64
	OperationsCount         int64
	TenantContactsCount     int64
}

// AdminUserDetailView returns a user together with their subscription and stats.
type AdminUserDetailView struct {
	User         AdminUserView
	Stats        AdminUserStats
	Subscription *billingapp.SubscriptionView
}

// AdminPropertyView is a read-only view of a property for admin operations.
type AdminPropertyView struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	OwnerPhone  string
	Name        string
	Type        propertiesdomain.PropertyType
	Address     string
	Description *string
	Attributes  propertiesdomain.Attributes
	Status      propertiesdomain.PropertyStatus
	Photos      []propertiesdomain.Photo
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AdminLeaseView is a read-only view of a lease for admin operations.
type AdminLeaseView struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           *uuid.UUID
	PropertyName         *string
	TenantContactID      *uuid.UUID
	TenantContact        *leasesdomain.TenantContact
	Status               leasesdomain.LeaseStatus
	StartDate            time.Time
	EndDate              *time.Time
	RentAmountKopecks    int64
	DepositAmountKopecks int64
	PaymentDay           int
	Comment              string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AdminTenantContactView is a read-only view of a tenant contact for admin operations.
// OwnerPhone carries the decrypted phone of the owning user; it is empty when
// the contact is embedded in another view (e.g. a lease) without a users join.
type AdminTenantContactView struct {
	leasesdomain.TenantContact
	OwnerPhone string
}

// AdminPropertyContactView is a read-only view of a property contact for admin operations.
type AdminPropertyContactView struct {
	propertiesdomain.PropertyContact
}

// AdminOperationView is a read-only view of a financial operation for admin operations.
type AdminOperationView struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           *uuid.UUID
	PropertyName         *string
	LeaseID              *uuid.UUID
	RecurringOperationID *uuid.UUID
	Type                 leasesdomain.OperationType
	CategoryID           uuid.UUID
	CategoryName         string
	Name                 string
	AmountKopecks        int64
	OperationDate        time.Time
	Comment              *string
	IsException          bool
	Status               leasesdomain.OperationStatus
	ReminderOffsetDays   *int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AdminUserFilters carries optional filters for the admin users list.
type AdminUserFilters struct {
	Phone              string
	Email              string
	Role               string
	SubscriptionStatus string
	Limit              int
	Offset             int
	Sort               string
	Order              string
}

// AdminPropertyFilters carries optional filters for the admin properties list.
// OwnerID is uuid.Nil for the flat cross-user list.
type AdminPropertyFilters struct {
	OwnerID uuid.UUID
	Status  string
	Q       string
	Limit   int
	Offset  int
	Sort    string
	Order   string
}

// AdminLeaseFilters carries optional filters for the admin leases list.
// OwnerID is uuid.Nil for the flat cross-user list.
type AdminLeaseFilters struct {
	OwnerID    uuid.UUID
	PropertyID uuid.UUID
	Status     string
	Limit      int
	Offset     int
	Sort       string
	Order      string
}

// AdminTenantContactFilters carries optional filters for the admin tenant contacts list.
// OwnerID is uuid.Nil for the flat cross-user list.
type AdminTenantContactFilters struct {
	OwnerID uuid.UUID
	Q       string
	Limit   int
	Offset  int
	Sort    string
	Order   string
}

// AdminPropertyContactFilters carries filters for the admin property contacts list.
// Sort is fixed (created_at ASC) and not configurable.
type AdminPropertyContactFilters struct {
	PropertyID uuid.UUID
	Limit      int
	Offset     int
}

// AdminOperationFilters carries optional filters for the admin operations list.
// OwnerID is uuid.Nil for the flat cross-user list.
type AdminOperationFilters struct {
	OwnerID    uuid.UUID
	Status     string
	Type       string
	PropertyID uuid.UUID
	LeaseID    uuid.UUID
	Q          string
	Limit      int
	Offset     int
	Sort       string
	Order      string
}

// AdminAuditLogView is the admin read model of one audit_log row.
type AdminAuditLogView struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	ActorID    *uuid.UUID
	ActorRole  string
	Action     string
	EntityType *string
	EntityID   *uuid.UUID
	Context    map[string]any
	RequestID  *string
	IP         *string
}

// AdminAuditLogFilters carries optional filters for the admin audit log list.
// Zero values disable the filter.
type AdminAuditLogFilters struct {
	ActorID    uuid.UUID // Off when uuid.Nil.
	Action     string
	EntityType string
	DateFrom   time.Time // Off when zero.
	DateTo     time.Time // Off when zero; exclusive upper bound.
	Limit      int
	Offset     int
	Sort       string
	Order      string
}

// AdminRecentUserView is a read-only view of a recently registered user for
// the admin dashboard. Phone is stored decrypted.
type AdminRecentUserView struct {
	ID        uuid.UUID
	Phone     string
	Name      *string
	Surname   *string
	CreatedAt time.Time
}

// AdminRecentPaymentView is a read-only view of a recent subscription payment
// for the admin dashboard. UserPhone is stored decrypted.
type AdminRecentPaymentView struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	UserPhone     string
	AmountKopecks int64
	Status        string
	CreatedAt     time.Time
}

// AdminStatsView aggregates platform-wide counters and recent activity for the
// admin dashboard. "Last30d" metrics cover rows created in the last 30 days.
type AdminStatsView struct {
	UsersTotal                           int64
	UsersNewLast30d                      int64
	SubscriptionsActive                  int64
	PropertiesActive                     int64
	PropertiesArchived                   int64
	LeasesTotal                          int64
	OperationsTotal                      int64
	PaymentsSucceededTotalKopecksLast30d int64
	PaymentsFailedCountLast30d           int64
	PaymentsRefundedCountLast30d         int64
	RecentUsers                          []AdminRecentUserView
	RecentPayments                       []AdminRecentPaymentView
}
