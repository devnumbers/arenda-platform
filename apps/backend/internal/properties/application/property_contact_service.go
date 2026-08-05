package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// CreatePropertyContactCommand carries the data needed to create a property contact.
type CreatePropertyContactCommand struct {
	Name  string
	Phone string
}

// UpdatePropertyContactCommand carries the optional updates for a property contact.
// A nil field means "do not change".
type UpdatePropertyContactCommand struct {
	Name  *string
	Phone *string
}

// PropertyContactService orchestrates property contact use cases.
type PropertyContactService struct {
	repo         PropertyContactRepository
	propertyRepo PropertyRepository
	db           txBeginner
	audit        auditapp.Recorder
	logger       *slog.Logger
}

// NewPropertyContactService creates a new property contact service.
func NewPropertyContactService(repo PropertyContactRepository, propertyRepo PropertyRepository, db txBeginner, audit auditapp.Recorder, logger *slog.Logger) *PropertyContactService {
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &PropertyContactService{repo: repo, propertyRepo: propertyRepo, db: db, audit: audit, logger: logger}
}

// CreatePropertyContact creates a contact for the given property. The property
// must exist, belong to the owner, and not be archived.
func (s *PropertyContactService) CreatePropertyContact(ctx context.Context, actor, propertyID uuid.UUID, cmd CreatePropertyContactCommand) (domain.PropertyContact, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domain.PropertyContact{}, ErrInvalidInput
	}
	if len(name) > 255 {
		return domain.PropertyContact{}, ErrInvalidInput
	}

	normalized, err := domain.NormalizePhone(cmd.Phone)
	if err != nil {
		return domain.PropertyContact{}, ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.PropertyContact{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPropertyRepo := s.propertyRepo.WithTx(tx)
	property, err := txPropertyRepo.GetByIDAndOwnerForUpdate(ctx, propertyID, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property: %w", err)
	}
	if property.Status == domain.PropertyStatusArchived {
		return domain.PropertyContact{}, ErrArchivedProperty
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.PropertyContact{}, fmt.Errorf("generate property contact id: %w", err)
	}

	contact := domain.PropertyContact{
		ID:         id,
		PropertyID: propertyID,
		OwnerID:    actor,
		Name:       name,
		Phone:      normalized,
	}

	txContactRepo := s.repo.WithTx(tx)
	created, err := txContactRepo.Create(ctx, contact)
	if err != nil {
		return domain.PropertyContact{}, fmt.Errorf("create property contact: %w", err)
	}

	// PII (name, phone) is never written to the audit context.
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyContactCreated,
		EntityType: auditdomain.EntityPropertyContact,
		EntityID:   &created.ID,
	}); err != nil {
		return domain.PropertyContact{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PropertyContact{}, fmt.Errorf("commit tx: %w", err)
	}
	return created, nil
}

// ListPropertyContacts returns all contacts of a property ordered by created_at
// ASC. Reading contacts of an archived property is allowed; a missing or foreign
// property returns ErrNotFound before any contact is read.
func (s *PropertyContactService) ListPropertyContacts(ctx context.Context, actor, propertyID uuid.UUID) ([]domain.PropertyContact, error) {
	if _, err := s.propertyRepo.GetByIDAndOwner(ctx, propertyID, actor); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get property: %w", err)
	}

	contacts, err := s.repo.ListByProperty(ctx, propertyID, actor)
	if err != nil {
		return nil, fmt.Errorf("list property contacts: %w", err)
	}
	return contacts, nil
}

// GetPropertyContact returns a single contact owned by the owner. Reading a
// contact of an archived property is allowed; a missing or foreign contact
// returns ErrNotFound.
func (s *PropertyContactService) GetPropertyContact(ctx context.Context, actor, propertyID, contactID uuid.UUID) (domain.PropertyContact, error) {
	if _, err := s.propertyRepo.GetByIDAndOwner(ctx, propertyID, actor); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property: %w", err)
	}

	contact, err := s.repo.GetByIDAndOwner(ctx, contactID, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property contact: %w", err)
	}
	if contact.PropertyID != propertyID {
		return domain.PropertyContact{}, ErrNotFound
	}
	return contact, nil
}

// UpdatePropertyContact applies a diff-patch to a property contact. The property
// must exist, belong to the owner, and not be archived. Only provided fields are
// changed.
func (s *PropertyContactService) UpdatePropertyContact(ctx context.Context, actor, propertyID, contactID uuid.UUID, cmd UpdatePropertyContactCommand) (domain.PropertyContact, error) {
	contact, err := s.repo.GetByIDAndOwner(ctx, contactID, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property contact: %w", err)
	}
	if contact.PropertyID != propertyID {
		return domain.PropertyContact{}, ErrNotFound
	}

	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" || len(name) > 255 {
			return domain.PropertyContact{}, ErrInvalidInput
		}
		contact.Name = name
	}
	if cmd.Phone != nil {
		normalized, err := domain.NormalizePhone(*cmd.Phone)
		if err != nil {
			return domain.PropertyContact{}, ErrInvalidInput
		}
		contact.Phone = normalized
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.PropertyContact{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPropertyRepo := s.propertyRepo.WithTx(tx)
	property, err := txPropertyRepo.GetByIDAndOwnerForUpdate(ctx, propertyID, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property: %w", err)
	}
	if property.Status == domain.PropertyStatusArchived {
		return domain.PropertyContact{}, ErrArchivedProperty
	}

	txContactRepo := s.repo.WithTx(tx)
	updated, err := txContactRepo.Update(ctx, actor, contact)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("update property contact: %w", err)
	}

	// PII (name, phone) is never written to the audit context.
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyContactUpdated,
		EntityType: auditdomain.EntityPropertyContact,
		EntityID:   &updated.ID,
		Context:    map[string]any{"fields": updatedPropertyContactFields(cmd)},
	}); err != nil {
		return domain.PropertyContact{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PropertyContact{}, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// DeletePropertyContact removes a property contact. The property must exist,
// belong to the owner, and not be archived.
func (s *PropertyContactService) DeletePropertyContact(ctx context.Context, actor, propertyID, contactID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPropertyRepo := s.propertyRepo.WithTx(tx)
	property, err := txPropertyRepo.GetByIDAndOwnerForUpdate(ctx, propertyID, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}
	if property.Status == domain.PropertyStatusArchived {
		return ErrArchivedProperty
	}

	txContactRepo := s.repo.WithTx(tx)
	contact, err := txContactRepo.GetByIDAndOwner(ctx, contactID, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property contact: %w", err)
	}
	if contact.PropertyID != propertyID {
		return ErrNotFound
	}

	if err := txContactRepo.Delete(ctx, contactID, actor); err != nil {
		return fmt.Errorf("delete property contact: %w", err)
	}

	// PII (name, phone) is never written to the audit context.
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyContactDeleted,
		EntityType: auditdomain.EntityPropertyContact,
		EntityID:   &contactID,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// updatedPropertyContactFields lists the names of the fields a command changes.
// Only field names are audited, never their values: property contact data is PII.
func updatedPropertyContactFields(cmd UpdatePropertyContactCommand) []string {
	fields := make([]string, 0, 2)
	if cmd.Name != nil {
		fields = append(fields, "name")
	}
	if cmd.Phone != nil {
		fields = append(fields, "phone")
	}
	return fields
}
