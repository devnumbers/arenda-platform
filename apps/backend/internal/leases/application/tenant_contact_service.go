package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
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

// TenantContactService orchestrates tenant contact use cases within the leases
// bounded context.
type TenantContactService struct {
	repo   TenantContactRepository
	logger *slog.Logger
}

// NewTenantContactService creates a new tenant contact service.
func NewTenantContactService(repo TenantContactRepository, logger *slog.Logger) *TenantContactService {
	if logger == nil {
		logger = slog.Default()
	}
	return &TenantContactService{repo: repo, logger: logger}
}

// CreateTenantContact creates a tenant contact for the given owner.
func (s *TenantContactService) CreateTenantContact(ctx context.Context, ownerID uuid.UUID, cmd CreateTenantContactCommand) (domain.TenantContact, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("generate tenant contact id: %w", err)
	}

	contact := domain.TenantContact{
		ID:         id,
		OwnerID:    ownerID,
		Name:       cmd.Name,
		Surname:    cmd.Surname,
		Patronymic: cmd.Patronymic,
		Phone:      cmd.Phone,
		Email:      cmd.Email,
		Comment:    cmd.Comment,
	}

	created, err := s.repo.Create(ctx, ownerID, contact)
	if err != nil {
		return domain.TenantContact{}, fmt.Errorf("create tenant contact: %w", err)
	}
	return created, nil
}

// GetTenantContact returns a tenant contact owned by the given owner.
func (s *TenantContactService) GetTenantContact(ctx context.Context, ownerID, id uuid.UUID) (domain.TenantContact, error) {
	contact, err := s.repo.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.TenantContact{}, ErrNotFound
		}
		return domain.TenantContact{}, fmt.Errorf("get tenant contact: %w", err)
	}
	return contact, nil
}

// ListTenantContacts returns all tenant contacts for the given owner.
func (s *TenantContactService) ListTenantContacts(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContact, error) {
	contacts, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list tenant contacts: %w", err)
	}
	return contacts, nil
}
