package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// ProfileService provides read and update operations for the user's own profile.
//
// It embeds the identity txStoreFactory so UpdateProfile runs through runInTx
// (ADR 0033, migration step 3): the user repository and audit recorder are
// bound to the transaction by the factory, and the use case describes only the
// business logic. Me is a plain read and stays outside the transaction.
type ProfileService struct {
	txStoreFactory
	reminderRescheduler sharedtz.ReminderRescheduler
}

// ProfileServiceConfig carries the non-transactional dependencies for
// ProfileService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewProfileService.
type ProfileServiceConfig struct {
	ReminderRescheduler sharedtz.ReminderRescheduler
}

// NewProfileService creates a ProfileService. It embeds the shared identity
// txStoreFactory so UpdateProfile runs through runInTx; the repositories and
// audit recorder are shared by every identity service (ADR 0033 γ-factory). A
// nil ReminderRescheduler is replaced with a no-op implementation.
func NewProfileService(
	factory txStoreFactory,
	cfg ProfileServiceConfig,
) *ProfileService {
	reminderRescheduler := cfg.ReminderRescheduler
	if reminderRescheduler == nil {
		reminderRescheduler = noopReminderRescheduler{}
	}
	return &ProfileService{
		txStoreFactory:      factory,
		reminderRescheduler: reminderRescheduler,
	}
}

type noopReminderRescheduler struct{}

func (noopReminderRescheduler) RescheduleForTimezoneChange(context.Context, uuid.UUID, string, string) error {
	return nil
}

// Me returns the user profile.
func (s *ProfileService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

// UpdateProfile updates the user's personal data. When the email changes, the
// new address is marked as unverified until confirmed. The mutation and the
// audit entry run in a single transaction through runInTx (ADR 0033, ADR 0020);
// the timezone reschedule stays post-commit because it writes to the
// notifications context, not the identity transaction.
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, cmd UpdateProfileCommand) (domain.User, error) {
	var updated domain.User
	oldTimezone := ""
	var oldEmail string

	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		// Capture old timezone/email before mutation so we can reschedule
		// reminders and detect an email change after commit.
		oldTimezone = user.Timezone.String()
		if user.Email != nil {
			oldEmail = user.Email.String()
		}

		if err := user.UpdatePersonalData(cmd.Name, cmd.Surname, cmd.Patronymic, cmd.Email, cmd.Timezone); err != nil {
			return fmt.Errorf("update personal data: %w", err)
		}

		if cmd.Email != nil && user.Email != nil && user.Email.String() != oldEmail {
			user.EmailVerifiedAt = nil
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
			Context:    map[string]any{"fields": updatedProfileFields(cmd)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}

	// Reschedule pending reminders if the timezone changed (wall-clock
	// semantics). The profile update is already committed; this runs
	// synchronously so the user sees the reschedule complete within the
	// request. It belongs to the notifications context, so it stays outside
	// the identity transaction.
	newTimezone := updated.Timezone.String()
	if cmd.Timezone != nil && newTimezone != oldTimezone {
		if err := s.reminderRescheduler.RescheduleForTimezoneChange(ctx, userID, oldTimezone, newTimezone); err != nil {
			return domain.User{}, fmt.Errorf("reschedule reminders for timezone change: %w", err)
		}
	}

	return updated, nil
}

// updatedProfileFields lists the names of the fields a command changes. Only
// field names are audited, never their values.
func updatedProfileFields(cmd UpdateProfileCommand) []string {
	fields := make([]string, 0, 5)
	if cmd.Name != nil {
		fields = append(fields, "name")
	}
	if cmd.Surname != nil {
		fields = append(fields, "surname")
	}
	if cmd.Patronymic != nil {
		fields = append(fields, "patronymic")
	}
	if cmd.Email != nil {
		fields = append(fields, "email")
	}
	if cmd.Timezone != nil {
		fields = append(fields, "timezone")
	}
	return fields
}
