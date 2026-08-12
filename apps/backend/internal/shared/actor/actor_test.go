package actor

import (
	"errors"
	"testing"
)

func TestNewRole_Valid(t *testing.T) {
	cases := []struct {
		raw  string
		want Role
	}{
		{"owner", RoleOwner},
		{"admin", RoleAdmin},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := NewRole(tc.raw)
			if err != nil {
				t.Fatalf("NewRole(%q) returned unexpected error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("NewRole(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			if got.String() != tc.raw {
				t.Fatalf("String() = %q, want %q", got.String(), tc.raw)
			}
		})
	}
}

func TestNewRole_Invalid(t *testing.T) {
	cases := []string{"", "superadmin", "OWNER", "Owner", "none", "viewer", "system"}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := NewRole(raw)
			if err == nil {
				t.Fatalf("NewRole(%q) = %q, want error", raw, got)
			}
			if !errors.Is(err, ErrInvalidRole) {
				t.Fatalf("NewRole(%q) err = %v, want errors.Is ErrInvalidRole", raw, err)
			}
			if got != "" {
				t.Fatalf("NewRole(%q) zero value = %q, want empty", raw, got)
			}
		})
	}
}

func TestRole_String(t *testing.T) {
	if got := RoleOwner.String(); got != "owner" {
		t.Fatalf("RoleOwner.String() = %q, want %q", got, "owner")
	}
	if got := RoleAdmin.String(); got != "admin" {
		t.Fatalf("RoleAdmin.String() = %q, want %q", got, "admin")
	}
}
