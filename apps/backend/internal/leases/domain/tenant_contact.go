package domain

import (
	"time"

	"github.com/google/uuid"
)

// TenantContact mirrors the minimal contact information needed by the leases
// bounded context. The authoritative tenant contacts module owns the full
// lifecycle; this type is used only as a read-model reference here.
type TenantContact struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Name       string
	Surname    *string
	Patronymic *string
	Phone      *string
	Email      *string
	Comment    *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
