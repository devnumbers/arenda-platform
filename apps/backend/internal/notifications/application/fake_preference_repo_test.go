package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakePreferenceRepo is an in-memory PreferenceRepository that records the
// upserted rows so preference tests can assert what ReplaceChannelPreferences
// stored.
type fakePreferenceRepo struct {
	storedPrefs   []domain.NotificationChannelPreference // ListChannelPreferences result.
	upsertedPrefs []domain.NotificationChannelPreference // Recorded by UpsertChannelPreference.
	upsertErr     error                                  // UpsertChannelPreference failure.
}

func (r *fakePreferenceRepo) ListChannelPreferences(context.Context, uuid.UUID) ([]domain.NotificationChannelPreference, error) {
	return r.storedPrefs, nil
}

func (r *fakePreferenceRepo) UpsertChannelPreference(_ context.Context, _ uuid.UUID, p domain.NotificationChannelPreference) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.upsertedPrefs = append(r.upsertedPrefs, p)
	return nil
}

func (r *fakePreferenceRepo) IsChannelAllowed(context.Context, uuid.UUID, domain.EventType, domain.NotificationChannel) (bool, error) {
	return true, nil
}

func (r *fakePreferenceRepo) WithTx(transaction.Tx) PreferenceRepository { return r }

var _ PreferenceRepository = (*fakePreferenceRepo)(nil)
