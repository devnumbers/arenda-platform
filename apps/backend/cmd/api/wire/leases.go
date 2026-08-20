package wire

import (
	leasespg "github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters/postgres"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// LeasesRepos holds the leases module's repositories, category service and
// property billing lifecycle. These are constructed before the properties
// services because PropertyService depends on PropertyBillingLifecycle and the
// shared lease/category repositories.
type LeasesRepos struct {
	OperationRepo            *leasespg.OperationRepository
	RecurringOpRepo          *leasespg.RecurringOperationRepository
	LeaseRepo                *leasespg.LeaseRepository
	CategoryRepo             *leasespg.OperationCategoryRepository
	CategoryService          *leasesapp.CategoryService
	PropertyBillingLifecycle *leasespg.PropertyBillingLifecycle
}

// WireLeasesRepos constructs the operation, recurring-operation, lease and
// category repositories, the category service and the property billing
// lifecycle. It must run before the properties services are built.
func WireLeasesRepos(p platformDeps, reminderScheduler notificationsapp.ReminderScheduler) *LeasesRepos {
	operationRepo := leasespg.NewOperationRepository(p.DB)
	recurringOpRepo := leasespg.NewRecurringOperationRepository(p.DB)
	leaseRepo := leasespg.NewLeaseRepository(p.DB)
	categoryRepo := leasespg.NewOperationCategoryRepository(p.DB)
	categoryService := leasesapp.NewCategoryService(categoryRepo, p.AuditRecorder)
	propertyBillingLifecycle := leasespg.NewPropertyBillingLifecycle(
		operationRepo, recurringOpRepo, leaseRepo, categoryRepo,
		reminderScheduler, p.AuditRecorder, p.Clock,
	)

	return &LeasesRepos{
		OperationRepo:            operationRepo,
		RecurringOpRepo:          recurringOpRepo,
		LeaseRepo:                leaseRepo,
		CategoryRepo:             categoryRepo,
		CategoryService:          categoryService,
		PropertyBillingLifecycle: propertyBillingLifecycle,
	}
}

// Leases holds the leases module's services wired by WireLeasesServices. The
// export service additionally depends on the leases property and
// property-contact repositories which mirror the properties ones but live in
// the leases package.
type Leases struct {
	LeaseService              *leasesapp.LeaseService
	TenantContactService      *leasesapp.TenantContactService
	OperationService          *leasesapp.OperationService
	ExportService             *leasesapp.ExportService
	RecurringOperationService *leasesapp.RecurringOperationService
}

// WireLeasesServices constructs the lease, tenant-contact, operation, export and
// recurring-operation services. It takes the leases repos (built earlier), the
// reminder scheduler and the reminder service (notifications module) that some
// services depend on.
func WireLeasesServices(
	p platformDeps,
	repos *LeasesRepos,
	reminderScheduler notificationsapp.ReminderScheduler,
	reminderService *notificationsapp.ReminderService,
) *Leases {
	leasePropertyRepo := leasespg.NewPropertyRepository(p.DB)
	leasePropertyContactRepo := leasespg.NewPropertyContactRepository(p.DB)
	tenantContactRepo := leasespg.NewTenantContactRepository(p.DB)

	// The single canonical txStoreFactory bundles the leases
	// repositories, the cross-context reminder scheduler, the audit recorder,
	// and the UoW (ADR 0033 γ-factory). It is passed to the three leases
	// services that open their own transactions so adding an Nth repository is
	// a change here, not in several constructors.
	factory := leasesapp.NewTxStoreFactory(
		repos.LeaseRepo,
		leasePropertyRepo,
		tenantContactRepo,
		repos.RecurringOpRepo,
		repos.OperationRepo,
		repos.CategoryRepo,
		reminderScheduler,
		p.AuditRecorder,
		p.UoW,
	)

	leaseService := leasesapp.NewLeaseService(
		repos.LeaseRepo,
		leasePropertyRepo,
		tenantContactRepo,
		repos.CategoryRepo,
		factory,
		p.Clock,
		p.TZResolver,
		p.Policy,
		p.Logger,
	)
	tenantContactService := leasesapp.NewTenantContactService(tenantContactRepo, p.AuditRecorder, p.Logger)
	// Both owner-wide services resolve the data owner of a property context
	// through the leases property repository (Property Sharing follow-up); the
	// policy is injected later via SetPolicy (see cmd/api/main.go).
	repos.CategoryService.SetProperties(leasePropertyRepo)
	tenantContactService.SetProperties(leasePropertyRepo)
	operationService := leasesapp.NewOperationService(
		repos.OperationRepo,
		leasePropertyRepo,
		repos.LeaseRepo,
		repos.CategoryRepo,
		factory,
		p.Clock,
		p.TZResolver,
		p.Policy,
		p.Logger,
	)
	exportService := leasesapp.NewExportService(
		repos.OperationRepo, repos.LeaseRepo, leasePropertyRepo, leasePropertyContactRepo,
		p.Policy, p.Clock, p.Logger,
	)
	recurringOperationService := leasesapp.NewRecurringOperationService(
		repos.RecurringOpRepo,
		repos.OperationRepo,
		leasePropertyRepo,
		repos.CategoryRepo,
		reminderService,
		factory,
		p.Clock,
		p.TZResolver,
		p.Policy,
		p.Logger,
	)

	return &Leases{
		LeaseService:              leaseService,
		TenantContactService:      tenantContactService,
		OperationService:          operationService,
		ExportService:             exportService,
		RecurringOperationService: recurringOperationService,
	}
}
