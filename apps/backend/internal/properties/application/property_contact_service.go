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
func (s *PropertyContactService) CreatePropertyContact(ctx context.Context, ownerID, propertyID uuid.UUID, cmd CreatePropertyContactCommand) (domain.PropertyContact, error) {
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
	property, err := txPropertyRepo.GetByIDAndOwnerForUpdate(ctx, propertyID, ownerID)
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
		OwnerID:    ownerID,
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
		ActorID:    &ownerID,
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
func (s *PropertyContactService) ListPropertyContacts(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.PropertyContact, error) {
	if _, err := s.propertyRepo.GetByIDAndOwner(ctx, propertyID, ownerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get property: %w", err)
	}

	contacts, err := s.repo.ListByProperty(ctx, propertyID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list property contacts: %w", err)
	}
	return contacts, nil
}
