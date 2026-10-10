package auth

import (
	"errors"
	"regexp"
	"unicode/utf8"
)

// usernamePattern is 3–20 lowercase letters, digits or underscores.
//
// The course version used ^[a-z0-9]$ — which matches exactly one character —
// and rejected names that matched it, so "Bad Name!" passed while "a" failed.
var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

var (
	ErrInvalidUsername = errors.New("username must be 3-20 lowercase letters, digits or underscores")
	ErrWeakPassword    = errors.New("password must be 8-72 characters")
)

func ValidateCredentials(username, password string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	if n := utf8.RuneCountInString(password); n < 8 || n > 72 {
		return ErrWeakPassword
	}
	return nil
}
