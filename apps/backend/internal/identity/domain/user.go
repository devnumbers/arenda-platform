package domain

import "github.com/google/uuid"

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
