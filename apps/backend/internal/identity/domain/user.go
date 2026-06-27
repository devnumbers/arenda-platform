package domain

import (
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

var ErrInvalidEmail = errors.New("invalid email")

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
		v := strings.TrimSpace(*email)
		if v != "" && !isValidEmail(v) {
			return ErrInvalidEmail
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

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
