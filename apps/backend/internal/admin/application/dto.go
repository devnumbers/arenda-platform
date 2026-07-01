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
	Name        string
	Type        propertiesdomain.PropertyType
	Address     string
	Description *string
	Status      propertiesdomain.PropertyStatus
	Occupancy   propertiesdomain.PropertyOccupancy
	Photos      []propertiesdomain.Photo
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AdminLeaseView is a read-only view of a lease for admin operations.
type AdminLeaseView struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           uuid.UUID
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
type AdminTenantContactView leasesdomain.TenantContact

// AdminOperationView is a read-only view of a financial operation for admin operations.
type AdminOperationView struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           uuid.UUID
	LeaseID              *uuid.UUID
	RecurringOperationID *uuid.UUID
	Type                 leasesdomain.OperationType
	Category             leasesdomain.OperationCategory
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
}

// AdminPropertyFilters carries optional filters for the admin properties list.
type AdminPropertyFilters struct {
	Status string
	Limit  int
	Offset int
}

// AdminListFilters carries pagination for simple admin list endpoints.
type AdminListFilters struct {
	Limit  int
	Offset int
}

// AdminOperationFilters carries optional filters for the admin operations list.
type AdminOperationFilters struct {
	Status     string
	Type       string
	PropertyID uuid.UUID
	LeaseID    uuid.UUID
	Limit      int
	Offset     int
}
