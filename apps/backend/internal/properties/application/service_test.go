package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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

func (r *lockingFakePropertyRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Property, error) {
	p, ok := r.data[id]
	if !ok {
		return domain.Property{}, ErrNotFound
	}
	return p, nil
}

func (r *lockingFakePropertyRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	return r.GetByIDAndOwnerForUpdate(ctx, id, uuid.Nil)
}

func (r *lockingFakePropertyRepo) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	return nil, nil
}

func (r *lockingFakePropertyRepo) SearchVisible(_ context.Context, _ uuid.UUID, _ PropertySearchQuery) ([]domain.Property, error) {
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

func (r *lockingFakePropertyRepo) SetPin(_ context.Context, id, _ uuid.UUID, pinnedAt *time.Time) (domain.Property, error) {
	// The caller already holds the lock via GetByIDForUpdate.
	p, ok := r.data[id]
	if !ok {
		return domain.Property{}, ErrNotFound
	}
	p.PinnedAt = pinnedAt
	r.data[id] = p
	return p, nil
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

func (r *lockingFakePropertyRepo) CountByOwnerAndType(_ context.Context, scope uuid.UUID, propertyType domain.PropertyType) (int, error) {
	count := 0
	for _, p := range r.data {
		if p.OwnerID == scope && p.Type == propertyType {
			count++
		}
	}
	return count, nil
}

func (r *lockingFakePropertyRepo) Delete(_ context.Context, _, _ uuid.UUID) error {
	return errors.New("not implemented")
}

func (r *lockingFakePropertyRepo) WithTx(tx transaction.Tx) PropertyRepository {
	ftx, ok := tx.(*fakePropertyTx)
	if !ok {
		panic(fmt.Sprintf("lockingFakePropertyRepo.WithTx: unexpected tx type %T", tx))
	}
	return &lockingFakePropertyRepo{
		lock: r.lock,
		data: r.data,
		tx:   ftx,
	}
}

func TestUpdateProperty_ConcurrentUpdatesDoNotOverwrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	property := domain.Property{
		ID:      propertyID,
		OwnerID: ownerID,
		Name:    fixtureOldName,
		Address: fixtureOldAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}

	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
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

type fakeSubscriptionLimiter struct {
	limit int
}

func (l fakeSubscriptionLimiter) ActivePropertyLimit(_ context.Context, _ uuid.UUID) (int, error) {
	return l.limit, nil
}

func (l fakeSubscriptionLimiter) WithTx(_ transaction.Tx) (SubscriptionLimiter, error) {
	return l, nil
}

func TestArchiveProperty_ConcurrentArchivesDoNotDoubleArchive(t *testing.T) {
	t.Parallel()

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
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			fakeSubscriptionLimiter{limit: 10}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
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
	t.Parallel()

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
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			fakeSubscriptionLimiter{limit: 1}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
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

var (
	_ PropertyPhotoRepository = fakePropertyPhotoRepo{}
	_ PhotoStorage            = fakePropertyPhotoStorage{}
)

func TestPropertyService_ListArchivedProperties(t *testing.T) {
	t.Parallel()

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
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	result, err := svc.ListArchivedProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("list archived properties: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 archived property, got %d", len(result.Items))
	}
	if result.Items[0].ID != archivedID {
		t.Errorf("expected property %s, got %s", archivedID, result.Items[0].ID)
	}
	if result.Items[0].Status != domain.PropertyStatusArchived {
		t.Errorf("expected archived status, got %q", result.Items[0].Status)
	}
}

// fakePropertyRepo is a simple map-based PropertyRepository for behavior tests
// that do not exercise concurrency.
type fakePropertyRepo struct {
	data map[uuid.UUID]domain.Property
}

func newFakePropertyRepo(properties ...domain.Property) *fakePropertyRepo {
	data := make(map[uuid.UUID]domain.Property, len(properties))
	for _, p := range properties {
		data[p.ID] = p
	}
	return &fakePropertyRepo{data: data}
}

func (r *fakePropertyRepo) Create(_ context.Context, _ uuid.UUID, property domain.Property) (domain.Property, error) {
	r.data[property.ID] = property
	return property, nil
}

func (r *fakePropertyRepo) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (domain.Property, error) {
	p, ok := r.data[id]
	if !ok {
		return domain.Property{}, ErrNotFound
	}
	return p, nil
}

func (r *fakePropertyRepo) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Property, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakePropertyRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	return r.GetByIDAndOwner(ctx, id, uuid.Nil)
}

func (r *fakePropertyRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	return r.GetByIDAndOwner(ctx, id, uuid.Nil)
}

func (r *fakePropertyRepo) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	active := make([]domain.Property, 0, len(r.data))
	for _, p := range r.data {
		if p.Status != domain.PropertyStatusArchived {
			active = append(active, p)
		}
	}
	return active, nil
}

func (r *fakePropertyRepo) ListArchivedByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	archived := make([]domain.Property, 0)
	for _, p := range r.data {
		if p.Status == domain.PropertyStatusArchived {
			archived = append(archived, p)
		}
	}
	return archived, nil
}

func (r *fakePropertyRepo) SearchVisible(_ context.Context, actor uuid.UUID, q PropertySearchQuery) ([]domain.Property, error) {
	matches := make([]domain.Property, 0, len(r.data))
	for _, p := range r.data {
		if p.OwnerID != actor || p.Status == domain.PropertyStatusArchived {
			continue
		}
		if q.Search != "" && !strings.Contains(p.Name, q.Search) && !strings.Contains(p.Address, q.Search) {
			continue
		}
		matches = append(matches, p)
	}
	if len(matches) > int(q.Limit) {
		matches = matches[:q.Limit]
	}
	return matches, nil
}

func (r *fakePropertyRepo) Update(_ context.Context, _ uuid.UUID, property domain.Property) (domain.Property, error) {
	if _, ok := r.data[property.ID]; !ok {
		return domain.Property{}, ErrNotFound
	}
	r.data[property.ID] = property
	return property, nil
}

func (r *fakePropertyRepo) SetPin(_ context.Context, id, _ uuid.UUID, pinnedAt *time.Time) (domain.Property, error) {
	p, ok := r.data[id]
	if !ok {
		return domain.Property{}, ErrNotFound
	}
	p.PinnedAt = pinnedAt
	r.data[id] = p
	return p, nil
}

func (r *fakePropertyRepo) Archive(_ context.Context, id, _ uuid.UUID) error {
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	p.Status = domain.PropertyStatusArchived
	r.data[id] = p
	return nil
}

func (r *fakePropertyRepo) Unarchive(_ context.Context, id, _ uuid.UUID) error {
	p, ok := r.data[id]
	if !ok {
		return ErrNotFound
	}
	p.Status = domain.PropertyStatusActive
	r.data[id] = p
	return nil
}

func (r *fakePropertyRepo) CountActiveByOwner(_ context.Context, _ uuid.UUID) (int, error) {
	count := 0
	for _, p := range r.data {
		if p.Status != domain.PropertyStatusArchived {
			count++
		}
	}
	return count, nil
}

func (r *fakePropertyRepo) CountByOwnerAndType(_ context.Context, scope uuid.UUID, propertyType domain.PropertyType) (int, error) {
	count := 0
	for _, p := range r.data {
		if p.OwnerID == scope && p.Type == propertyType {
			count++
		}
	}
	return count, nil
}

func (r *fakePropertyRepo) Delete(_ context.Context, id, _ uuid.UUID) error {
	if _, ok := r.data[id]; !ok {
		return ErrNotFound
	}
	delete(r.data, id)
	return nil
}

func (r *fakePropertyRepo) WithTx(_ transaction.Tx) PropertyRepository {
	return r
}

var _ PropertyRepository = (*fakePropertyRepo)(nil)

func TestPropertyService_ArchiveExcessProperties_ArchivesExcess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	keepAID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	keepBID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	excessID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: keepAID, OwnerID: ownerID, Name: "Keep A", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: keepBID, OwnerID: ownerID, Name: "Keep B", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: excessID, OwnerID: ownerID, Name: "Excess", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Date(2026,
			6,
			10,
			0,
			0,
			0,
			0,
			time.UTC)},
		testOwnerPolicy{},
		nil,
	)

	if _, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 2, nil); err != nil {
		t.Fatalf("ArchiveExcessProperties failed: %v", err)
	}

	excess, err := repo.GetByIDAndOwner(ctx, excessID, ownerID)
	if err != nil {
		t.Fatalf("get excess property: %v", err)
	}
	if excess.Status != domain.PropertyStatusArchived {
		t.Errorf("expected excess property archived, got %q", excess.Status)
	}
	for _, id := range []uuid.UUID{keepAID, keepBID} {
		kept, err := repo.GetByIDAndOwner(ctx, id, ownerID)
		if err != nil {
			t.Fatalf("get kept property %s: %v", id, err)
		}
		if kept.Status == domain.PropertyStatusArchived {
			t.Errorf("property %s should stay active, got %q", id, kept.Status)
		}
	}
}

func TestPropertyService_ArchiveExcessProperties_WithinLimitDoesNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyAID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	propertyBID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: propertyAID, OwnerID: ownerID, Name: "A", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: propertyBID, OwnerID: ownerID, Name: "B", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)

	if _, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 2, nil); err != nil {
		t.Fatalf("ArchiveExcessProperties failed: %v", err)
	}
	for _, id := range []uuid.UUID{propertyAID, propertyBID} {
		p, err := repo.GetByIDAndOwner(ctx, id, ownerID)
		if err != nil {
			t.Fatalf("get property %s: %v", id, err)
		}
		if p.Status != domain.PropertyStatusActive {
			t.Errorf("property %s should stay active, got %q", id, p.Status)
		}
	}
}

// TestGetProperty_AccessOutcomes verifies the page-entry privacy policy (issue
// #156 T3, T9): the owner and active members read the object; a missing
// membership (including a revoked one) is indistinguishable from a missing
// object (ErrNotFound); a suspended membership is the single exception and
// yields ErrAccessSuspended so the transport can answer 403 with the
// membership_suspended code.
func TestGetProperty_AccessOutcomes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	tests := []struct {
		name    string
		role    sharedpolicy.Role
		wantErr error
	}{
		{"owner reads the object", sharedpolicy.RoleOwner, nil},
		{"active full-access member reads the object", sharedpolicy.RoleFullAccess, nil},
		{"active viewer reads the object", sharedpolicy.RoleViewer, nil},
		{"no membership looks like not found", sharedpolicy.RoleNone, ErrNotFound},
		{"revoked membership looks like not found", sharedpolicy.RoleNone, ErrNotFound},
		{"suspended membership signals access suspended", sharedpolicy.RoleSuspended, ErrAccessSuspended},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakePropertyRepo(
				domain.Property{
					ID: propertyID, OwnerID: ownerID, Name: testObjName, Address: testPropertyAddress,
					Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
				},
			)
			svc := NewPropertyService(
				repo,
				fakePropertyPhotoRepo{},
				fakePropertyPhotoStorage{},
				newPropertyTestFactory(repo,
					fakePropertyPhotoRepo{},
					nil),
				fakePropertyClock{now: time.Now()},
				staticRolePolicy{role: tt.role},
				nil,
			)

			_, err := svc.GetProperty(ctx, ownerID, propertyID)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetProperty error = %v, want %v", err, tt.wantErr)
			}

			// The GetProperty denial must propagate unchanged.
			if tt.wantErr != nil {
				_, err = svc.GetProperty(ctx, ownerID, propertyID)
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GetProperty error = %v, want %v", err, tt.wantErr)
				}
			}
		})
	}
}

// recordingSlotPolicy records the RecipientSlotPolicy calls so tests can assert
// which properties triggered a suspended-membership recovery.
type recordingSlotPolicy struct {
	recovered           []uuid.UUID
	recoveredRecipients []uuid.UUID
	enforcedOnUnarchive []uuid.UUID
}

func (p *recordingSlotPolicy) RecoverSuspendedForProperty(
	_ context.Context, _ transaction.Tx, propertyID uuid.UUID,
) ([]accessdomain.Membership, error) {
	p.recovered = append(p.recovered, propertyID)
	return nil, nil
}

func (p *recordingSlotPolicy) EnforceOnUnarchiveForProperty(
	_ context.Context, _ transaction.Tx, propertyID uuid.UUID,
) ([]accessdomain.Membership, error) {
	p.enforcedOnUnarchive = append(p.enforcedOnUnarchive, propertyID)
	return nil, nil
}

func (p *recordingSlotPolicy) RecoverAfterPropertyDelete(context.Context, transaction.Tx, uuid.UUID) ([]accessdomain.Membership, error) {
	return nil, nil
}

func (p *recordingSlotPolicy) RecoverSuspended(
	_ context.Context, _ transaction.Tx, recipientID uuid.UUID,
) ([]accessdomain.Membership, error) {
	p.recoveredRecipients = append(p.recoveredRecipients, recipientID)
	return nil, nil
}

var _ RecipientSlotPolicy = (*recordingSlotPolicy)(nil)

// TestPropertyService_ArchiveExcessProperties_RecoversSuspendedMembers verifies
// that the tariff auto-archive behaves like a manual archive for shared-access
// members (issue #163): every archived property triggers a FIFO recovery of its
// recipients' suspended memberships.
func TestPropertyService_ArchiveExcessProperties_RecoversSuspendedMembers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	keepID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	excessAID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	excessBID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: keepID, OwnerID: ownerID, Name: "Keep", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: excessAID, OwnerID: ownerID, Name: excessNameA, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: excessBID, OwnerID: ownerID, Name: excessNameB, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	)
	slots := &recordingSlotPolicy{}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Date(2026,
			6,
			10,
			0,
			0,
			0,
			0,
			time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(slots)

	if _, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 1, nil); err != nil {
		t.Fatalf("ArchiveExcessProperties failed: %v", err)
	}

	// Both excess properties were archived and each triggered a recovery.
	want := map[uuid.UUID]bool{excessAID: true, excessBID: true}
	if len(slots.recovered) != len(want) {
		t.Fatalf("expected RecoverSuspendedForProperty for %v, got %v", want, slots.recovered)
	}
	for _, id := range slots.recovered {
		if !want[id] {
			t.Errorf("unexpected RecoverSuspendedForProperty for %s (all calls: %v)", id, slots.recovered)
		}
	}
}

// scopedPropertyRepo wraps fakePropertyRepo with owner-scoped list semantics:
// production repositories filter rows by the data owner (scope), while the
// map-based fake predates sharing and returns every row. The access-context
// tests (issue T11) need the production filtering to distinguish the actor's
// own properties from the ones shared with them.
type scopedPropertyRepo struct {
	*fakePropertyRepo
}

func (r scopedPropertyRepo) ListActiveByOwner(ctx context.Context, scope uuid.UUID) ([]domain.Property, error) {
	all, err := r.fakePropertyRepo.ListActiveByOwner(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Property, 0, len(all))
	for _, p := range all {
		if p.OwnerID == scope {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r scopedPropertyRepo) ListArchivedByOwner(ctx context.Context, scope uuid.UUID) ([]domain.Property, error) {
	all, err := r.fakePropertyRepo.ListArchivedByOwner(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Property, 0, len(all))
	for _, p := range all {
		if p.OwnerID == scope {
			out = append(out, p)
		}
	}
	return out, nil
}

var _ PropertyRepository = scopedPropertyRepo{}

// TestListProperties_AccessRoles verifies the actor's access context on the
// list endpoint (issue T11): own properties carry RoleOwner, shared ones
// carry the membership role, and an accidental self-membership never demotes
// the owner. The owner display name is never resolved on the list path.
func TestListProperties_AccessRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherOwnerID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sharedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	brokenID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: ownID, OwnerID: ownerID, Name: testPropertyNameOwn, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: sharedID, OwnerID: otherOwnerID, Name: testPropertyNameShared, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: brokenID, OwnerID: otherOwnerID, Name: "Broken", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
		{PropertyID: sharedID, Role: sharedpolicy.RoleViewer},
		// An accidental self-membership must not demote the owner.
		{PropertyID: ownID, Role: sharedpolicy.RoleViewer},
		// A membership whose DB role degraded to RoleNone (corrupt data) is
		// skipped so "none" never leaks into the API contract.
		{PropertyID: brokenID, Role: sharedpolicy.RoleNone},
	}})

	result, err := svc.ListProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListProperties failed: %v", err)
	}
	byID := make(map[uuid.UUID]domain.Property, len(result.Items))
	for _, p := range result.Items {
		byID[p.ID] = p
	}
	if len(byID) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(byID))
	}
	if got := byID[ownID].AccessRole; got != sharedpolicy.RoleOwner {
		t.Errorf("own property AccessRole = %q, want %q", got, sharedpolicy.RoleOwner)
	}
	if got := byID[sharedID].AccessRole; got != sharedpolicy.RoleViewer {
		t.Errorf("shared property AccessRole = %q, want %q", got, sharedpolicy.RoleViewer)
	}
	if _, ok := byID[brokenID]; ok {
		t.Errorf("membership with role %q must be excluded from the list", sharedpolicy.RoleNone)
	}
	if got := byID[sharedID].OwnerName; got != "" {
		t.Errorf("list path must not resolve the owner name, got %q", got)
	}
}

// TestListArchivedProperties_AccessRoles verifies that the archived list
// carries the same access context as the active list (issue T11).
func TestListArchivedProperties_AccessRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherOwnerID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sharedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	brokenID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: ownID, OwnerID: ownerID, Name: testPropertyNameOwn, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
		domain.Property{
			ID: sharedID, OwnerID: otherOwnerID, Name: testPropertyNameShared, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
		domain.Property{
			ID: brokenID, OwnerID: otherOwnerID, Name: "Broken", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
		{PropertyID: sharedID, Role: sharedpolicy.RoleFullAccess},
		// A membership whose DB role degraded to RoleNone (corrupt data) is
		// skipped so "none" never leaks into the API contract.
		{PropertyID: brokenID, Role: sharedpolicy.RoleNone},
	}})

	result, err := svc.ListArchivedProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListArchivedProperties failed: %v", err)
	}
	byID := make(map[uuid.UUID]domain.Property, len(result.Items))
	for _, p := range result.Items {
		byID[p.ID] = p
	}
	if len(byID) != 2 {
		t.Fatalf("expected 2 archived properties, got %d", len(byID))
	}
	if got := byID[ownID].AccessRole; got != sharedpolicy.RoleOwner {
		t.Errorf("own archived AccessRole = %q, want %q", got, sharedpolicy.RoleOwner)
	}
	if got := byID[sharedID].AccessRole; got != sharedpolicy.RoleFullAccess {
		t.Errorf("shared archived AccessRole = %q, want %q", got, sharedpolicy.RoleFullAccess)
	}
	if _, ok := byID[brokenID]; ok {
		t.Errorf("membership with role %q must be excluded from the archived list", sharedpolicy.RoleNone)
	}
}

// Test fixture display strings of the ticket #702 list tests (goconst).
const (
	testMemberNameMaria    = "Мария Иванова"
	testPropertyNameOwn    = "Own"
	testPropertyNameShared = "Shared"
)

// TestListProperties_SharedRowOwnerName verifies the owner-name projection
// of the shared list rows (owner decision on the #756 walkthrough fixes):
// a shared row carries the property owner's display name — the card shows
// whose object it is; own rows and an unwired resolver keep none.
func TestListProperties_SharedRowOwnerName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherOwnerID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sharedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	newSvc := func(t *testing.T) *PropertyService {
		t.Helper()
		repo := scopedPropertyRepo{newFakePropertyRepo(
			domain.Property{
				ID: ownID, OwnerID: ownerID, Name: testPropertyNameOwn, Address: testPropertyAddress,
				Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
			},
			domain.Property{
				ID: sharedID, OwnerID: otherOwnerID, Name: testPropertyNameShared, Address: testPropertyAddress,
				Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
			},
		)}
		svc := NewPropertyService(
			repo,
			fakePropertyPhotoRepo{},
			fakePropertyPhotoStorage{},
			newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
			fakePropertyClock{now: time.Now()},
			testOwnerPolicy{},
			nil,
		)
		svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
			{PropertyID: sharedID, Role: sharedpolicy.RoleViewer},
		}})
		return svc
	}

	t.Run("shared rows carry the owner name, own rows do not", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(t)
		svc.SetOwnerDisplayNameResolver(fakeOwnerNames{names: map[uuid.UUID]string{
			otherOwnerID: testMemberNameMaria,
		}})

		result, err := svc.ListProperties(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListProperties failed: %v", err)
		}
		byID := make(map[uuid.UUID]domain.Property, len(result.Items))
		for _, p := range result.Items {
			byID[p.ID] = p
		}
		if got := byID[sharedID].OwnerName; got != testMemberNameMaria {
			t.Errorf("shared row OwnerName = %q, want %q", got, testMemberNameMaria)
		}
		if got := byID[ownID].OwnerName; got != "" {
			t.Errorf("own row OwnerName = %q, want empty", got)
		}
	})

	t.Run("unwired resolver leaves the name empty", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(t)

		result, err := svc.ListProperties(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListProperties failed: %v", err)
		}
		for _, p := range result.Items {
			if p.OwnerName != "" {
				t.Errorf("row %s OwnerName = %q, want empty without the port", p.ID, p.OwnerName)
			}
		}
	})
}

// TestListArchivedProperties_SharedRowOwnerName verifies that the archived
// list carries the same owner-name projection on its shared rows, and that
// it never carries suspended placeholders — archived objects hide their
// suspended legs behind the archive (#163).
func TestListArchivedProperties_SharedRowOwnerName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherOwnerID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	sharedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	repo := scopedPropertyRepo{newFakePropertyRepo(
		domain.Property{
			ID: sharedID, OwnerID: otherOwnerID, Name: testPropertyNameShared, Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
		{PropertyID: sharedID, Role: sharedpolicy.RoleViewer},
	}})
	svc.SetOwnerDisplayNameResolver(fakeOwnerNames{names: map[uuid.UUID]string{
		otherOwnerID: testMemberNameMaria,
	}})
	svc.SetSuspendedSharedMemberships(fakeSuspendedShared{items: []SharedSuspendedMembership{
		{
			PropertyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			Role:       sharedpolicy.RoleViewer,
			OwnerName:  testMemberNameMaria,
			OwnerEmail: "maria@example.com",
		},
	}})

	result, err := svc.ListArchivedProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListArchivedProperties failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 archived property, got %d", len(result.Items))
	}
	if got := result.Items[0].OwnerName; got != testMemberNameMaria {
		t.Errorf("archived shared row OwnerName = %q, want %q", got, testMemberNameMaria)
	}
	if len(result.SuspendedShared) != 0 {
		t.Errorf("archived list must not carry suspended placeholders, got %v", result.SuspendedShared)
	}
}

// TestListProperties_SuspendedShared verifies the blur-card placeholders
// (ticket #702): the page carries the actor's suspended shared memberships
// as the port reports them — no object data, owner contact only; an unwired
// port carries none, and a placeholder never leaks into the list items.
func TestListProperties_SuspendedShared(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	suspendedID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	newSvc := func(t *testing.T) *PropertyService {
		t.Helper()
		repo := scopedPropertyRepo{newFakePropertyRepo(
			domain.Property{
				ID: ownID, OwnerID: ownerID, Name: testPropertyNameOwn, Address: testPropertyAddress,
				Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
			},
		)}
		return NewPropertyService(
			repo,
			fakePropertyPhotoRepo{},
			fakePropertyPhotoStorage{},
			newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
			fakePropertyClock{now: time.Now()},
			testOwnerPolicy{},
			nil,
		)
	}

	want := []SharedSuspendedMembership{
		{
			PropertyID: suspendedID,
			Role:       sharedpolicy.RoleViewer,
			OwnerName:  testMemberNameMaria,
			OwnerEmail: "maria@example.com",
		},
	}

	t.Run("wired port fills the page", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(t)
		svc.SetSuspendedSharedMemberships(fakeSuspendedShared{items: want})

		result, err := svc.ListProperties(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListProperties failed: %v", err)
		}
		if !slices.Equal(result.SuspendedShared, want) {
			t.Errorf("SuspendedShared = %v, want %v", result.SuspendedShared, want)
		}
		for _, p := range result.Items {
			if p.ID == suspendedID {
				t.Errorf("suspended property %s leaked into the list items", p.ID)
			}
		}
	})

	t.Run("unwired port carries none", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(t)

		result, err := svc.ListProperties(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListProperties failed: %v", err)
		}
		if len(result.SuspendedShared) != 0 {
			t.Errorf("SuspendedShared = %v, want empty without the port", result.SuspendedShared)
		}
	})
}

// TestGetProperty_AccessContext verifies the actor's access context on the
// detail endpoint (issue T11): the owner gets RoleOwner and no owner name;
// a recipient gets the membership role and the owner's public display name.
// A resolver failure is logged and degrades to an empty name, never to a
// failed request. The owner's account email follows the same shape
// (Figma 2200-97365, the detail's owner contact row): a deliberate exposure
// for recipients only, degrading to empty.
func TestGetProperty_AccessContext(t *testing.T) {
	t.Parallel()

	// Фикстура отображаемого имени владельца.
	const testOwnerName = "Ivan Petrov"

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	property := domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testObjName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
	}

	newSvc := func(role sharedpolicy.Role, resolver OwnerDisplayNameResolver) *PropertyService {
		repo := newFakePropertyRepo(property)
		svc := NewPropertyService(
			repo,
			fakePropertyPhotoRepo{},
			fakePropertyPhotoStorage{},
			newPropertyTestFactory(repo,
				fakePropertyPhotoRepo{},
				nil),
			fakePropertyClock{now: time.Now()},
			staticRolePolicy{role: role},
			nil,
		)
		if resolver != nil {
			svc.SetOwnerDisplayNameResolver(resolver)
		}
		return svc
	}

	t.Run("owner gets owner role and no owner name", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleOwner, fakeOwnerNames{names: map[uuid.UUID]string{ownerID: testOwnerName}})
		p, err := svc.GetProperty(ctx, ownerID, propertyID)
		if err != nil {
			t.Fatalf("GetProperty failed: %v", err)
		}
		if p.AccessRole != sharedpolicy.RoleOwner {
			t.Errorf("AccessRole = %q, want %q", p.AccessRole, sharedpolicy.RoleOwner)
		}
		if p.OwnerName != "" {
			t.Errorf("owner must not get an owner name, got %q", p.OwnerName)
		}
	})

	t.Run("recipient gets membership role and owner name", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleFullAccess, fakeOwnerNames{names: map[uuid.UUID]string{ownerID: testOwnerName}})
		p, err := svc.GetProperty(ctx, recipientID, propertyID)
		if err != nil {
			t.Fatalf("GetProperty failed: %v", err)
		}
		if p.AccessRole != sharedpolicy.RoleFullAccess {
			t.Errorf("AccessRole = %q, want %q", p.AccessRole, sharedpolicy.RoleFullAccess)
		}
		if p.OwnerName != testOwnerName {
			t.Errorf("OwnerName = %q, want %q", p.OwnerName, testOwnerName)
		}
	})

	t.Run("recipient without resolver gets no owner name", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleViewer, nil)
		p, err := svc.GetProperty(ctx, recipientID, propertyID)
		if err != nil {
			t.Fatalf("GetProperty failed: %v", err)
		}
		if p.AccessRole != sharedpolicy.RoleViewer {
			t.Errorf("AccessRole = %q, want %q", p.AccessRole, sharedpolicy.RoleViewer)
		}
		if p.OwnerName != "" {
			t.Errorf("OwnerName = %q, want empty without a resolver", p.OwnerName)
		}
	})

	t.Run("resolver error degrades to an empty owner name", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleViewer, fakeOwnerNames{err: errors.New("lookup failed")})
		p, err := svc.GetProperty(ctx, recipientID, propertyID)
		if err != nil {
			t.Fatalf("resolver error must not fail the request: %v", err)
		}
		if p.OwnerName != "" {
			t.Errorf("OwnerName = %q, want empty on resolver error", p.OwnerName)
		}
	})
}

// TestGetProperty_OwnerEmail verifies the owner's account email in the
// detail's access context (Figma 2200-97365, the owner contact row): filled
// for recipients only, empty for the owner, degrading to empty on a resolver
// failure or when the port is unwired.
func TestGetProperty_OwnerEmail(t *testing.T) {
	t.Parallel()

	// Фикстура почты владельца (Figma 2200-97365).
	const testOwnerEmail = "ivan@example.com"

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recipientID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	property := domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testObjName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
	}

	newSvc := func(role sharedpolicy.Role, resolver OwnerEmailResolver) *PropertyService {
		repo := newFakePropertyRepo(property)
		svc := NewPropertyService(
			repo,
			fakePropertyPhotoRepo{},
			fakePropertyPhotoStorage{},
			newPropertyTestFactory(repo,
				fakePropertyPhotoRepo{},
				nil),
			fakePropertyClock{now: time.Now()},
			staticRolePolicy{role: role},
			nil,
		)
		if resolver != nil {
			svc.SetOwnerEmailResolver(resolver)
		}
		return svc
	}

	t.Run("recipient gets the owner email", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleFullAccess, fakeOwnerEmails{emails: map[uuid.UUID]string{
			ownerID: testOwnerEmail,
		}})
		p, err := svc.GetProperty(ctx, recipientID, propertyID)
		if err != nil {
			t.Fatalf("GetProperty failed: %v", err)
		}
		if p.OwnerEmail != testOwnerEmail {
			t.Errorf("OwnerEmail = %q, want %q", p.OwnerEmail, testOwnerEmail)
		}
	})

	t.Run("owner gets no owner email", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleOwner, fakeOwnerEmails{emails: map[uuid.UUID]string{
			ownerID: testOwnerEmail,
		}})
		p, err := svc.GetProperty(ctx, ownerID, propertyID)
		if err != nil {
			t.Fatalf("GetProperty failed: %v", err)
		}
		if p.OwnerEmail != "" {
			t.Errorf("owner must not get an owner email, got %q", p.OwnerEmail)
		}
	})

	t.Run("recipient without the port gets an empty owner email", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleViewer, nil)
		p, err := svc.GetProperty(ctx, recipientID, propertyID)
		if err != nil {
			t.Fatalf("GetProperty failed: %v", err)
		}
		if p.OwnerEmail != "" {
			t.Errorf("OwnerEmail = %q, want empty without a resolver", p.OwnerEmail)
		}
	})

	t.Run("resolver error degrades to an empty owner email", func(t *testing.T) {
		t.Parallel()
		svc := newSvc(sharedpolicy.RoleViewer, fakeOwnerEmails{err: errors.New("lookup failed")})
		p, err := svc.GetProperty(ctx, recipientID, propertyID)
		if err != nil {
			t.Fatalf("resolver error must not fail the request: %v", err)
		}
		if p.OwnerEmail != "" {
			t.Errorf("OwnerEmail = %q, want empty on resolver error", p.OwnerEmail)
		}
	})
}

// TestPropertyService_ArchiveProperty_RecoversSuspendedForOwnerRecipient
// verifies that archiving one's OWN object frees one of the owner's tariff
// slots and triggers a per-recipient recovery of the owner's own suspended
// shared memberships. The owner is never a member row of their own object, so
// the per-property recovery (RecoverSuspendedForProperty) does not visit them;
// ArchiveProperty must additionally call RecoverSuspended(ownerID). See issue
// #158 (T4).
func TestPropertyService_ArchiveProperty_RecoversSuspendedForOwnerRecipient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: propertyID, OwnerID: ownerID, Name: "Своя квартира", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)
	slots := &recordingSlotPolicy{}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Date(2026,
			6,
			10,
			0,
			0,
			0,
			0,
			time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(slots)

	if _, err := svc.ArchiveProperty(ctx, ownerID, propertyID); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}

	// The per-property recovery runs (for the archived object's members)...
	if len(slots.recovered) == 0 {
		t.Errorf("expected RecoverSuspendedForProperty to run for the archived object, got %v", slots.recovered)
	}
	// ...AND the owner's own suspended queue is recovered, because archiving an
	// own object freed one of the owner's tariff slots.
	if !slices.Contains(slots.recoveredRecipients, ownerID) {
		t.Errorf("expected RecoverSuspended(ownerID) for the archiving owner, got recoveredRecipients = %v", slots.recoveredRecipients)
	}
}

// TestPropertyService_DeleteProperty_RecoversSuspendedForOwnerRecipient is the
// delete-side counterpart: deleting one's OWN object frees one of the owner's
// tariff slots, so DeleteProperty must call RecoverSuspended(ownerID) in
// addition to the per-property RecoverAfterPropertyDelete.
func TestPropertyService_DeleteProperty_RecoversSuspendedForOwnerRecipient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: propertyID, OwnerID: ownerID, Name: "Своя квартира", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
	)
	slots := &recordingSlotPolicy{}
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Date(2026,
			6,
			10,
			0,
			0,
			0,
			0,
			time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	svc.SetRecipientSlotPolicy(slots)

	if err := svc.DeleteProperty(ctx, ownerID, propertyID); err != nil {
		t.Fatalf("DeleteProperty: %v", err)
	}

	if !slices.Contains(slots.recoveredRecipients, ownerID) {
		t.Errorf("expected RecoverSuspended(ownerID) for the deleting owner, got recoveredRecipients = %v", slots.recoveredRecipients)
	}
}
