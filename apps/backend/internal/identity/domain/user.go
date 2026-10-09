// Package domain defines the identity domain model: users with roles and timezones, phone and email values,
// sessions, login codes with purposes and attempt windows.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID
	Phone           Phone
	Role            Role
	Name            *string
	Surname         *string
	Patronymic      *string
	Email           *Email
	EmailVerifiedAt *time.Time
	Timezone        Timezone
	// PhotoKey is the storage key of the profile photo (ADR 0065): nil — no
	// photo, a key — the private object streamed by GET /me/photo. The
	// content type travels alongside (PhotoContentType) — the sniffed value
	// of the upload, never a client header.
	PhotoKey         *string
	PhotoContentType *string
}

func NewOwner(phone Phone) (User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	return User{
		ID:    id,
		Phone: phone,
		Role:  RoleOwner,
	}, nil
}

// UpdatePersonalData applies the free-edit profile fields. The email is
// deliberately absent: since #721 the address changes only through the
// confirmed two-code flow, so the old "free edit resets verification" rule is
// gone with it.
func (u *User) UpdatePersonalData(name, surname, patronymic, timezone *string) error {
	if name != nil {
		u.Name = nonEmptyPtr(strings.TrimSpace(*name))
	}
	if surname != nil {
		u.Surname = nonEmptyPtr(strings.TrimSpace(*surname))
	}
	if patronymic != nil {
		u.Patronymic = nonEmptyPtr(strings.TrimSpace(*patronymic))
	}
	if timezone != nil {
		v, err := NewTimezone(*timezone)
		if err != nil {
			return err
		}
		u.Timezone = v
	}
	return nil
}

// VerifyEmail records the verified email address and the moment it was
// confirmed. The email is already a validated value object and the timestamp
// comes from the caller's clock, so this is a pure assignment with no further
// validation or idempotency check — the creation-path always operates on a
// fresh aggregate, and the existing-user path is handled separately in the
// application layer (issue #241).
func (u *User) VerifyEmail(email Email, at time.Time) {
	u.Email = &email
	u.EmailVerifiedAt = &at
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
