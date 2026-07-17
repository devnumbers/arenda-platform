package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

type PropertyOccupancy string

const (
	OccupancyFree     PropertyOccupancy = "free"
	OccupancyOccupied PropertyOccupancy = "occupied"
)

type Property struct {
	ID               uuid.UUID
	OwnerID          uuid.UUID
	Name             string
	Type             PropertyType
	Address          string
	Description      string
	Status           PropertyStatus
	Occupancy        PropertyOccupancy
	Photos           []Photo
	CreatedAt        time.Time
	UpdatedAt        time.Time
	OverdueRentCount int
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

func NewProperty(ownerID uuid.UUID, name, address, description string, propertyType PropertyType) (Property, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Property{}, fmt.Errorf("generate property id: %w", err)
	}

	p := Property{
		ID:          id,
		OwnerID:     ownerID,
		Name:        name,
		Type:        propertyType,
		Address:     address,
		Description: description,
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
