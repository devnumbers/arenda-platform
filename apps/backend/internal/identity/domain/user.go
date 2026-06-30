package domain

import (
	"strings"

	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
)

type User struct {
	ID         uuid.UUID
	Phone      Phone
	Role       Role
	Name       *string
	Surname    *string
	Patronymic *string
	Email      *string
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

func (u *User) UpdatePersonalData(name, surname, patronymic, email Optional[string]) error {
	if name.Set {
		u.Name = nonEmptyPtr(strings.TrimSpace(name.Value))
	}
	if surname.Set {
		u.Surname = nonEmptyPtr(strings.TrimSpace(name.Value))
	}
	if patronymic.Set {
		u.Patronymic = nonEmptyPtr(strings.TrimSpace(name.Value))
	}
	if email.Set {
		v, err := NormalizeEmail(email.Value)
		if err != nil {
			return err
		}
		u.Email = nonEmptyPtr(v)
	}
	return nil
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
