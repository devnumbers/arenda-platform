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
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// CreateTenantContactCommand carries the data needed to create a tenant contact.
type CreateTenantContactCommand struct {
	Name       string
	Surname    *string
	Patronymic *string
	Phone      *string
	Email      *string
	Comment    *string
	// PropertyID carries the optional property context: when set, the contact
	// is created in the account of the property's data owner after the
	// shared-access write gate (issue #157 follow-up).
	PropertyID *uuid.UUID
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
	// Policy is injected after construction (see SetPolicy) because the
	// membership-aware policy is built after the tenant contact service in the
	// composition root. When nil, only the actor's own data is listed (pre-T2a).
	policy sharedpolicy.Policy
	// Scopes is optionally injected (see SetAccessibleScopes); when nil only the
	// actor's own contacts are listed.
	scopes AccessibleScopes
	// Properties resolves the data owner of a property for owner-wide writes
	// issued in a property context (Property Sharing follow-up). Injected via
	// SetProperties; when nil a property context cannot be resolved.
	properties PropertyRepository
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

// SetPolicy injects the authorization policy. The membership-aware policy is
// built after the tenant contact service in the composition root, so it is
// wired via this setter. When not set, only the actor's own data is listed
// (issue #157).
func (s *TenantContactService) SetPolicy(policy sharedpolicy.Policy) {
	s.policy = policy
}

// SetAccessibleScopes injects the access-context adapter that resolves the
// owners whose owner-wide data the actor may read. Optional: when nil, only the
// actor's own contacts are listed (issue #157).
func (s *TenantContactService) SetAccessibleScopes(scopes AccessibleScopes) {
	s.scopes = scopes
}

// SetProperties injects the property repository used to resolve the data
// owner of a property context on writes (Property Sharing follow-up).
func (s *TenantContactService) SetProperties(properties PropertyRepository) {
	s.properties = properties
}

// CreateTenantContact creates a tenant contact. Without a property context
// the contact is created in the actor's own account; with cmd.PropertyID set
// it is created in the account of the property's data owner after the
// shared-access write gate (issue #157 follow-up, card #145 decision).
func (s *TenantContactService) CreateTenantContact(
	ctx context.Context,
	actor uuid.UUID,
	cmd CreateTenantContactCommand,
) (domain.TenantContact, error) {
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

	role, scope, err := resolveOwnerWideWriteScope(ctx, s.policy, s.properties, actor, cmd.PropertyID)
	if err != nil {
		return domain.TenantContact{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("generate tenant contact id: %w", err)
	}

	contact := domain.TenantContact{
		ID:         id,
		OwnerID:    scope,
		Name:       cmd.Name,
		Surname:    cmd.Surname,
		Patronymic: cmd.Patronymic,
		Phone:      cmd.Phone,
		Email:      cmd.Email,
		Comment:    cmd.Comment,
	}

	created, err := s.repo.Create(ctx, scope, contact)
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("create tenant contact: %w", err)
	}

	// Post-commit, fail-loud: the create is already committed, so the audit
	// error is returned deliberately to surface audit gaps. A retry may
	// duplicate the contact — acceptable for this entity.
	// Tenant PII (name, phone, email) is never written to the audit context.
	if err := s.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionTenantContactCreated,
		EntityType: auditdomain.EntityTenantContact,
		EntityID:   &created.ID,
	}); err != nil {
		return domain.TenantContact{}, fmt.Errorf("record audit: %w", err)
	}
	return created, nil
}

// GetTenantContact returns a tenant contact. The contact's data owner is
// resolved unscoped, then the actor's owner-wide role is gated by the policy
// port (Property Sharing follow-up): any member with the view capability reads
// the owner's contacts, anyone without access gets ErrNotFound. Without the
// policy wired the historical owner-only behaviour is kept.
func (s *TenantContactService) GetTenantContact(ctx context.Context, actor, id uuid.UUID) (domain.TenantContact, error) {
	contact, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.TenantContact{}, ErrNotFound
		}
		return domain.TenantContact{}, fmt.Errorf("get tenant contact: %w", err)
	}

	if s.policy == nil {
		if actor != contact.OwnerID {
			return domain.TenantContact{}, ErrNotFound
		}
		return contact, nil
	}
	role, err := s.policy.Role(ctx, actor, contact.OwnerID)
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanView(role) {
		return domain.TenantContact{}, ErrNotFound
	}
	return contact, nil
}

// UpdateTenantContact updates a tenant contact. The contact's data owner is
// resolved unscoped, then the actor's owner-wide role is gated by the policy
// port (Property Sharing follow-up): a full-access member edits the owner's
// contacts, a viewer gets ErrForbidden, and anyone without access gets
// ErrNotFound. Without the policy wired the historical owner-only behaviour
// is kept.
func (s *TenantContactService) UpdateTenantContact(
	ctx context.Context,
	actor, id uuid.UUID,
	cmd UpdateTenantContactCommand,
) (domain.TenantContact, error) {
	contact, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.TenantContact{}, ErrNotFound
		}
		return domain.TenantContact{}, fmt.Errorf("get tenant contact: %w", err)
	}

	role, err := s.gateTenantContactWrite(ctx, actor, contact)
	if err != nil {
		return domain.TenantContact{}, err
	}

	contact, err = applyTenantContactUpdate(contact, cmd)
	if err != nil {
		return domain.TenantContact{}, err
	}

	updated, err := s.repo.Update(ctx, contact.OwnerID, contact)
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
		ActorRole:  actorRoleFromPolicyRole(role),
		Action:     auditdomain.ActionTenantContactUpdated,
		EntityType: auditdomain.EntityTenantContact,
		EntityID:   &id,
		Context:    map[string]any{auditKeyFields: updatedTenantContactFields(cmd)},
	}); err != nil {
		return domain.TenantContact{}, fmt.Errorf("record audit: %w", err)
	}
	return updated, nil
}

// gateTenantContactWrite resolves the actor's owner-wide write role for a
// tenant contact and gates it: without a policy wired the historical
// owner-only behaviour applies (only the owner, ErrNotFound for anyone else).
func (s *TenantContactService) gateTenantContactWrite(
	ctx context.Context,
	actor uuid.UUID,
	contact domain.TenantContact,
) (sharedpolicy.Role, error) {
	if s.policy == nil {
		if actor != contact.OwnerID {
			return sharedpolicy.RoleOwner, ErrNotFound
		}
		return sharedpolicy.RoleOwner, nil
	}
	role, err := s.policy.Role(ctx, actor, contact.OwnerID)
	if err != nil {
		return sharedpolicy.RoleOwner, fmt.Errorf("resolve role: %w", err)
	}
	if err := writeRoleGate(role); err != nil {
		return sharedpolicy.RoleOwner, err
	}
	return role, nil
}

// applyTenantContactUpdate applies the command's field patches to the contact:
// a present field replaces the value (validated for name, phone, and email),
// an empty string clears it.
func applyTenantContactUpdate(contact domain.TenantContact, cmd UpdateTenantContactCommand) (domain.TenantContact, error) {
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return domain.TenantContact{}, ErrInvalidInput
		}
		contact.Name = name
	}
	if cmd.Surname != nil {
		contact.Surname = optionalTrimmedString(cmd.Surname)
	}
	if cmd.Patronymic != nil {
		contact.Patronymic = optionalTrimmedString(cmd.Patronymic)
	}
	if cmd.Phone != nil {
		if err := applyPhonePatch(&contact, cmd.Phone); err != nil {
			return domain.TenantContact{}, err
		}
	}
	if cmd.Email != nil {
		if err := applyEmailPatch(&contact, cmd.Email); err != nil {
			return domain.TenantContact{}, err
		}
	}
	if cmd.Comment != nil {
		contact.Comment = optionalTrimmedString(cmd.Comment)
	}
	return contact, nil
}

// optionalTrimmedString trims an optional string patch: an empty value clears
// the field (nil), a non-empty value is returned trimmed.
func optionalTrimmedString(v *string) *string {
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// applyPhonePatch applies an optional phone patch to the contact: an empty
// value clears the phone, a non-empty value must normalize to a valid phone.
func applyPhonePatch(contact *domain.TenantContact, v *string) error {
	phone := strings.TrimSpace(*v)
	if phone == "" {
		contact.Phone = nil
		return nil
	}
	normalized, err := domain.NormalizePhone(phone)
	if err != nil {
		return ErrInvalidInput
	}
	contact.Phone = &normalized
	return nil
}

// applyEmailPatch applies an optional email patch to the contact: an empty
// value clears the email, a non-empty value must be a valid email address.
func applyEmailPatch(contact *domain.TenantContact, v *string) error {
	email := strings.TrimSpace(*v)
	if email == "" {
		contact.Email = nil
		return nil
	}
	if err := domain.ValidateEmail(email); err != nil {
		return ErrInvalidInput
	}
	contact.Email = &email
	return nil
}

// ListTenantContacts returns all tenant contacts for the owner, plus the
// contacts of owners whose properties the actor is a member of when the access
// adapter is injected (issue #157, T2a). Each accessible owner is gated by the
// policy port so the actor only sees owners for which CanView holds.
func (s *TenantContactService) ListTenantContacts(ctx context.Context, actor uuid.UUID) ([]domain.TenantContact, error) {
	var result []domain.TenantContact
	seen := map[uuid.UUID]bool{}
	if err := s.forEachAccessibleScope(ctx, actor, func(ctx context.Context, scope uuid.UUID) error {
		contacts, err := s.repo.ListByOwner(ctx, scope)
		if err != nil {
			return fmt.Errorf("list tenant contacts: %w", err)
		}
		for _, c := range contacts {
			if !seen[c.ID] {
				seen[c.ID] = true
				result = append(result, c)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// ListTenantContactsWithLeaseStatus returns all tenant contacts for the owner,
// each enriched with the active lease (if any) and the most recent terminal
// lease. When the access adapter is injected, contacts of owners whose
// properties the actor is a member of are appended (issue #157, T2a).
func (s *TenantContactService) ListTenantContactsWithLeaseStatus(
	ctx context.Context,
	actor uuid.UUID,
) ([]domain.TenantContactWithLeases, error) {
	var result []domain.TenantContactWithLeases
	seen := map[uuid.UUID]bool{}
	if err := s.forEachAccessibleScope(ctx, actor, func(ctx context.Context, scope uuid.UUID) error {
		contacts, err := s.repo.ListWithLeaseStatus(ctx, scope)
		if err != nil {
			return fmt.Errorf("list tenant contacts with lease status: %w", err)
		}
		for _, c := range contacts {
			if !seen[c.ID] {
				seen[c.ID] = true
				result = append(result, c)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// forEachAccessibleScope invokes load for the actor's own scope and, when the
// policy and scopes adapter are injected, for each accessible owner gated by
// CanView (issue #157, T2a). When either dependency is nil only the actor's own
// scope is loaded (the pre-T2a behaviour).
func (s *TenantContactService) forEachAccessibleScope(
	ctx context.Context,
	actor uuid.UUID,
	load func(ctx context.Context, scope uuid.UUID) error,
) error {
	if err := load(ctx, actor); err != nil {
		return err
	}
	if s.policy == nil || s.scopes == nil {
		return nil
	}
	owners, err := s.scopes.AccessibleOwners(ctx, actor)
	if err != nil {
		return fmt.Errorf("list accessible owners: %w", err)
	}
	for _, owner := range owners {
		role, err := s.policy.Role(ctx, actor, owner)
		if err != nil {
			return fmt.Errorf("resolve role for owner: %w", err)
		}
		if !sharedpolicy.CanView(role) {
			continue
		}
		if err := load(ctx, owner); err != nil {
			return err
		}
	}
	return nil
}

// ListTenantContactsByIDs returns the tenant contacts for the given owner and IDs.
func (s *TenantContactService) ListTenantContactsByIDs(
	ctx context.Context,
	actor uuid.UUID,
	ids []uuid.UUID,
) (map[uuid.UUID]domain.TenantContact, error) {
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
