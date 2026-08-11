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
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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
	// policy is injected after construction (see SetPolicy) because the
	// membership-aware policy is built after the properties module in the
	// composition root. When nil, the historical owner-only behaviour is kept.
	policy sharedpolicy.Policy
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

// SetPolicy injects the authorization policy (Property Sharing follow-up). The
// membership-aware policy is built after the properties module in the
// composition root, so it is wired via this setter. When not set, only the
// owner can read and write property contacts.
func (s *PropertyContactService) SetPolicy(policy sharedpolicy.Policy) {
	s.policy = policy
}

// CreatePropertyContact creates a contact for the given property. The property
// must exist and not be archived. With the policy wired, a member with the edit
// capability creates the contact in the account of the property's data owner
// (Property Sharing follow-up); without it, only the owner can create contacts.
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

	role := sharedpolicy.RoleOwner
	if s.policy != nil {
		r, err := s.policy.RoleForProperty(ctx, actor, propertyID)
		if err != nil {
			return domain.PropertyContact{}, fmt.Errorf("resolve role: %w", err)
		}
		if err := propertyContactWriteGate(r); err != nil {
			return domain.PropertyContact{}, err
		}
		role = r
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.PropertyContact{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPropertyRepo := s.propertyRepo.WithTx(tx)
	property, err := s.propertyForUpdate(ctx, txPropertyRepo, actor, propertyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property: %w", err)
	}
	scope := property.OwnerID
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
		OwnerID:    scope,
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
		ActorRole:  actorRoleFromPolicyRole(role),
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
// property returns ErrNotFound before any contact is read. With the policy
// wired, any member with the view capability reads the contacts of the
// property's data owner (Property Sharing follow-up).
func (s *PropertyContactService) ListPropertyContacts(ctx context.Context, actor, propertyID uuid.UUID) ([]domain.PropertyContact, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return nil, err
	}

	contacts, err := s.repo.ListByProperty(ctx, propertyID, scope)
	if err != nil {
		return nil, fmt.Errorf("list property contacts: %w", err)
	}
	return contacts, nil
}

// GetPropertyContact returns a single contact. Reading a contact of an
// archived property is allowed; a missing or foreign contact returns
// ErrNotFound. With the policy wired, any member with the view capability
// reads the contacts of the property's data owner (Property Sharing follow-up).
func (s *PropertyContactService) GetPropertyContact(ctx context.Context, actor, propertyID, contactID uuid.UUID) (domain.PropertyContact, error) {
	scope, err := s.readScope(ctx, actor, propertyID)
	if err != nil {
		return domain.PropertyContact{}, err
	}

	contact, err := s.repo.GetByIDAndOwner(ctx, contactID, scope)
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

// readScope applies the shared-access read gate for property contacts and
// returns the data owner (scope) for repository calls. Any role without the
// view capability maps to ErrNotFound so the existence of a contact is never
// revealed. Without the policy wired only the owner can read contacts (the
// historical behaviour).
func (s *PropertyContactService) readScope(ctx context.Context, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	if s.policy == nil {
		if _, err := s.propertyRepo.GetByIDAndOwner(ctx, propertyID, actor); err != nil {
			if errors.Is(err, ErrNotFound) {
				return uuid.Nil, ErrNotFound
			}
			return uuid.Nil, fmt.Errorf("get property: %w", err)
		}
		return actor, nil
	}

	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanView(role) {
		return uuid.Nil, ErrNotFound
	}
	property, err := s.propertyRepo.GetByID(ctx, propertyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("get property: %w", err)
	}
	return property.OwnerID, nil
}

// UpdatePropertyContact applies a diff-patch to a property contact. The
// property must exist and not be archived. Only provided fields are changed.
// With the policy wired, a member with the edit capability updates the contacts
// of the property's data owner (Property Sharing follow-up); without it, only
// the owner can update contacts.
func (s *PropertyContactService) UpdatePropertyContact(ctx context.Context, actor, propertyID, contactID uuid.UUID, cmd UpdatePropertyContactCommand) (domain.PropertyContact, error) {
	role := sharedpolicy.RoleOwner
	if s.policy != nil {
		r, err := s.policy.RoleForProperty(ctx, actor, propertyID)
		if err != nil {
			return domain.PropertyContact{}, fmt.Errorf("resolve role: %w", err)
		}
		if err := propertyContactWriteGate(r); err != nil {
			return domain.PropertyContact{}, err
		}
		role = r
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.PropertyContact{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPropertyRepo := s.propertyRepo.WithTx(tx)
	property, err := s.propertyForUpdate(ctx, txPropertyRepo, actor, propertyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("get property: %w", err)
	}
	scope := property.OwnerID
	if property.Status == domain.PropertyStatusArchived {
		return domain.PropertyContact{}, ErrArchivedProperty
	}

	txContactRepo := s.repo.WithTx(tx)
	contact, err := txContactRepo.GetByIDAndOwner(ctx, contactID, scope)
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

	updated, err := txContactRepo.Update(ctx, scope, contact)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.PropertyContact{}, ErrNotFound
		}
		return domain.PropertyContact{}, fmt.Errorf("update property contact: %w", err)
	}

	// PII (name, phone) is never written to the audit context.
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
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

// DeletePropertyContact removes a property contact. The property must exist
// and not be archived. With the policy wired, a member with the edit capability
// deletes the contacts of the property's data owner (Property Sharing
// follow-up); without it, only the owner can delete contacts.
func (s *PropertyContactService) DeletePropertyContact(ctx context.Context, actor, propertyID, contactID uuid.UUID) error {
	role := sharedpolicy.RoleOwner
	if s.policy != nil {
		r, err := s.policy.RoleForProperty(ctx, actor, propertyID)
		if err != nil {
			return fmt.Errorf("resolve role: %w", err)
		}
		if err := propertyContactWriteGate(r); err != nil {
			return err
		}
		role = r
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPropertyRepo := s.propertyRepo.WithTx(tx)
	property, err := s.propertyForUpdate(ctx, txPropertyRepo, actor, propertyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}
	scope := property.OwnerID
	if property.Status == domain.PropertyStatusArchived {
		return ErrArchivedProperty
	}

	txContactRepo := s.repo.WithTx(tx)
	contact, err := txContactRepo.GetByIDAndOwner(ctx, contactID, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property contact: %w", err)
	}
	if contact.PropertyID != propertyID {
		return ErrNotFound
	}

	if err := txContactRepo.Delete(ctx, contactID, scope); err != nil {
		return fmt.Errorf("delete property contact: %w", err)
	}

	// PII (name, phone) is never written to the audit context.
	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
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

// propertyForUpdate fetches the property inside a write transaction. With the
// policy wired the fetch is unscoped (the role gate has already authorized the
// actor); without it the historical owner-scoped fetch is kept.
func (s *PropertyContactService) propertyForUpdate(ctx context.Context, repo PropertyRepository, actor, propertyID uuid.UUID) (domain.Property, error) {
	if s.policy != nil {
		return repo.GetByIDForUpdate(ctx, propertyID)
	}
	return repo.GetByIDAndOwnerForUpdate(ctx, propertyID, actor)
}

// propertyContactWriteGate maps a resolved role to the write-gate outcome for
// property contacts (Property Sharing follow-up): none/suspended map to
// ErrNotFound (object privacy), a view-only role to ErrForbidden.
func propertyContactWriteGate(role sharedpolicy.Role) error {
	if role == sharedpolicy.RoleNone || role == sharedpolicy.RoleSuspended {
		return ErrNotFound
	}
	if !sharedpolicy.CanEdit(role) {
		return ErrForbidden
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
