package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// ProfileService provides read and update operations for the user's own profile.
type ProfileService struct {
	users               UserRepository
	audit               auditapp.Recorder
	db                  transaction.Beginner
	reminderRescheduler sharedtz.ReminderRescheduler
}

// NewProfileService creates a ProfileService. A nil reminderRescheduler is
// replaced with a no-op implementation.
func NewProfileService(users UserRepository, audit auditapp.Recorder, db transaction.Beginner, reminderRescheduler sharedtz.ReminderRescheduler) *ProfileService {
	if audit == nil {
		audit = auditapp.Noop{}
	}
	if reminderRescheduler == nil {
		reminderRescheduler = noopReminderRescheduler{}
	}
	return &ProfileService{users: users, audit: audit, db: db, reminderRescheduler: reminderRescheduler}
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
// new address is marked as unverified until confirmed.
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, cmd UpdateProfileCommand) (domain.User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	users, err := s.users.WithTx(tx)
	if err != nil {
		return domain.User{}, fmt.Errorf("bind user repository to tx: %w", err)
	}
	user, err := users.GetByIDForUpdate(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}

	// Capture old timezone before mutation so we can reschedule reminders if it changed.
	oldTimezone := user.Timezone.String()

	var oldEmail string
	if user.Email != nil {
		oldEmail = user.Email.String()
	}

	if err := user.UpdatePersonalData(cmd.Name, cmd.Surname, cmd.Patronymic, cmd.Email, cmd.Timezone); err != nil {
		return domain.User{}, fmt.Errorf("update personal data: %w", err)
	}

	emailChanged := cmd.Email != nil && user.Email != nil && user.Email.String() != oldEmail
	if emailChanged {
		user.EmailVerifiedAt = nil
	}

	updated, err := users.Update(ctx, user)
	if err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	user = updated

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  AuditActorRole(user.Role),
		Action:     auditdomain.ActionProfileUpdated,
		EntityType: auditdomain.EntityUser,
		EntityID:   &userID,
		Context:    map[string]any{"fields": updatedProfileFields(cmd)},
	}); err != nil {
		return domain.User{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	// Reschedule pending reminders if the timezone changed (wall-clock semantics).
	// The profile update is already committed; this runs synchronously so the
	// user sees the reschedule complete within the request.
	newTimezone := user.Timezone.String()
	if cmd.Timezone != nil && newTimezone != oldTimezone {
		if err := s.reminderRescheduler.RescheduleForTimezoneChange(ctx, userID, oldTimezone, newTimezone); err != nil {
			return domain.User{}, fmt.Errorf("reschedule reminders for timezone change: %w", err)
		}
	}

	return user, nil
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
