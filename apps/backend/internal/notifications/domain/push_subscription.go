package domain

import (
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Length bounds for the Web Push subscription fields. The p256dh client public
// key and the auth secret are base64url-encoded (RFC 8291): a 65-byte
// uncompressed P-256 key encodes to 87 chars, a 16-byte auth secret to 22
// chars. We accept a generous range rather than an exact length so future
// encodings (compressed keys, padding) are not rejected.
const (
	pushEndpointMinLen = 1
	pushEndpointMaxLen = 2048
	pushP256dhMinLen   = 1
	pushP256dhMaxLen   = 512
	pushAuthMinLen     = 1
	pushAuthMaxLen     = 512
)

// ErrInvalidPushSubscription is returned when a push subscription field fails
// validation.
var ErrInvalidPushSubscription = errors.New("invalid push subscription")

// PushSubscription is a stored Web Push subscription: the browser-issued push
// endpoint URL plus the per-subscription ECDH P-256 public key (p256dh) and
// authentication secret (auth) needed to encrypt payloads (RFC 8291). One row
// represents one device/browser; the endpoint is globally unique.
type PushSubscription struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Endpoint       string
	P256dh         string
	Auth           string
	ExpirationTime *time.Time
	// Enabled is the device's master push toggle «Получать пуш-уведомления»
	// (решение #738): turning it off keeps the subscription and the category
	// flags, dispatch just skips the device — re-enabling is instant.
	Enabled bool
	// Categories is the device's own copy of the four configurable category
	// flags (per-device push settings, решение #738).
	Categories CategoryPrefs
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Accepts is the device's delivery verdict for one category: the master
// toggle gates everything (including the always-on service categories —
// master-off mutes the device entirely), the category flag gates its own
// category.
func (s PushSubscription) Accepts(c Category) bool {
	return s.Enabled && s.Categories.Allows(c)
}

// ValidatePushSubscription checks the invariants of a stored subscription's
// mutable fields: a non-empty https (or http for local dev) endpoint URL and
// non-empty, length-bounded p256dh/auth secrets. It does not decode the
// base64url payloads — that is the push-sender's job.
func ValidatePushSubscription(endpoint, p256dh, auth string) error {
	if err := validatePushEndpoint(endpoint); err != nil {
		return err
	}
	if err := validateBoundedField(p256dh, pushP256dhMinLen, pushP256dhMaxLen, "p256dh"); err != nil {
		return err
	}
	if err := validateBoundedField(auth, pushAuthMinLen, pushAuthMaxLen, "auth"); err != nil {
		return err
	}
	return nil
}

// validatePushEndpoint requires a non-empty absolute http(s) URL. Push services
// always issue https endpoints; http is permitted for local fakes/testing.
func validatePushEndpoint(endpoint string) error {
	if err := validateBoundedField(endpoint, pushEndpointMinLen, pushEndpointMaxLen, "endpoint"); err != nil {
		return err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return errors.Join(ErrInvalidPushSubscription, err)
	}
	if !u.IsAbs() {
		return errors.Join(ErrInvalidPushSubscription, errors.New("endpoint must be an absolute URL"))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.Join(ErrInvalidPushSubscription, errors.New("endpoint scheme must be http or https"))
	}
	if u.Host == "" {
		return errors.Join(ErrInvalidPushSubscription, errors.New("endpoint must have a host"))
	}
	return nil
}

func validateBoundedField(value string, minLen, maxLen int, name string) error {
	if len(value) < minLen {
		return errors.Join(ErrInvalidPushSubscription, errors.New(name+" is too short"))
	}
	if len(value) > maxLen {
		return errors.Join(ErrInvalidPushSubscription, errors.New(name+" is too long"))
	}
	return nil
}
