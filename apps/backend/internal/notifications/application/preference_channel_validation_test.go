package application

import (
	"errors"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

func TestValidateChannelPreferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prefs   []domain.NotificationChannelPreference
		wantErr bool
	}{
		{
			name:    "empty set rejected",
			prefs:   nil,
			wantErr: true,
		},
		{
			name:  "full set accepted",
			prefs: domain.DefaultNotificationChannelPreferences(),
		},
		{
			name: "partial set rejected",
			prefs: []domain.NotificationChannelPreference{
				{EventType: domain.EventSubscriptionGrace, Channel: domain.ChannelEmail, Allowed: true},
			},
			wantErr: true,
		},
		{
			name: "too many entries rejected",
			prefs: func() []domain.NotificationChannelPreference {
				prefs := domain.DefaultNotificationChannelPreferences()
				prefs = append(prefs, domain.NotificationChannelPreference{
					EventType: domain.EventSubscriptionGrace,
					Channel:   domain.ChannelEmail,
					Allowed:   true,
				})
				return prefs
			}(),
			wantErr: true,
		},
		{
			name: "duplicate pair rejected",
			prefs: func() []domain.NotificationChannelPreference {
				prefs := domain.DefaultNotificationChannelPreferences()
				// Overwrite the last entry with a duplicate of the first to
				// keep the count at the expected size while introducing a dup.
				prefs[len(prefs)-1] = prefs[0]
				return prefs
			}(),
			wantErr: true,
		},
		{
			name: "unknown channel rejected",
			prefs: func() []domain.NotificationChannelPreference {
				prefs := domain.DefaultNotificationChannelPreferences()
				prefs[0].Channel = domain.NotificationChannel("carrier-pigeon")
				return prefs
			}(),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateChannelPreferences(tt.prefs)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidPreferences) {
				t.Errorf("expected ErrInvalidPreferences, got %v", err)
			}
		})
	}
}
