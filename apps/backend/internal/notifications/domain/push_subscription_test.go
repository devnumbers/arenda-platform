package domain

import (
	"errors"
	"testing"
)

func TestValidatePushSubscription(t *testing.T) {
	t.Parallel()

	validEndpoint := "https://fcm.googleapis.com/fcm/send/cid"
	validP256dh := "BG3bT1r6xXm2Na3pH4d5sE7F8aN9o0pQ1rS2tU3vW4xY5zA6bC7dE8fG9hI0jK1lM"
	validAuth := "n9o0pQ1rS2tU3vW4xY5"

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		if err := ValidatePushSubscription(validEndpoint, validP256dh, validAuth); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("http endpoint allowed (local fakes)", func(t *testing.T) {
		t.Parallel()
		if err := ValidatePushSubscription("http://localhost:8080/push/abc", validP256dh, validAuth); err != nil {
			t.Fatalf("expected no error for http endpoint, got %v", err)
		}
	})

	cases := []struct {
		name     string
		endpoint string
		p256dh   string
		auth     string
	}{
		{"empty endpoint", "", validP256dh, validAuth},
		{"relative endpoint", "/push/abc", validP256dh, validAuth},
		{"non-http scheme", "ftp://example.com/push", validP256dh, validAuth},
		{"missing host", "https:///push/abc", validP256dh, validAuth},
		{"empty p256dh", validEndpoint, "", validAuth},
		{"empty auth", validEndpoint, validP256dh, ""},
		{"endpoint too long", "https://example.com/" + string(make([]byte, 2100)), validP256dh, validAuth},
		{"p256dh too long", validEndpoint, string(make([]byte, 600)), validAuth},
		{"auth too long", validEndpoint, validP256dh, string(make([]byte, 600))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePushSubscription(tc.endpoint, tc.p256dh, tc.auth)
			if !errors.Is(err, ErrInvalidPushSubscription) {
				t.Fatalf("expected ErrInvalidPushSubscription, got %v", err)
			}
		})
	}
}
