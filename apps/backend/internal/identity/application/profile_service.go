package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// ProfileService provides read and update operations for the user's own profile.
//
// It embeds the identity txStoreFactory so UpdateProfile runs through runInTx
// (ADR 0033, migration step 3): the user repository and audit recorder are
// bound to the transaction by the factory, and the use case describes only the
// business logic. Me is a plain read and stays outside the transaction.
type ProfileService struct {
	txStoreFactory
}

// NewProfileService creates a ProfileService. It embeds the shared identity
// txStoreFactory so UpdateProfile runs through runInTx; the repositories and
// audit recorder are shared by every identity service (ADR 0033 γ-factory).
func NewProfileService(factory txStoreFactory) *ProfileService {
	return &ProfileService{txStoreFactory: factory}
}

// Me returns the user profile.
func (s *ProfileService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

// UpdateProfile updates the user's personal data. The email-change →
// unverified reset is owned by domain.User.UpdatePersonalData; the mutation
// and the audit entry run in a single transaction through runInTx (ADR 0033,
// ADR 0020).
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, cmd UpdateProfileCommand) (domain.User, error) {
	var updated domain.User

	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if err := user.UpdatePersonalData(cmd.Name, cmd.Surname, cmd.Patronymic, cmd.Email, cmd.Timezone); err != nil {
			return fmt.Errorf("update personal data: %w", err)
		}

		updated, err = stores.users.Update(ctx, user)
		if err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleFromRole(updated.Role),
			Action:     auditdomain.ActionProfileUpdated,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
			Context:    map[string]any{"fields": cmd.ChangedFields()},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}

	return updated, nil
}
