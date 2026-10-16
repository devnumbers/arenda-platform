package wire

import "testing"

// TestBaseURLHost pins the domain extraction feeding the SMTP HeloName: the
// stage/prod values must greet relays as dev.rentlee.ru / rentlee.ru, local
// development keeps "localhost", and garbage degrades to "" (the stdlib
// default greeting) instead of failing wiring.
func TestBaseURLHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want string
	}{
		{"https://dev.rentlee.ru", "dev.rentlee.ru"},
		{"https://rentlee.ru/", "rentlee.ru"},
		{"http://localhost:3000", "localhost"},
		{"http://127.0.0.1:8080", "127.0.0.1"},
		{"", ""},
		{"://broken", ""},
	}

	for _, tc := range cases {
		if got := baseURLHost(tc.raw); got != tc.want {
			t.Errorf("baseURLHost(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
