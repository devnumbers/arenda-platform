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
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

// CreateTenantContactCommand carries the data needed to create a tenant contact.
type CreateTenantContactCommand struct {
	Name       string
	Surname    *string
	Patronymic *string
	Phone      *string
	Email      *string
	Comment    *string
}

// UpdateTenantContactCommand carries the optional updates for a tenant contact.
type UpdateTenantContactCommand struct {
	Name       *string
	Surname    *string
	Patronymic *string
	Phone      *string
	Email      *string
	Comment    *string
}

// TenantContactService orchestrates tenant contact use cases within the leases
// bounded context.
type TenantContactService struct {
	repo   TenantContactRepository
	audit  auditapp.Recorder
	logger *slog.Logger
}

// NewTenantContactService creates a new tenant contact service.
func NewTenantContactService(repo TenantContactRepository, audit auditapp.Recorder, logger *slog.Logger) *TenantContactService {
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &TenantContactService{repo: repo, audit: audit, logger: logger}
}

// CreateTenantContact creates a tenant contact for the given owner.
func (s *TenantContactService) CreateTenantContact(ctx context.Context, actor uuid.UUID, cmd CreateTenantContactCommand) (domain.TenantContact, error) {
	if strings.TrimSpace(cmd.Name) == "" {
		return domain.TenantContact{}, ErrInvalidInput
	}

	if cmd.Phone != nil {
		normalized, err := domain.NormalizePhone(*cmd.Phone)
		if err != nil {
			return domain.TenantContact{}, ErrInvalidInput
		}
		cmd.Phone = &normalized
	}

	if cmd.Email != nil {
		if err := domain.ValidateEmail(*cmd.Email); err != nil {
			return domain.TenantContact{}, ErrInvalidInput
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("generate tenant contact id: %w", err)
	}

	contact := domain.TenantContact{
		ID:         id,
		OwnerID:    actor,
		Name:       cmd.Name,
		Surname:    cmd.Surname,
		Patronymic: cmd.Patronymic,
		Phone:      cmd.Phone,
		Email:      cmd.Email,
		Comment:    cmd.Comment,
	}

	created, err := s.repo.Create(ctx, actor, contact)
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("create tenant contact: %w", err)
	}

	// Post-commit, fail-loud: the create is already committed, so the audit
	// error is returned deliberately to surface audit gaps. A retry may
	// duplicate the contact — acceptable for this entity.
	// Tenant PII (name, phone, email) is never written to the audit context.
	if err := s.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionTenantContactCreated,
		EntityType: auditdomain.EntityTenantContact,
		EntityID:   &created.ID,
	}); err != nil {
		return domain.TenantContact{}, fmt.Errorf("record audit: %w", err)
	}
	return created, nil
}

// GetTenantContact returns a tenant contact owned by the given owner.
func (s *TenantContactService) GetTenantContact(ctx context.Context, actor, id uuid.UUID) (domain.TenantContact, error) {
	contact, err := s.repo.GetByIDAndOwner(ctx, id, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.TenantContact{}, ErrNotFound
		}
		return domain.TenantContact{}, fmt.Errorf("get tenant contact: %w", err)
	}
	return contact, nil
}

// UpdateTenantContact updates a tenant contact owned by the given owner.
func (s *TenantContactService) UpdateTenantContact(ctx context.Context, actor, id uuid.UUID, cmd UpdateTenantContactCommand) (domain.TenantContact, error) {
	contact, err := s.repo.GetByIDAndOwner(ctx, id, actor)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.TenantContact{}, ErrNotFound
		}
		return domain.TenantContact{}, fmt.Errorf("get tenant contact: %w", err)
	}

	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return domain.TenantContact{}, ErrInvalidInput
		}
		contact.Name = name
	}
	if cmd.Surname != nil {
		surname := strings.TrimSpace(*cmd.Surname)
		if surname == "" {
			contact.Surname = nil
		} else {
			contact.Surname = &surname
		}
	}
	if cmd.Patronymic != nil {
		patronymic := strings.TrimSpace(*cmd.Patronymic)
		if patronymic == "" {
			contact.Patronymic = nil
		} else {
			contact.Patronymic = &patronymic
		}
	}
	if cmd.Phone != nil {
		phone := strings.TrimSpace(*cmd.Phone)
		if phone == "" {
			contact.Phone = nil
		} else {
			normalized, err := domain.NormalizePhone(phone)
			if err != nil {
				return domain.TenantContact{}, ErrInvalidInput
			}
			contact.Phone = &normalized
		}
	}
	if cmd.Email != nil {
		email := strings.TrimSpace(*cmd.Email)
		if email == "" {
			contact.Email = nil
		} else {
			if err := domain.ValidateEmail(email); err != nil {
				return domain.TenantContact{}, ErrInvalidInput
			}
			contact.Email = &email
		}
	}
	if cmd.Comment != nil {
		comment := strings.TrimSpace(*cmd.Comment)
		if comment == "" {
			contact.Comment = nil
		} else {
			contact.Comment = &comment
		}
	}

	updated, err := s.repo.Update(ctx, actor, contact)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.TenantContact{}, ErrNotFound
		}
		return domain.TenantContact{}, fmt.Errorf("update tenant contact: %w", err)
	}

	// Post-commit, fail-loud: the update is already committed, so the audit
	// error is returned deliberately to surface audit gaps. Retries are safe.
	if err := s.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionTenantContactUpdated,
		EntityType: auditdomain.EntityTenantContact,
		EntityID:   &id,
		Context:    map[string]any{"fields": updatedTenantContactFields(cmd)},
	}); err != nil {
		return domain.TenantContact{}, fmt.Errorf("record audit: %w", err)
	}
	return updated, nil
}

// ListTenantContacts returns all tenant contacts for the given owner.
func (s *TenantContactService) ListTenantContacts(ctx context.Context, actor uuid.UUID) ([]domain.TenantContact, error) {
	contacts, err := s.repo.ListByOwner(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list tenant contacts: %w", err)
	}
	return contacts, nil
}

// ListTenantContactsWithLeaseStatus returns all tenant contacts for the owner,
// each enriched with the active lease (if any) and the most recent terminal lease.
func (s *TenantContactService) ListTenantContactsWithLeaseStatus(ctx context.Context, actor uuid.UUID) ([]domain.TenantContactWithLeases, error) {
	contacts, err := s.repo.ListWithLeaseStatus(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list tenant contacts with lease status: %w", err)
	}
	return contacts, nil
}

// ListTenantContactsByIDs returns the tenant contacts for the given owner and IDs.
func (s *TenantContactService) ListTenantContactsByIDs(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]domain.TenantContact, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]domain.TenantContact{}, nil
	}
	contacts, err := s.repo.ListByIDs(ctx, actor, ids)
	if err != nil {
		return nil, fmt.Errorf("list tenant contacts by ids: %w", err)
	}
	result := make(map[uuid.UUID]domain.TenantContact, len(contacts))
	for _, contact := range contacts {
		result[contact.ID] = contact
	}
	return result, nil
}

// updatedTenantContactFields lists the names of the fields a command changes.
// Only field names are audited, never their values: tenant contact data is PII.
func updatedTenantContactFields(cmd UpdateTenantContactCommand) []string {
	fields := make([]string, 0, 6)
	if cmd.Name != nil {
		fields = append(fields, "name")
	}
	if cmd.Surname != nil {
		fields = append(fields, "surname")
	}
	if cmd.Patronymic != nil {
		fields = append(fields, "patronymic")
	}
	if cmd.Phone != nil {
		fields = append(fields, "phone")
	}
	if cmd.Email != nil {
		fields = append(fields, "email")
	}
	if cmd.Comment != nil {
		fields = append(fields, "comment")
	}
	return fields
}
