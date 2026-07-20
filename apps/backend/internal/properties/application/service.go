package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"sort"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

const (
	maxPhotoCount  = 10
	maxPhotoSize   = 5 * 1024 * 1024 // 5 MiB
	photoKeyPrefix = "properties"
)

var allowedPhotoContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type CreatePropertyCommand struct {
	Name        string
	Type        string
	Address     string
	Description string
}

type UpdatePropertyCommand struct {
	Name        *string
	Type        *string
	Address     *string
	Description *string
	Status      *string
}

type PropertyService struct {
	repo              PropertyRepository
	photoRepo         PropertyPhotoRepository
	photoStorage      PhotoStorage
	occupancyProvider OccupancyProvider
	limiter           SubscriptionLimiter
	billingLifecycle  PropertyBillingLifecycle
	leaseRepo         LeaseRepository
	db                txBeginner
	audit             auditapp.Recorder
	clock             clock.Clock
	logger            *slog.Logger
}

func NewPropertyService(
	repo PropertyRepository,
	photoRepo PropertyPhotoRepository,
	photoStorage PhotoStorage,
	occupancyProvider OccupancyProvider,
	limiter SubscriptionLimiter,
	billingLifecycle PropertyBillingLifecycle,
	leaseRepo LeaseRepository,
	db txBeginner,
	audit auditapp.Recorder,
	clock clock.Clock,
	logger *slog.Logger,
) *PropertyService {
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &PropertyService{
		repo:              repo,
		photoRepo:         photoRepo,
		photoStorage:      photoStorage,
		occupancyProvider: occupancyProvider,
		limiter:           limiter,
		billingLifecycle:  billingLifecycle,
		leaseRepo:         leaseRepo,
		db:                db,
		audit:             audit,
		clock:             clock,
		logger:            logger,
	}
}

func (s *PropertyService) CreateProperty(ctx context.Context, ownerID uuid.UUID, cmd CreatePropertyCommand) (domain.Property, error) {
	propertyType, err := domain.ParsePropertyType(cmd.Type)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
	}

	property, err := domain.NewProperty(ownerID, cmd.Name, cmd.Address, cmd.Description, propertyType)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	now := s.clock.Now()
	property.CreatedAt = now
	property.UpdatedAt = now

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)
	txLimiter, err := s.limiter.WithTx(tx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("bind limiter transaction: %w", err)
	}

	limit, err := txLimiter.ActivePropertyLimit(ctx, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("get active property limit: %w", err)
	}

	count, err := txRepo.CountActiveByOwner(ctx, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("count active properties: %w", err)
	}
	if count >= limit {
		return domain.Property{}, ErrLimitExceeded
	}

	created, err := txRepo.Create(ctx, ownerID, property)
	if err != nil {
		return domain.Property{}, fmt.Errorf("create property: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyCreated,
		EntityType: auditdomain.EntityProperty,
		EntityID:   &created.ID,
		Context:    map[string]any{"name": created.Name, "type": string(created.Type)},
	}); err != nil {
		return domain.Property{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	created.Occupancy = domain.OccupancyFree
	created.Photos = []domain.Photo{}
	return created, nil
}

func (s *PropertyService) ListProperties(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error) {
	properties, err := s.repo.ListActiveByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}

	occupied, err := s.occupancyProvider.OccupiedPropertyIDs(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("check occupancy: %w", err)
	}

	for i := range properties {
		if occupied[properties[i].ID] {
			properties[i].Occupancy = domain.OccupancyOccupied
		} else {
			properties[i].Occupancy = domain.OccupancyFree
		}
	}

	properties, err = s.withPhotos(ctx, properties...)
	if err != nil {
		return nil, err
	}

	return properties, nil
}

func (s *PropertyService) ListArchivedProperties(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error) {
	properties, err := s.repo.ListArchivedByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list archived properties: %w", err)
	}

	properties, err = s.withPhotos(ctx, properties...)
	if err != nil {
		return nil, err
	}

	return properties, nil
}

func (s *PropertyService) GetProperty(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, error) {
	property, err := s.repo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	occupied, err := s.occupancyProvider.IsOccupied(ctx, ownerID, property.ID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
	}
	if occupied {
		property.Occupancy = domain.OccupancyOccupied
	} else {
		property.Occupancy = domain.OccupancyFree
	}

	properties, err := s.withPhotos(ctx, property)
	if err != nil {
		return domain.Property{}, err
	}
	return properties[0], nil
}

func (s *PropertyService) GetPropertyWithOpenLease(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, leasesdomain.Lease, error) {
	property, err := s.GetProperty(ctx, ownerID, id)
	if err != nil {
		return domain.Property{}, leasesdomain.Lease{}, err
	}

	lease, err := s.leaseRepo.GetOpenLeaseByProperty(ctx, ownerID, property.ID)
	if err != nil {
		if errors.Is(err, leasesapp.ErrNotFound) {
			return property, leasesdomain.Lease{}, nil
		}
		return domain.Property{}, leasesdomain.Lease{}, fmt.Errorf("get open lease: %w", err)
	}

	now := s.clock.Now()
	lease.Status = lease.EffectiveStatus(now)
	return property, lease, nil
}

func (s *PropertyService) ListPropertyLeases(ctx context.Context, ownerID, propertyID uuid.UUID) ([]leasesdomain.Lease, error) {
	if _, err := s.repo.GetByIDAndOwner(ctx, propertyID, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get property: %w", err)
	}

	leases, err := s.leaseRepo.ListByProperty(ctx, ownerID, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list property leases: %w", err)
	}

	now := s.clock.Now()
	for i := range leases {
		leases[i].Status = leases[i].EffectiveStatus(now)
	}
	return leases, nil
}

func (s *PropertyService) UpdateProperty(ctx context.Context, ownerID, id uuid.UUID, cmd UpdatePropertyCommand) (domain.Property, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)
	txOccupancy := s.occupancyProvider.WithTx(tx)

	property, err := txRepo.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		return domain.Property{}, ErrArchivedProperty
	}

	if cmd.Name != nil {
		property.Name = *cmd.Name
	}
	if cmd.Address != nil {
		property.Address = *cmd.Address
	}
	if cmd.Description != nil {
		property.Description = *cmd.Description
	}
	if cmd.Type != nil {
		propertyType, err := domain.ParsePropertyType(*cmd.Type)
		if err != nil {
			return domain.Property{}, fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
		}
		property.Type = propertyType
	}
	if cmd.Status != nil {
		status, err := domain.ParsePropertyStatus(*cmd.Status)
		if err != nil {
			return domain.Property{}, fmt.Errorf("%w: invalid property status: %w", ErrInvalidInput, err)
		}
		if !isUpdatableStatusTransition(property.Status, status) {
			return domain.Property{}, &InvalidStatusTransitionError{From: property.Status, To: status}
		}
		if status == domain.PropertyStatusMaintenance && property.Status == domain.PropertyStatusActive {
			occupied, err := txOccupancy.IsOccupied(ctx, ownerID, property.ID)
			if err != nil {
				return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
			}
			if occupied {
				return domain.Property{}, ErrPropertyHasOpenLease
			}
		}
		property.Status = status
	}

	if err := property.Validate(); err != nil {
		return domain.Property{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	property.UpdatedAt = s.clock.Now()

	updated, err := txRepo.Update(ctx, ownerID, property)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("update property: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyUpdated,
		EntityType: auditdomain.EntityProperty,
		EntityID:   &id,
		Context:    map[string]any{"fields": updatedPropertyFields(cmd)},
	}); err != nil {
		return domain.Property{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	properties, err := s.withPhotos(ctx, updated)
	if err != nil {
		return domain.Property{}, err
	}
	return properties[0], nil
}

func (s *PropertyService) ArchiveProperty(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	archived, err := s.archivePropertyInTx(
		ctx,
		s.repo.WithTx(tx),
		s.occupancyProvider.WithTx(tx),
		s.billingLifecycle.WithTx(tx),
		ownerID,
		id,
		false,
	)
	if err != nil {
		return domain.Property{}, err
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyArchived,
		EntityType: auditdomain.EntityProperty,
		EntityID:   &id,
	}); err != nil {
		return domain.Property{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	properties, err := s.withPhotos(ctx, archived)
	if err != nil {
		return domain.Property{}, err
	}
	return properties[0], nil
}

// archivePropertyInTx performs the core archive logic inside an existing
// transaction. The caller is responsible for committing or rolling back tx.
// When forceCompleteLeases is false, a property with an open lease is rejected
// with ErrPropertyHasOpenLease; when true, open leases are force-completed
// with the same side effects as a user-initiated lease completion before the
// property is archived.
func (s *PropertyService) archivePropertyInTx(
	ctx context.Context,
	repo PropertyRepository,
	occupancy OccupancyProvider,
	billing PropertyBillingLifecycle,
	ownerID, id uuid.UUID,
	forceCompleteLeases bool,
) (domain.Property, error) {
	property, err := repo.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		return domain.Property{}, ErrAlreadyArchived
	}

	occupied, err := occupancy.IsOccupied(ctx, ownerID, property.ID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
	}
	if occupied && !forceCompleteLeases {
		return domain.Property{}, ErrPropertyHasOpenLease
	}

	now := s.clock.Now()
	if err := billing.CompleteOpenLeases(ctx, ownerID, id, now); err != nil {
		return domain.Property{}, fmt.Errorf("complete open leases: %w", err)
	}

	if err := billing.Suspend(ctx, id, timeutil.Date(now)); err != nil {
		return domain.Property{}, fmt.Errorf("suspend billing: %w", err)
	}

	if err := repo.Archive(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("archive property: %w", err)
	}

	archived, err := repo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("reload archived property: %w", err)
	}

	return archived, nil
}

// ArchiveExcessProperties archives active properties beyond the given limit,
// keeping the most recently updated properties. Properties with an open lease
// have that lease force-completed (same side effects as a user-initiated lease
// completion) before archiving, so the tariff limit is always enforced.
func (s *PropertyService) ArchiveExcessProperties(ctx context.Context, tx transaction.Tx, ownerID uuid.UUID, limit int) error {
	if limit < 0 {
		return nil
	}

	txRepo := s.repo.WithTx(tx)
	properties, err := txRepo.ListActiveByOwner(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("list active properties: %w", err)
	}

	// Keep the most recently updated properties; archive the rest.
	sort.SliceStable(properties, func(i, j int) bool {
		return properties[i].UpdatedAt.After(properties[j].UpdatedAt)
	})

	if len(properties) <= limit {
		return nil
	}

	txOccupancy := s.occupancyProvider.WithTx(tx)
	txBillingLifecycle := s.billingLifecycle.WithTx(tx)
	txAudit := s.audit.WithTx(tx)

	for _, p := range properties[limit:] {
		_, err := s.archivePropertyInTx(ctx, txRepo, txOccupancy, txBillingLifecycle, ownerID, p.ID, true)
		if err != nil {
			if errors.Is(err, ErrAlreadyArchived) {
				s.logger.WarnContext(ctx, "skipping auto-archive of property",
					"property_id", p.ID.String(),
					"owner_id", ownerID.String(),
					"error", err.Error())
				continue
			}
			return fmt.Errorf("archive property %s: %w", p.ID, err)
		}
		// Fail-safe: an audit failure aborts the billing operation that
		// triggered the auto-archive.
		if err := txAudit.Record(ctx, auditdomain.Entry{
			ActorID:    &ownerID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyArchived,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &p.ID,
			Context:    map[string]any{"trigger": "billing_limit"},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
	}
	return nil
}

func (s *PropertyService) UnarchiveProperty(ctx context.Context, ownerID, id uuid.UUID) (domain.Property, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)
	txLimiter, err := s.limiter.WithTx(tx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("bind limiter transaction: %w", err)
	}

	property, err := txRepo.GetByIDAndOwnerForUpdate(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status != domain.PropertyStatusArchived {
		return domain.Property{}, ErrNotArchived
	}

	limit, err := txLimiter.ActivePropertyLimit(ctx, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("get active property limit: %w", err)
	}

	count, err := txRepo.CountActiveByOwner(ctx, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("count active properties: %w", err)
	}
	if count >= limit {
		return domain.Property{}, ErrLimitExceeded
	}

	if err := txRepo.Unarchive(ctx, id, ownerID); err != nil {
		return domain.Property{}, fmt.Errorf("unarchive property: %w", err)
	}

	if err := s.billingLifecycle.WithTx(tx).Resume(ctx, id, ownerID, timeutil.Date(s.clock.Now())); err != nil {
		return domain.Property{}, fmt.Errorf("resume billing: %w", err)
	}

	unarchived, err := txRepo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("reload unarchived property: %w", err)
	}

	occupied, err := s.occupancyProvider.WithTx(tx).IsOccupied(ctx, ownerID, unarchived.ID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("check occupancy: %w", err)
	}
	if occupied {
		unarchived.Occupancy = domain.OccupancyOccupied
	} else {
		unarchived.Occupancy = domain.OccupancyFree
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyUnarchived,
		EntityType: auditdomain.EntityProperty,
		EntityID:   &id,
	}); err != nil {
		return domain.Property{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	properties, err := s.withPhotos(ctx, unarchived)
	if err != nil {
		return domain.Property{}, err
	}
	return properties[0], nil
}

// AddPropertyPhoto validates and uploads a photo for the given property.
func (s *PropertyService) AddPropertyPhoto(ctx context.Context, ownerID, propertyID uuid.UUID, file io.Reader, filename string, contentType string, size int64) (domain.Property, error) {
	if _, ok := allowedPhotoContentTypes[contentType]; !ok {
		return domain.Property{}, fmt.Errorf("%w: unsupported content type %q", ErrInvalidInput, contentType)
	}
	if size > maxPhotoSize {
		return domain.Property{}, fmt.Errorf("%w: file size %d exceeds %d bytes", ErrInvalidInput, size, maxPhotoSize)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Property{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)
	txPhotoRepo := s.photoRepo.WithTx(tx)

	property, err := txRepo.GetByIDAndOwnerForUpdate(ctx, propertyID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		return domain.Property{}, ErrArchivedProperty
	}

	count, err := txPhotoRepo.CountByPropertyID(ctx, propertyID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("count photos: %w", err)
	}
	if count >= maxPhotoCount {
		return domain.Property{}, ErrPhotoLimitReached
	}

	photoID, err := uuid.NewV7()
	if err != nil {
		return domain.Property{}, fmt.Errorf("generate photo id: %w", err)
	}

	ext := allowedPhotoContentTypes[contentType]
	key := fmt.Sprintf("%s/%s/%s%s", photoKeyPrefix, propertyID.String(), photoID.String(), ext)

	url, err := s.photoStorage.Upload(ctx, key, contentType, size, file)
	if err != nil {
		return domain.Property{}, fmt.Errorf("upload photo: %w", err)
	}

	if _, err := txPhotoRepo.Create(ctx, photoID, propertyID, url); err != nil {
		return domain.Property{}, fmt.Errorf("create photo record: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyPhotoAdded,
		EntityType: auditdomain.EntityPropertyPhoto,
		EntityID:   &photoID,
		Context:    map[string]any{"property_id": propertyID},
	}); err != nil {
		return domain.Property{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Property{}, fmt.Errorf("commit tx: %w", err)
	}

	photos, err := s.photoRepo.GetByPropertyID(ctx, propertyID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("load photos: %w", err)
	}
	property.Photos = photos

	return property, nil
}

// DeletePropertyPhoto removes a photo record from the database and then deletes
// the file from storage on a best-effort basis. The DB record is the source of
// truth; if storage cleanup fails, the operation still succeeds and the orphan
// object is logged for later cleanup.
func (s *PropertyService) DeletePropertyPhoto(ctx context.Context, ownerID, propertyID, photoID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)
	txPhotoRepo := s.photoRepo.WithTx(tx)

	property, err := txRepo.GetByIDAndOwnerForUpdate(ctx, propertyID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		return ErrArchivedProperty
	}

	photo, err := txPhotoRepo.GetByIDAndPropertyID(ctx, photoID, propertyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get photo: %w", err)
	}

	if err := txPhotoRepo.Delete(ctx, photoID); err != nil {
		return fmt.Errorf("delete photo record: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &ownerID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyPhotoDeleted,
		EntityType: auditdomain.EntityPropertyPhoto,
		EntityID:   &photoID,
		Context:    map[string]any{"property_id": propertyID},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	key, err := photoStorageKey(propertyID, photoID, photo.URL)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to derive storage key for photo cleanup",
			"property_id", propertyID.String(),
			"photo_id", photoID.String(),
			"error", err.Error())
		return nil
	}

	if err := s.photoStorage.Delete(ctx, key); err != nil {
		s.logger.WarnContext(ctx, "failed to delete photo from storage, record already removed",
			"property_id", propertyID.String(),
			"photo_id", photoID.String(),
			"key", key,
			"error", err.Error())
	}

	return nil
}

func photoStorageKey(propertyID, photoID uuid.UUID, photoURL string) (string, error) {
	parsed, err := url.Parse(photoURL)
	if err != nil {
		return "", fmt.Errorf("parse photo url: %w", err)
	}
	ext := path.Ext(parsed.Path)
	if ext == "" {
		return "", errors.New("could not determine extension from photo url path")
	}
	return fmt.Sprintf("%s/%s/%s%s", photoKeyPrefix, propertyID.String(), photoID.String(), ext), nil
}

// withPhotos loads and attaches photos to the given properties.
func (s *PropertyService) withPhotos(ctx context.Context, properties ...domain.Property) ([]domain.Property, error) {
	if len(properties) == 0 {
		return properties, nil
	}

	ids := make([]uuid.UUID, len(properties))
	for i, p := range properties {
		ids[i] = p.ID
	}

	photosByProperty, err := s.photoRepo.GetByPropertyIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load photos: %w", err)
	}

	for i := range properties {
		properties[i].Photos = photosByProperty[properties[i].ID]
	}
	return properties, nil
}

func isUpdatableStatusTransition(from, to domain.PropertyStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case domain.PropertyStatusActive:
		return to == domain.PropertyStatusMaintenance
	case domain.PropertyStatusMaintenance:
		return to == domain.PropertyStatusActive
	case domain.PropertyStatusArchived:
		// Archived properties are terminal and cannot transition anywhere.
		return false
	}
	return false
}

// updatedPropertyFields lists the names of the fields a command changes. Only
// field names are audited, never their values.
func updatedPropertyFields(cmd UpdatePropertyCommand) []string {
	fields := make([]string, 0, 5)
	if cmd.Name != nil {
		fields = append(fields, "name")
	}
	if cmd.Type != nil {
		fields = append(fields, "type")
	}
	if cmd.Address != nil {
		fields = append(fields, "address")
	}
	if cmd.Description != nil {
		fields = append(fields, "description")
	}
	if cmd.Status != nil {
		fields = append(fields, "status")
	}
	return fields
}
