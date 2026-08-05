package http

import (
	"context"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// LeasePresenter renders leases-domain objects into the generated openapi lease
// response DTOs, resolving tenant contacts through the TenantContactService. It
// is also used by the properties HTTP adapter to embed lease summaries in
// property responses, so its constructor and the methods the properties adapter
// calls are exported.
type LeasePresenter struct {
	tenantContactSvc *leasesapp.TenantContactService
}

// NewLeasePresenter creates a LeasePresenter backed by the given tenant contact service.
func NewLeasePresenter(tenantContactSvc *leasesapp.TenantContactService) *LeasePresenter {
	return &LeasePresenter{tenantContactSvc: tenantContactSvc}
}

func (p *LeasePresenter) tenantContactIDs(ctx context.Context, ownerID uuid.UUID, leases []leasesdomain.Lease) (map[uuid.UUID]leasesdomain.TenantContact, error) {
	ids := make([]uuid.UUID, 0, len(leases))
	for _, lease := range leases {
		if lease.TenantContactID != nil && *lease.TenantContactID != uuid.Nil {
			ids = append(ids, *lease.TenantContactID)
		}
	}
	if len(ids) == 0 {
		return map[uuid.UUID]leasesdomain.TenantContact{}, nil
	}
	return p.tenantContactSvc.ListTenantContactsByIDs(ctx, ownerID, ids)
}

// TenantContactIDs resolves the distinct tenant contacts referenced by the
// given leases into a map keyed by contact ID. It is the batched lookup used by
// list endpoints so each response avoids a per-lease round trip.
func (p *LeasePresenter) TenantContactIDs(ctx context.Context, ownerID uuid.UUID, leases []leasesdomain.Lease) (map[uuid.UUID]leasesdomain.TenantContact, error) {
	return p.tenantContactIDs(ctx, ownerID, leases)
}

func (p *LeasePresenter) leaseResponse(ctx context.Context, ownerID uuid.UUID, lease leasesdomain.Lease, contacts map[uuid.UUID]leasesdomain.TenantContact, overdueSince, nextPaymentDate *time.Time, hasOverdue bool) (openapi.LeaseResponse, error) {
	resp := openapi.LeaseResponse{
		Id:                   lease.ID,
		OwnerId:              lease.OwnerID,
		PropertyId:           leasesdomain.PropertyIDPtr(lease.PropertyID),
		Status:               openapi.LeaseStatus(lease.Status),
		StartDate:            openapi_types.Date{Time: lease.StartDate},
		EndDate:              httpsupport.DatePtrToOpenAPI(lease.EndDate),
		RentAmountKopecks:    int(lease.RentAmountKopecks),
		DepositAmountKopecks: int(lease.DepositAmountKopecks),
		PaymentDay:           lease.PaymentDay,
		HasOverdue:           hasOverdue,
		CreatedAt:            lease.CreatedAt,
		UpdatedAt:            lease.UpdatedAt,
	}
	if overdueSince != nil {
		resp.CurrentPeriodOverdue = true
		resp.OverdueSince = httpsupport.DatePtrToOpenAPI(overdueSince)
	}
	if nextPaymentDate != nil {
		resp.NextPaymentDate = httpsupport.DatePtrToOpenAPI(nextPaymentDate)
	}
	if lease.Comment != "" {
		resp.Comment = &lease.Comment
	}
	if lease.TenantContactID != nil && *lease.TenantContactID != uuid.Nil {
		if contacts != nil {
			contact, ok := contacts[*lease.TenantContactID]
			if !ok {
				return openapi.LeaseResponse{}, leasesapp.ErrTenantContactNotFound
			}
			resp.TenantContact = new(tenantContactResponse(contact))
		} else {
			contact, err := p.tenantContactSvc.GetTenantContact(ctx, ownerID, *lease.TenantContactID)
			if err != nil {
				return openapi.LeaseResponse{}, err
			}
			resp.TenantContact = new(tenantContactResponse(contact))
		}
	}
	return resp, nil
}

// LeaseResponse maps a single lease (plus optional pre-resolved contacts and
// payment-schedule fields) into the openapi LeaseResponse DTO. The properties
// HTTP adapter calls this to embed the active lease summary in property
// responses and to render a property's lease list.
func (p *LeasePresenter) LeaseResponse(ctx context.Context, ownerID uuid.UUID, lease leasesdomain.Lease, contacts map[uuid.UUID]leasesdomain.TenantContact, overdueSince, nextPaymentDate *time.Time, hasOverdue bool) (openapi.LeaseResponse, error) {
	return p.leaseResponse(ctx, ownerID, lease, contacts, overdueSince, nextPaymentDate, hasOverdue)
}
