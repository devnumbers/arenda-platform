package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeLeaseRepo struct {
	leases []domain.Lease
}

var _ leasesapp.LeaseRepository = (*fakeLeaseRepo)(nil)

type fakeLeaseClock struct{}

func (fakeLeaseClock) Now() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

type fakeLeaseBeginner struct{}

func (fakeLeaseBeginner) Begin(context.Context) (transaction.Tx, error) { return testTx{}, nil }

func (r *fakeLeaseRepo) Create(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	return lease, nil
}

func (r *fakeLeaseRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, nil
}

func (r *fakeLeaseRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, nil
}

func (r *fakeLeaseRepo) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	for _, lease := range r.leases {
		if lease.ID == id && lease.OwnerID == ownerID {
			return lease, nil
		}
	}
	return domain.Lease{}, leasesapp.ErrNotFound
}

func (r *fakeLeaseRepo) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakeLeaseRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Lease, error) {
	return r.leases, nil
}

func (r *fakeLeaseRepo) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Lease, error) {
	var out []domain.Lease
	for _, lease := range r.leases {
		if lease.OwnerID == ownerID && lease.PropertyID == propertyID {
			out = append(out, lease)
		}
	}
	return out, nil
}

func (r *fakeLeaseRepo) Update(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	return lease, nil
}

func (r *fakeLeaseRepo) Complete(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, nil
}

func (r *fakeLeaseRepo) CountOpenLeasesByProperty(ctx context.Context, propertyID uuid.UUID) (int, error) {
	return 0, nil
}

func (r *fakeLeaseRepo) GetOpenLeaseByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, nil
}

func (r *fakeLeaseRepo) ListOpenLeasesWithPastEndDate(ctx context.Context, asOf time.Time, limit int) ([]domain.Lease, error) {
	return nil, nil
}

func (r *fakeLeaseRepo) WithTx(tx transaction.Tx) leasesapp.LeaseRepository {
	return r
}

type fakeTenantContactRepo struct {
	contacts        []domain.TenantContact
	listByIDsCalls  int
	getByIDCalls    int
	listByIDsInputs [][]uuid.UUID
}

func (r *fakeTenantContactRepo) Create(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error) {
	return contact, nil
}

func (r *fakeTenantContactRepo) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error) {
	r.getByIDCalls++
	for _, c := range r.contacts {
		if c.ID == id && c.OwnerID == ownerID {
			return c, nil
		}
	}
	return domain.TenantContact{}, leasesapp.ErrNotFound
}

func (r *fakeTenantContactRepo) ListByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]domain.TenantContact, error) {
	r.listByIDsCalls++
	r.listByIDsInputs = append(r.listByIDsInputs, ids)
	result := make([]domain.TenantContact, 0, len(ids))
	for _, c := range r.contacts {
		if c.OwnerID != ownerID {
			continue
		}
		for _, id := range ids {
			if c.ID == id {
				result = append(result, c)
			}
		}
	}
	return result, nil
}

func (r *fakeTenantContactRepo) Update(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error) {
	return contact, nil
}

func (r *fakeTenantContactRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContact, error) {
	return r.contacts, nil
}

func (r *fakeTenantContactRepo) ListWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error) {
	return nil, nil
}

func (r *fakeTenantContactRepo) WithTx(tx transaction.Tx) leasesapp.TenantContactRepository {
	return r
}

func TestLeaseHandlers_ListLeases_FetchesContactsInBatch(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	contactID1 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
	contactID2 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a04")

	leases := []domain.Lease{
		{ID: uuid.New(), OwnerID: ownerID, PropertyID: propertyID, Status: domain.LeaseStatusActive, TenantContactID: &contactID1, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PaymentDay: 1},
		{ID: uuid.New(), OwnerID: ownerID, PropertyID: propertyID, Status: domain.LeaseStatusActive, TenantContactID: &contactID2, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PaymentDay: 1},
	}

	contactRepo := &fakeTenantContactRepo{
		contacts: []domain.TenantContact{
			{ID: contactID1, OwnerID: ownerID, Name: "Alice"},
			{ID: contactID2, OwnerID: ownerID, Name: "Bob"},
		},
	}
	leaseRepo := &fakeLeaseRepo{leases: leases}

	leaseSvc := leasesapp.NewLeaseService(leaseRepo, nil, contactRepo, nil, nil, nil, fakeLeaseBeginner{}, fakeLeaseClock{}, slog.Default())
	tenantSvc := leasesapp.NewTenantContactService(contactRepo, slog.Default())
	handlers := NewLeaseHandlers(leaseSvc, tenantSvc, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/leases", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.ListLeases(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if contactRepo.listByIDsCalls != 1 {
		t.Fatalf("expected 1 ListByIDs call, got %d", contactRepo.listByIDsCalls)
	}
	if contactRepo.getByIDCalls != 0 {
		t.Fatalf("expected 0 GetByIDAndOwner calls, got %d", contactRepo.getByIDCalls)
	}
	if len(contactRepo.listByIDsInputs) != 1 || len(contactRepo.listByIDsInputs[0]) != 2 {
		t.Fatalf("expected batch with 2 contact ids, got %v", contactRepo.listByIDsInputs)
	}

	var resp openapi.LeasesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 leases, got %d", len(resp.Items))
	}
	if resp.Items[0].TenantContact == nil || resp.Items[0].TenantContact.Name != "Alice" {
		t.Fatalf("expected first tenant contact Alice, got %v", resp.Items[0].TenantContact)
	}
	if resp.Items[1].TenantContact == nil || resp.Items[1].TenantContact.Name != "Bob" {
		t.Fatalf("expected second tenant contact Bob, got %v", resp.Items[1].TenantContact)
	}
}

func TestLeaseHandlers_ListLeases_NoContacts(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")

	leases := []domain.Lease{
		{ID: uuid.New(), OwnerID: ownerID, PropertyID: propertyID, Status: domain.LeaseStatusActive, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PaymentDay: 1},
	}

	contactRepo := &fakeTenantContactRepo{}
	leaseRepo := &fakeLeaseRepo{leases: leases}

	leaseSvc := leasesapp.NewLeaseService(leaseRepo, nil, contactRepo, nil, nil, nil, fakeLeaseBeginner{}, fakeLeaseClock{}, slog.Default())
	tenantSvc := leasesapp.NewTenantContactService(contactRepo, slog.Default())
	handlers := NewLeaseHandlers(leaseSvc, tenantSvc, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/leases", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.ListLeases(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if contactRepo.listByIDsCalls != 0 {
		t.Fatalf("expected 0 ListByIDs calls when no contacts, got %d", contactRepo.listByIDsCalls)
	}

	var resp openapi.LeasesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 lease, got %d", len(resp.Items))
	}
	if resp.Items[0].TenantContact != nil {
		t.Fatalf("expected nil tenant contact, got %v", resp.Items[0].TenantContact)
	}
}

func TestLeaseHandlers_ReturnLeaseDeposit(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	leaseID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
	endDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	leaseRepo := &fakeLeaseRepo{
		leases: []domain.Lease{
			{
				ID:                   leaseID,
				OwnerID:              ownerID,
				PropertyID:           propertyID,
				Status:               domain.LeaseStatusCompleted,
				StartDate:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:              &endDate,
				RentAmountKopecks:    10000,
				DepositAmountKopecks: 50000,
				PaymentDay:           1,
			},
		},
	}
	opsRepo := &fakeOperationRepoForHandlers{}
	contactRepo := &fakeTenantContactRepo{}
	propertyRepo := newFakePropertyRepoForHandlers()

	leaseSvc := leasesapp.NewLeaseService(
		leaseRepo,
		&fakeLeasesAppPropertyRepo{wrapped: propertyRepo},
		contactRepo,
		fakeRecurringOperationRepoForHandlers{},
		opsRepo,
		fakeScheduler{},
		fakeLeaseBeginner{},
		fakeLeaseClock{},
		slog.Default(),
	)
	tenantSvc := leasesapp.NewTenantContactService(contactRepo, slog.Default())
	handlers := NewLeaseHandlers(leaseSvc, tenantSvc, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/leases/"+leaseID.String()+"/deposit-return", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.ReturnLeaseDeposit(rr, req, leaseID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp openapi.LeaseResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Id != leaseID {
		t.Fatalf("expected lease id %s, got %s", leaseID, resp.Id)
	}
	if resp.DepositAmountKopecks != 50000 {
		t.Fatalf("expected deposit 50000, got %d", resp.DepositAmountKopecks)
	}
	if resp.Status != openapi.LeaseStatusCompleted {
		t.Fatalf("expected completed status, got %s", resp.Status)
	}

	exists, err := opsRepo.HasDepositReturnForLease(context.Background(), leaseID)
	if err != nil {
		t.Fatalf("check deposit return: %v", err)
	}
	if !exists {
		t.Fatal("expected deposit return operation to be created")
	}
}

func TestLeaseHandlers_ReturnLeaseDeposit_NotFound(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	leaseID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")

	leaseRepo := &fakeLeaseRepo{leases: nil}
	opsRepo := &fakeOperationRepoForHandlers{}
	contactRepo := &fakeTenantContactRepo{}
	propertyRepo := newFakePropertyRepoForHandlers()

	leaseSvc := leasesapp.NewLeaseService(
		leaseRepo,
		&fakeLeasesAppPropertyRepo{wrapped: propertyRepo},
		contactRepo,
		fakeRecurringOperationRepoForHandlers{},
		opsRepo,
		fakeScheduler{},
		fakeLeaseBeginner{},
		fakeLeaseClock{},
		slog.Default(),
	)
	tenantSvc := leasesapp.NewTenantContactService(contactRepo, slog.Default())
	handlers := NewLeaseHandlers(leaseSvc, tenantSvc, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/leases/"+leaseID.String()+"/deposit-return", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.ReturnLeaseDeposit(rr, req, leaseID)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLeaseHandlers_ReturnLeaseDeposit_Unauthorized(t *testing.T) {
	leaseID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")

	handlers := NewLeaseHandlers(nil, nil, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/leases/"+leaseID.String()+"/deposit-return", nil)
	rr := httptest.NewRecorder()

	handlers.ReturnLeaseDeposit(rr, req, leaseID)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func withOwnerID(ctx context.Context, ownerID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, ownerID)
}
