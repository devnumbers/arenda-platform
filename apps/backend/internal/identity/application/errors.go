package application

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrUserBlocked         = errors.New("user is temporarily blocked")
	ErrCodeSentTooRecently = errors.New("code sent too recently")
	ErrPhoneAlreadyTaken   = errors.New("phone already taken")
	ErrPhoneUnchanged      = errors.New("new phone must differ from current phone")
	ErrEmailAlreadyTaken   = errors.New("email already taken")
	ErrEmailDoesNotMatch   = errors.New("email does not match the phone number")
)
