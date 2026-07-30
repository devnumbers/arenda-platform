package domain

import "github.com/google/uuid"

// LeaseIDPtr returns a pointer to id if it is not the zero UUID; otherwise it returns nil.
func LeaseIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// PropertyIDPtr returns a pointer to id if it is not the zero UUID; otherwise
// it returns nil. PropertyID follows the Nil convention: uuid.Nil means the
// entity is not attached to a property (the property was deleted in detach
// mode), the same convention as LeaseID and RecurringOperationID.
func PropertyIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
