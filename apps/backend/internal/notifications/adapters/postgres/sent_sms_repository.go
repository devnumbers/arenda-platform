package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
)

// SentSMSReminderRepository persists records of sent SMS reminders.
type SentSMSReminderRepository struct {
	db postgres.DBTX
}

// NewSentSMSReminderRepository creates a new sent SMS reminder repository.
func NewSentSMSReminderRepository(db postgres.DBTX) *SentSMSReminderRepository {
	return &SentSMSReminderRepository{db: db}
}

// Save stores a record of a successfully sent SMS reminder.
func (r *SentSMSReminderRepository) Save(ctx context.Context, reminderID *uuid.UUID, ownerID uuid.UUID, phone, message, providerResponse string, sentAt time.Time) error {
	var reminderIDPg pgtype.UUID
	if reminderID != nil {
		reminderIDPg = pgconv.UUIDToPgtype(*reminderID)
	}
	_, err := postgres.New(r.db).CreateSentSMSReminder(ctx, postgres.CreateSentSMSReminderParams{
		ReminderID:       reminderIDPg,
		OwnerID:          pgconv.UUIDToPgtype(ownerID),
		Phone:            phone,
		Message:          message,
		ProviderResponse: pgtype.Text{String: providerResponse, Valid: providerResponse != ""},
		SentAt:           pgtype.Timestamptz{Time: sentAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create sent sms reminder: %w", err)
	}
	return nil
}

var _ application.SentSMSReminderRepository = (*SentSMSReminderRepository)(nil)
