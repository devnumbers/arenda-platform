package domain

import "github.com/google/uuid"

// LeaseIDPtr returns a pointer to id if it is not the zero UUID; otherwise it returns nil.
func LeaseIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
