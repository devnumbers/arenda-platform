// Package domain holds the Contacts context core: the contact card of the
// owner's contact book (ADR 0054). Everything here is pure computation — no
// clocks, no I/O.
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ErrInvalidInput marks a contact that violates the create/update contract
// (empty or overlong name, malformed phone or email).
// ErrInvalidPhone marks a phone that is not a Russian number in any accepted
// spelling; ErrInvalidEmail marks a malformed email address.
var (
	ErrInvalidInput = errors.New("contacts: invalid input")
	ErrInvalidPhone = errors.New("invalid Russian phone number")
	ErrInvalidEmail = errors.New("invalid email")
)

// MaxNameLength bounds the name fields (first, last, patronymic), counted in
// characters — a product copy limit, not a byte limit (the demolished
// property_contacts vertical capped its name the same way).
const MaxNameLength = 255

// Contact is one card of the owner's contact book: a person useful for a
// property — plumber, management company, concierge (ADR 0054). It is not a
// service user and never has an account. The optional property binding makes
// it visible to the property's shared members; with no binding the contact
// lives in the owner's book alone.
//
// Optional text fields use "" for «not set»; the persistence adapter maps ""
// onto NULL. PropertyID nil means «без объекта».
type Contact struct {
	ID uuid.UUID
	// OwnerID is the book the contact lives in: the data owner's id (ADR
	// 0028 scope). A shared member's create lands the card in the property
	// owner's book, never the actor's own.
	OwnerID uuid.UUID
	// PropertyID is the optional property binding; nil survives the
	// property's deletion (FK ON DELETE SET NULL, ADR 0054).
	PropertyID *uuid.UUID
	FirstName  string
	LastName   string
	Patronymic string
	// Role is what the person is to the property — «сантехник», «консьерж».
	// Free text, never an access role (contacts/CONTEXT.md «Роль»).
	Role string
	// Phone is stored normalized to +7XXXXXXXXXX; "" when absent.
	Phone string
	Email string
	// Messenger is one pair: the messenger's name (any service) and the
	// username in it (contacts/CONTEXT.md «Мессенджер»).
	MessengerName     string
	MessengerUsername string
	// Note is the free-text reminder — door codes, what to ask on the call.
	Note string
	// CreatedAt/UpdatedAt are the row's timestamps (updated_at is
	// trigger-maintained on write).
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FullName returns the contact's name joined from the non-empty parts —
// the display form the admin surface composes (ADR 0054 consequences).
func (c Contact) FullName() string {
	return strings.Join(nonEmpty(c.FirstName, c.LastName, c.Patronymic), " ")
}

var (
	phoneRegex = regexp.MustCompile(`^(?:\+7|7|8)(\d{10})$`)
	emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// NormalizePhone converts a Russian phone number to the canonical
// +7XXXXXXXXXX format (the demolished properties vertical's rule, kept
// verbatim per ticket #506).
func NormalizePhone(phone string) (string, error) {
	digits := phoneRegex.FindStringSubmatch(strings.TrimSpace(phone))
	if digits == nil {
		return "", ErrInvalidPhone
	}
	return "+7" + digits[1], nil
}

// ValidatePhone validates a Russian phone number: any 10 digits after the
// country code — +7XXXXXXXXXX, 7XXXXXXXXXX and 8XXXXXXXXXX.
func ValidatePhone(phone string) error {
	_, err := NormalizePhone(phone)
	return err
}

// ValidateEmailFormat checks the email's shape. The same loose
// «something@tld.tld» contract the identity context validates with — a
// contact's email is never mailed to by this context.
func ValidateEmailFormat(email string) error {
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}

// Validate is the single validator of the contact invariants — every writer
// (create and update alike) delegates here, so the rules cannot drift between
// layers. It enforces: a non-empty first name and every name field within
// MaxNameLength characters (counted in runes — the limit is a product copy
// limit), a normalized-or-absent phone and a well-formed-or-absent email. The
// remaining text fields are free-form. Callers normalize the draft (trim,
// fold whitespace-only optionals to "") before validating.
func Validate(c Contact) error {
	if strings.TrimSpace(c.FirstName) == "" {
		return ErrInvalidInput
	}
	for _, name := range []string{c.FirstName, c.LastName, c.Patronymic} {
		if utf8.RuneCountInString(strings.TrimSpace(name)) > MaxNameLength {
			return ErrInvalidInput
		}
	}
	if c.Phone != "" {
		if _, err := NormalizePhone(c.Phone); err != nil {
			return ErrInvalidInput
		}
	}
	if c.Email != "" {
		if err := ValidateEmailFormat(c.Email); err != nil {
			return ErrInvalidInput
		}
	}
	return nil
}

func nonEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
