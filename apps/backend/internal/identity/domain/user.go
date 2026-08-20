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

func (u *User) UpdatePersonalData(name, surname, patronymic, email, timezone *string) error {
	if name != nil {
		u.Name = nonEmptyPtr(strings.TrimSpace(*name))
	}
	if surname != nil {
		u.Surname = nonEmptyPtr(strings.TrimSpace(*surname))
	}
	if patronymic != nil {
		u.Patronymic = nonEmptyPtr(strings.TrimSpace(*patronymic))
	}
	if email != nil {
		v, err := NewEmail(*email)
		if err != nil {
			return err
		}
		// Changing the email invalidates verification: a new address must be
		// confirmed again before it is trusted. Compare the normalized values
		// before overwriting u.Email so an identical resubmit leaves the
		// verified flag untouched.
		if u.Email != nil && u.Email.String() != v.String() {
			u.EmailVerifiedAt = nil
		}
		u.Email = &v
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
