// Package domain defines the property domain model: properties with per-type attributes, photo metadata and
// property contacts.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

type PropertyType string

const (
	PropertyTypeApartment  PropertyType = "apartment"
	PropertyTypeRoom       PropertyType = "room"
	PropertyTypeApartments PropertyType = "apartments"
	PropertyTypeHouse      PropertyType = "house"
	PropertyTypeCommercial PropertyType = "commercial"
	PropertyTypeOffice     PropertyType = "office"
	PropertyTypeWarehouse  PropertyType = "warehouse"
	PropertyTypeGarage     PropertyType = "garage"
	PropertyTypeParking    PropertyType = "parking"
	PropertyTypeLand       PropertyType = "land"
)

var ErrInvalidPropertyType = errors.New("invalid property type")

func ParsePropertyType(s string) (PropertyType, error) {
	t := PropertyType(s)
	if !t.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidPropertyType, s)
	}
	return t, nil
}

func (t PropertyType) Valid() bool {
	switch t {
	case PropertyTypeApartment,
		PropertyTypeRoom,
		PropertyTypeApartments,
		PropertyTypeHouse,
		PropertyTypeCommercial,
		PropertyTypeOffice,
		PropertyTypeWarehouse,
		PropertyTypeGarage,
		PropertyTypeParking,
		PropertyTypeLand:
		return true
	}
	return false
}

type PropertyStatus string

const (
	PropertyStatusActive      PropertyStatus = "active"
	PropertyStatusMaintenance PropertyStatus = "maintenance"
	PropertyStatusArchived    PropertyStatus = "archived"
)

var ErrInvalidPropertyStatus = errors.New("invalid property status")

func ParsePropertyStatus(s string) (PropertyStatus, error) {
	st := PropertyStatus(s)
	if !st.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidPropertyStatus, s)
	}
	return st, nil
}

func (s PropertyStatus) Valid() bool {
	switch s {
	case PropertyStatusActive,
		PropertyStatusMaintenance,
		PropertyStatusArchived:
		return true
	}
	return false
}

// OccupancyStatus is the computed occupancy state of a property («Занятость
// объекта», резолюция #584): derived from the property's single unfinished
// rental against the data owner's today (ADR 0048) — never from the property
// lifecycle status (active/maintenance/archived).
type OccupancyStatus string

// The four occupancy states: the three unfinished-rental statuses (the
// rentals vocabulary, rentals/CONTEXT.md «Статусы Аренды») plus the
// no-rental state.
const (
	// OccupancyUpcoming — the unfinished rental starts later («Аренда с DD.MM»).
	OccupancyUpcoming OccupancyStatus = "upcoming"
	// OccupancyActive — the unfinished rental runs.
	OccupancyActive OccupancyStatus = "active"
	// OccupancyNeedsAttention — the rental's planned end has passed and the
	// rental is not completed («Аренда завершена» badge, the red dot).
	OccupancyNeedsAttention OccupancyStatus = "needs_attention"
	// OccupancyNone — no unfinished rental.
	OccupancyNone OccupancyStatus = "none"
)

// Occupancy is the per-property occupancy projection (ticket #585): the
// read-side view the lists report for the badges, the red dot and the «По
// статусу» sorting (резолюция #584). The rental fields describe the
// property's single unfinished rental; a property without one reports
// OccupancyNone with both dates nil.
type Occupancy struct {
	Status OccupancyStatus
	// StartDate is the unfinished rental's start («Аренда с DD.MM» of an
	// upcoming rental).
	StartDate *time.Time
	// PlannedEndDate is the unfinished rental's planned end («Осталось N
	// месяцев»); nil for an open-ended rental.
	PlannedEndDate *time.Time
}

type Property struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Type        PropertyType
	Address     string
	Description string
	Attributes  Attributes
	Status      PropertyStatus
	Photos      []Photo
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// MembersCount is the shared-access participant count: membership rows
	// (any status; the owner is never a membership row) plus pending email
	// invitations (issue #163). Filled by the read queries that carry the
	// members_count projection; zero on write-path responses (create/update).
	MembersCount int
	// AccessRole is the role of the requesting actor on this property
	// (issue T11): RoleOwner for own properties, the membership role for
	// shared ones. Empty when not populated (internal use). Filled by the
	// read/write service paths that resolve the actor's role.
	AccessRole sharedpolicy.Role
	// PinnedAt is the global pin (ticket #577): nil — not pinned, a moment —
	// pinned since then. The lists order the pinned first, among themselves
	// by this time; archiving clears it.
	PinnedAt *time.Time
	// Occupancy is the «Занятость» projection of the list reads (ticket
	// #585, резолюция #584): nil when not computed — the write paths and an
	// unwired projection port; the list reads always report it.
	Occupancy *Occupancy
	// HasOverdueOperations is the payments half of the red dot (ticket #585,
	// резолюция #584): the property has overdue planned operations — stored
	// planned rows dated before the data owner's today (ADR 0048). Reported
	// by the list reads alongside Occupancy; false when not computed.
	HasOverdueOperations bool
	// OwnerName is the public display name of the property owner ("Name
	// Surname" or a masked phone, never an email), filled when the actor is
	// not the owner: by the detail read (issue T11) and by the list reads
	// for shared rows (owner decision on the #756 walkthrough fixes); empty
	// otherwise.
	OwnerName string
}

// Photo is a photo attached to a property.
type Photo struct {
	ID  uuid.UUID
	URL string
}

var (
	ErrEmptyPropertyName          = errors.New("property name is empty")
	ErrPropertyNameTooLong        = errors.New("property name exceeds 255 characters")
	ErrEmptyPropertyAddress       = errors.New("property address is empty")
	ErrPropertyAddressTooLong     = errors.New("property address exceeds 500 characters")
	ErrPropertyDescriptionTooLong = errors.New("property description exceeds 2000 characters")
)

func NewProperty(ownerID uuid.UUID, name, address, description string, propertyType PropertyType, attrs Attributes) (Property, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Property{}, fmt.Errorf("generate property id: %w", err)
	}

	if attrs == nil {
		attrs = Attributes{}
	}

	p := Property{
		ID:          id,
		OwnerID:     ownerID,
		Name:        name,
		Type:        propertyType,
		Address:     address,
		Description: description,
		Attributes:  attrs,
		Status:      PropertyStatusActive,
	}

	if err := p.Validate(); err != nil {
		return Property{}, err
	}

	return p, nil
}

func (p Property) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ErrEmptyPropertyName
	}
	if len(p.Name) > 255 {
		return ErrPropertyNameTooLong
	}
	if strings.TrimSpace(p.Address) == "" {
		return ErrEmptyPropertyAddress
	}
	if len(p.Address) > 500 {
		return ErrPropertyAddressTooLong
	}
	if len(p.Description) > 2000 {
		return ErrPropertyDescriptionTooLong
	}
	if !p.Type.Valid() {
		return ErrInvalidPropertyType
	}
	if !p.Status.Valid() {
		return ErrInvalidPropertyStatus
	}
	return nil
}
