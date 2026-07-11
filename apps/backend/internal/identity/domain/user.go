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
}

func NewOwner(phone Phone) (User, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return User{}, err
	}
	return User{
		ID:    id,
		Phone: phone,
		Role:  RoleOwner,
	}, nil
}

func (u *User) UpdatePersonalData(name, surname, patronymic, email *string) error {
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
		u.Email = &v
	}
	return nil
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
