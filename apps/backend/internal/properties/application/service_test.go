package application

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakePropertyClock struct {
	now time.Time
}

func (c fakePropertyClock) Now() time.Time { return c.now }

type fakePropertyTx struct {
	unlock func()
}

func (tx *fakePropertyTx) Commit(_ context.Context) error {
	if tx.unlock != nil {
		tx.unlock()
		tx.unlock = nil
	}
	return nil
}

func (tx *fakePropertyTx) Rollback(_ context.Context) error {
	if tx.unlock != nil {
		tx.unlock()
		tx.unlock = nil
	}
	return nil
}

type fakePropertyTxBeginner struct{}

func (fakePropertyTxBeginner) Begin(_ context.Context) (transaction.Tx, error) {
	return &fakePropertyTx{}, nil
}

type fakeOccupancyProvider struct{}

func (fakeOccupancyProvider) IsOccupied(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (fakeOccupancyProvider) OccupiedPropertyIDs(_ context.Context, _ uuid.UUID) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

func (p fakeOccupancyProvider) WithTx(_ transaction.Tx) OccupancyProvider {
	return p
}

type lockingFakePropertyRepo struct {
	lock *sync.Mutex
	data map[uuid.UUID]domain.Property
	tx   *fakePropertyTx
}

func newLockingFakePropertyRepo(properties ...domain.Property) *lockingFakePropertyRepo {
	data := make(map[uuid.UUID]domain.Property, len(properties))
	for _, p := range properties {
		data[p.ID] = p
	}
	return &lockingFakePropertyRepo{
		lock: &sync.Mutex{},
		data: data,
	}
}

func (r *lockingFakePropertyRepo) Create(_ context.Context, _ uuid.UUID, property domain.Property) (domain.Property, error) {
	r.data[property.ID] = property
	return property, nil
}

func (r *lockingFakePropertyRepo) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (domain.Property, error) {
	p, ok := r.data[id]
	if !ok {
		return domain.Property{}, ErrNotFound
	}
	return p, nil
}

func (r *lockingFakePropertyRepo) GetByIDAndOwnerForUpdate(_ context.Context, id, _ uuid.UUID) (domain.Property, error) {
	r.lock.Lock()
	if r.tx != nil {
		r.tx.unlock = r.lock.Unlock
	} else {
		defer r.lock.Unlock()
	}
	p, ok := r.data[id]
	if !ok {
		return domain.Property{}, ErrNotFound
	}
	return p, nil
}

func (r *lockingFakePropertyRepo) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	return nil, nil
}

func (r *lockingFakePropertyRepo) ListArchivedByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	archived := make([]domain.Property, 0)
	for _, p := range r.data {
		if p.Status == domain.PropertyStatusArchived {
			archived = append(archived, p)
		}
	}
	return archived, nil
}

func (r *lockingFakePropertyRepo) Update(_ context.Context, _ uuid.UUID, property domain.Property) (domain.Property, error) {
	// The caller already holds the lock via GetByIDAndOwnerForUpdate.
	if _, ok := r.data[property.ID]; !ok {
		return domain.Property{}, ErrNotFound
	}
	r.data[property.ID] = property
	return property, nil
}

func (r *lockingFakePropertyRepo) Archive(_ context.Context, id, _ uuid.UUID) error {
	// The caller already holds the lock via GetByIDAndOwnerForUpdate.
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	p.Status = domain.PropertyStatusArchived
	r.data[id] = p
	return nil
}

func (r *lockingFakePropertyRepo) Unarchive(_ context.Context, id, _ uuid.UUID) error {
	// The caller already holds the lock via GetByIDAndOwnerForUpdate.
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	p.Status = domain.PropertyStatusActive
	r.data[id] = p
	return nil
}

func (r *lockingFakePropertyRepo) CountActiveByOwner(_ context.Context, _ uuid.UUID) (int, error) {
	count := 0
	for _, p := range r.data {
		if p.Status != domain.PropertyStatusArchived {
			count++
		}
	}
	return count, nil
}

func (r *lockingFakePropertyRepo) WithTx(tx transaction.Tx) PropertyRepository {
	return &lockingFakePropertyRepo{
		lock: r.lock,
		data: r.data,
		tx:   tx.(*fakePropertyTx),
	}
}

func TestUpdateProperty_ConcurrentUpdatesDoNotOverwrite(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	property := domain.Property{
		ID:      propertyID,
		OwnerID: ownerID,
		Name:    "Old Name",
		Address: "Old Address",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}

	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProvider{},
		nil,
		nil,
		stubLeaseRepo{},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: time.Now()},
		nil,
	)

	var wg sync.WaitGroup
	var firstDone atomic.Bool
	wg.Add(2)

	go func() {
		defer wg.Done()
		name := "New Name"
		_, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
			Name: &name,
		})
		if err != nil {
			t.Errorf("name update failed: %v", err)
		}
		firstDone.Store(true)
	}()

	go func() {
		defer wg.Done()
		for !firstDone.Load() {
			time.Sleep(time.Millisecond)
		}
		address := "New Address"
		_, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
			Address: &address,
		})
		if err != nil {
			t.Errorf("address update failed: %v", err)
		}
	}()

	wg.Wait()

	final, err := repo.GetByIDAndOwner(ctx, propertyID, ownerID)
	if err != nil {
		t.Fatalf("get final property: %v", err)
	}
	if final.Name != "New Name" {
		t.Errorf("name update lost: got %q, want New Name", final.Name)
	}
	if final.Address != "New Address" {
		t.Errorf("address update lost: got %q, want New Address", final.Address)
	}
}

var _ PropertyRepository = (*lockingFakePropertyRepo)(nil)
var _ OccupancyProvider = fakeOccupancyProvider{}

type fakeSubscriptionLimiter struct {
	limit int
}

func (l fakeSubscriptionLimiter) ActivePropertyLimit(_ context.Context, _ uuid.UUID) (int, error) {
	return l.limit, nil
}

func (l fakeSubscriptionLimiter) WithTx(_ transaction.Tx) (SubscriptionLimiter, error) {
	return l, nil
}

type fakePropertyBillingLifecycle struct{}

func (fakePropertyBillingLifecycle) Suspend(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (fakePropertyBillingLifecycle) Resume(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (fakePropertyBillingLifecycle) WithTx(_ transaction.Tx) PropertyBillingLifecycle {
	return fakePropertyBillingLifecycle{}
}

func TestArchiveProperty_ConcurrentArchivesDoNotDoubleArchive(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	property := domain.Property{
		ID:      propertyID,
		OwnerID: ownerID,
		Name:    "Active Property",
		Address: "Address",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}

	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProvider{},
		fakeSubscriptionLimiter{limit: 10},
		fakePropertyBillingLifecycle{},
		stubLeaseRepo{},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: time.Now()},
		nil,
	)

	var wg sync.WaitGroup
	var firstDone atomic.Bool
	var firstErr, secondErr error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, firstErr = svc.ArchiveProperty(ctx, ownerID, propertyID)
		firstDone.Store(true)
	}()

	go func() {
		defer wg.Done()
		for !firstDone.Load() {
			time.Sleep(time.Millisecond)
		}
		_, secondErr = svc.ArchiveProperty(ctx, ownerID, propertyID)
	}()

	wg.Wait()

	if firstErr != nil && secondErr != nil {
		t.Fatalf("both archive attempts failed: %v / %v", firstErr, secondErr)
	}
	if firstErr != nil && !errors.Is(firstErr, ErrAlreadyArchived) {
		t.Fatalf("first archive failed unexpectedly: %v", firstErr)
	}
	if secondErr != nil && !errors.Is(secondErr, ErrAlreadyArchived) {
		t.Fatalf("second archive failed unexpectedly: %v", secondErr)
	}

	final, err := repo.GetByIDAndOwner(ctx, propertyID, ownerID)
	if err != nil {
		t.Fatalf("get final property: %v", err)
	}
	if final.Status != domain.PropertyStatusArchived {
		t.Errorf("expected archived status, got %q", final.Status)
	}
}

func TestUnarchiveProperty_ConcurrentUnarchivesRespectLimit(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	propertyB := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	repo := newLockingFakePropertyRepo()
	repo.data[propertyA] = domain.Property{
		ID:      propertyA,
		OwnerID: ownerID,
		Name:    "Archived A",
		Address: "Address A",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusArchived,
	}
	repo.data[propertyB] = domain.Property{
		ID:      propertyB,
		OwnerID: ownerID,
		Name:    "Archived B",
		Address: "Address B",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusArchived,
	}

	// Limit of one active property means only one unarchive can succeed.
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProvider{},
		fakeSubscriptionLimiter{limit: 1},
		fakePropertyBillingLifecycle{},
		stubLeaseRepo{},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: time.Now()},
		nil,
	)

	var wg sync.WaitGroup
	var firstDone atomic.Bool
	var firstErr, secondErr error
	var firstResult, secondResult domain.Property
	wg.Add(2)

	go func() {
		defer wg.Done()
		firstResult, firstErr = svc.UnarchiveProperty(ctx, ownerID, propertyA)
		firstDone.Store(true)
	}()

	go func() {
		defer wg.Done()
		for !firstDone.Load() {
			time.Sleep(time.Millisecond)
		}
		secondResult, secondErr = svc.UnarchiveProperty(ctx, ownerID, propertyB)
	}()

	wg.Wait()

	successCount := 0
	if firstErr == nil {
		successCount++
	} else if !errors.Is(firstErr, ErrLimitExceeded) {
		t.Fatalf("first unarchive failed unexpectedly: %v", firstErr)
	}
	if secondErr == nil {
		successCount++
	} else if !errors.Is(secondErr, ErrLimitExceeded) {
		t.Fatalf("second unarchive failed unexpectedly: %v", secondErr)
	}

	if successCount != 1 {
		t.Fatalf("expected exactly one successful unarchive, got %d (first=%v, second=%v)", successCount, firstResult.Status, secondResult.Status)
	}

	activeCount := 0
	for _, p := range repo.data {
		if p.Status == domain.PropertyStatusActive {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("expected 1 active property, got %d", activeCount)
	}
}

var _ SubscriptionLimiter = fakeSubscriptionLimiter{}
var _ PropertyBillingLifecycle = fakePropertyBillingLifecycle{}

type stubLeaseRepo struct{}

func (r stubLeaseRepo) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]leasesdomain.Lease, error) {
	return nil, nil
}

func (r stubLeaseRepo) GetOpenLeaseByProperty(_ context.Context, _, _ uuid.UUID) (leasesdomain.Lease, error) {
	return leasesdomain.Lease{}, ErrNotFound
}

var _ LeaseRepository = stubLeaseRepo{}

type fakePropertyPhotoRepo struct{}

func (fakePropertyPhotoRepo) Create(_ context.Context, _, _ uuid.UUID, _ string) (domain.Photo, error) {
	return domain.Photo{}, nil
}

func (fakePropertyPhotoRepo) GetByPropertyID(_ context.Context, _ uuid.UUID) ([]domain.Photo, error) {
	return nil, nil
}

func (fakePropertyPhotoRepo) GetByPropertyIDs(_ context.Context, _ []uuid.UUID) (map[uuid.UUID][]domain.Photo, error) {
	return map[uuid.UUID][]domain.Photo{}, nil
}

func (fakePropertyPhotoRepo) CountByPropertyID(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (fakePropertyPhotoRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Photo, error) {
	return domain.Photo{}, nil
}

func (fakePropertyPhotoRepo) GetByIDAndPropertyID(_ context.Context, _, _ uuid.UUID) (domain.Photo, error) {
	return domain.Photo{}, nil
}

func (fakePropertyPhotoRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (fakePropertyPhotoRepo) WithTx(_ transaction.Tx) PropertyPhotoRepository {
	return fakePropertyPhotoRepo{}
}

type fakePropertyPhotoStorage struct{}

func (fakePropertyPhotoStorage) Upload(_ context.Context, _, _ string, _ int64, _ io.Reader) (string, error) {
	return "", nil
}

func (fakePropertyPhotoStorage) Delete(_ context.Context, _ string) error {
	return nil
}

func (fakePropertyPhotoStorage) HeadBucket(_ context.Context) error {
	return nil
}

var _ PropertyPhotoRepository = fakePropertyPhotoRepo{}
var _ PhotoStorage = fakePropertyPhotoStorage{}

type fakeLeaseRepoForProperties struct {
	leases []leasesdomain.Lease
}

func (r fakeLeaseRepoForProperties) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]leasesdomain.Lease, error) {
	return r.leases, nil
}

func (r fakeLeaseRepoForProperties) GetOpenLeaseByProperty(_ context.Context, _, propertyID uuid.UUID) (leasesdomain.Lease, error) {
	for _, lease := range r.leases {
		if lease.PropertyID == propertyID && lease.Status.IsOpen() {
			return lease, nil
		}
	}
	return leasesdomain.Lease{}, ErrNotFound
}

var _ LeaseRepository = fakeLeaseRepoForProperties{}

func TestPropertyService_ListPropertyLeases(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	property := domain.Property{
		ID:      propertyID,
		OwnerID: ownerID,
		Name:    "Test Property",
		Address: "Address",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}

	leases := []leasesdomain.Lease{
		{
			ID:                leaseID,
			OwnerID:           ownerID,
			PropertyID:        propertyID,
			Status:            leasesdomain.LeaseStatusActive,
			StartDate:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			RentAmountKopecks: 10000,
			PaymentDay:        1,
		},
	}

	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProvider{},
		nil,
		nil,
		fakeLeaseRepoForProperties{leases: leases},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: now},
		nil,
	)

	result, err := svc.ListPropertyLeases(ctx, ownerID, propertyID)
	if err != nil {
		t.Fatalf("ListPropertyLeases failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 lease, got %d", len(result))
	}
	if result[0].ID != leaseID {
		t.Errorf("expected lease id %s, got %s", leaseID, result[0].ID)
	}
	if result[0].Status != leasesdomain.LeaseStatusActive {
		t.Errorf("expected effective status active, got %q", result[0].Status)
	}
}

func TestPropertyService_ListPropertyLeases_PropertyNotFound(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	missingPropertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Now()

	repo := newLockingFakePropertyRepo()
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProvider{},
		nil,
		nil,
		fakeLeaseRepoForProperties{},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: now},
		nil,
	)

	_, err := svc.ListPropertyLeases(ctx, ownerID, missingPropertyID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPropertyService_ListArchivedProperties(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	archivedID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newLockingFakePropertyRepo()
	repo.data[archivedID] = domain.Property{
		ID:      archivedID,
		OwnerID: ownerID,
		Name:    "Archived",
		Address: "Archive St",
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusArchived,
	}

	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProvider{},
		nil,
		nil,
		stubLeaseRepo{},
		fakePropertyTxBeginner{},
		nil,
		fakePropertyClock{now: time.Now()},
		nil,
	)

	result, err := svc.ListArchivedProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("list archived properties: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 archived property, got %d", len(result))
	}
	if result[0].ID != archivedID {
		t.Errorf("expected property %s, got %s", archivedID, result[0].ID)
	}
	if result[0].Status != domain.PropertyStatusArchived {
		t.Errorf("expected archived status, got %q", result[0].Status)
	}
}
