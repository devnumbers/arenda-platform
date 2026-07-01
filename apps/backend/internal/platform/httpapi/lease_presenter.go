package httpapi

import (
	"context"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type leasePresenter struct {
	tenantContactSvc *leasesapp.TenantContactService
}

func newLeasePresenter(tenantContactSvc *leasesapp.TenantContactService) *leasePresenter {
	return &leasePresenter{tenantContactSvc: tenantContactSvc}
}

func (p *leasePresenter) tenantContactIDs(ctx context.Context, ownerID uuid.UUID, leases []leasesdomain.Lease) (map[uuid.UUID]leasesdomain.TenantContact, error) {
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

func (p *leasePresenter) leaseResponse(ctx context.Context, ownerID uuid.UUID, lease leasesdomain.Lease, contacts map[uuid.UUID]leasesdomain.TenantContact) (openapi.LeaseResponse, error) {
	resp := openapi.LeaseResponse{
		Id:                   lease.ID,
		OwnerId:              lease.OwnerID,
		PropertyId:           lease.PropertyID,
		Status:               openapi.LeaseStatus(lease.Status),
		StartDate:            openapi_types.Date{Time: lease.StartDate},
		EndDate:              datePtrToOpenAPI(lease.EndDate),
		RentAmountKopecks:    int(lease.RentAmountKopecks),
		DepositAmountKopecks: int(lease.DepositAmountKopecks),
		PaymentDay:           lease.PaymentDay,
		CreatedAt:            lease.CreatedAt,
		UpdatedAt:            lease.UpdatedAt,
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
