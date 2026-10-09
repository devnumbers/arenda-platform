package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEmailChangeGrant_NewSetsTTLOnly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	userID := uuid.Must(uuid.NewV7())

	grant := NewEmailChangeGrant(userID, "token-hash", now)

	if grant.ID == uuid.Nil {
		t.Fatal("grant ID is nil, want an app-generated UUIDv7")
	}
	if grant.UserID != userID {
		t.Fatalf("grant UserID = %s, want %s", grant.UserID, userID)
	}
	// The grant is born on the current-address code check, before any new
	// address is named: it must carry no email until the next step binds one
	// (protocol #1202).
	if grant.Email != nil {
		t.Fatalf("grant Email = %s, want nil (the address binds later)", grant.Email)
	}
	if grant.TokenHash != "token-hash" {
		t.Fatalf("grant TokenHash = %q, want %q", grant.TokenHash, "token-hash")
	}
	if grant.CreatedAt != now {
		t.Fatalf("grant CreatedAt = %s, want %s", grant.CreatedAt, now)
	}
	if want := now.Add(EmailChangeGrantTTL); !grant.ExpiresAt.Equal(want) {
		t.Fatalf("grant ExpiresAt = %s, want %s (TTL 10 minutes)", grant.ExpiresAt, want)
	}
}

func TestEmailChangeGrant_Expired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	grant := NewEmailChangeGrant(uuid.Must(uuid.NewV7()), "h", now)

	tests := []struct {
		name    string
		at      time.Time
		expired bool
	}{
		{"inside the TTL window", now.Add(9 * time.Minute), false},
		{"the last instant of the TTL window", now.Add(EmailChangeGrantTTL - time.Nanosecond), false},
		{"exactly at expiry", now.Add(EmailChangeGrantTTL), true},
		{"after expiry", now.Add(EmailChangeGrantTTL + time.Minute), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := grant.Expired(tt.at); got != tt.expired {
				t.Fatalf("Expired(%s) = %v, want %v", tt.at, got, tt.expired)
			}
		})
	}
}
