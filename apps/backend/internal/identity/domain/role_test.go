package domain

import (
	"errors"
	"testing"
)

func TestNewRole(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Role
		wantErr bool
	}{
		{"owner", "owner", RoleOwner, false},
		{"admin", "admin", RoleAdmin, false},
		{"unknown role", "superuser", "", true},
		{"empty", "", "", true},
		{"case sensitive owner", "Owner", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewRole(tc.input)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidRole) {
					t.Fatalf("NewRole(%q) error = %v, want ErrInvalidRole", tc.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewRole(%q) error = %v, want nil", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NewRole(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRole_String(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleOwner, "owner"},
		{RoleAdmin, "admin"},
	}

	for _, tc := range tests {
		if got := tc.role.String(); got != tc.want {
			t.Fatalf("%v.String() = %q, want %q", tc.role, got, tc.want)
		}
	}
}
