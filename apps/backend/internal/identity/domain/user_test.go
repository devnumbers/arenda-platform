package domain

import (
	"errors"
	"reflect"
	"testing"
)

func ptr(s string) *string {
	p := new(string)
	*p = s
	return p
}

func TestUser_UpdatePersonalData(t *testing.T) {
	cases := []struct {
		name    string
		before  User
		opts    struct{ name, surname, patronymic, email Optional[string] }
		want    User
		wantErr error
	}{
		{
			name: "absent field leaves value unchanged",
			before: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
			opts: struct{ name, surname, patronymic, email Optional[string] }{},
			want: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
		},
		{
			name: "explicit null clears value",
			before: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
			opts: struct{ name, surname, patronymic, email Optional[string] }{
				name: Optional[string]{Set: true},
			},
			want: User{
				Name:       nil,
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
		},
		{
			name: "empty string clears value",
			before: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
			opts: struct{ name, surname, patronymic, email Optional[string] }{
				surname: Optional[string]{Set: true, Value: ""},
			},
			want: User{
				Name:       ptr("Ivan"),
				Surname:    nil,
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
		},
		{
			name: "whitespace-only string clears value",
			before: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
			opts: struct{ name, surname, patronymic, email Optional[string] }{
				patronymic: Optional[string]{Set: true, Value: "   \t\n"},
			},
			want: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: nil,
				Email:      ptr("ivan@example.com"),
			},
		},
		{
			name: "non-empty value is trimmed and stored",
			before: User{
				Name:       ptr("Old"),
				Surname:    ptr("Old"),
				Patronymic: ptr("Old"),
				Email:      ptr("old@example.com"),
			},
			opts: struct{ name, surname, patronymic, email Optional[string] }{
				name:       Optional[string]{Set: true, Value: "  Ivan  "},
				surname:    Optional[string]{Set: true, Value: "\tIvanov"},
				patronymic: Optional[string]{Set: true, Value: "Ivanovich\n"},
				email:      Optional[string]{Set: true, Value: "  Ivan@Example.Com  "},
			},
			want: User{
				Name:       ptr("Ivan"),
				Surname:    ptr("Ivanov"),
				Patronymic: ptr("Ivanovich"),
				Email:      ptr("ivan@example.com"),
			},
		},
		{
			name:   "email is lowercased and trimmed",
			before: User{},
			opts: struct{ name, surname, patronymic, email Optional[string] }{
				email: Optional[string]{Set: true, Value: "  Ivan@Example.COM  "},
			},
			want: User{
				Email: ptr("ivan@example.com"),
			},
		},
		{
			name: "invalid email returns error",
			before: User{
				Email: ptr("valid@example.com"),
			},
			opts: struct{ name, surname, patronymic, email Optional[string] }{
				email: Optional[string]{Set: true, Value: "not-an-email"},
			},
			want: User{
				Email: ptr("valid@example.com"),
			},
			wantErr: ErrInvalidEmail,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := tc.before
			err := u.UpdatePersonalData(tc.opts.name, tc.opts.surname, tc.opts.patronymic, tc.opts.email)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UpdatePersonalData error = %v, want %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(u.Name, tc.want.Name) ||
				!reflect.DeepEqual(u.Surname, tc.want.Surname) ||
				!reflect.DeepEqual(u.Patronymic, tc.want.Patronymic) ||
				!reflect.DeepEqual(u.Email, tc.want.Email) {
				t.Fatalf("UpdatePersonalData produced unexpected user fields; got %+v, want %+v", u, tc.want)
			}
		})
	}
}
