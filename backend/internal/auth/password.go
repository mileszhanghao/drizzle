// Package auth handles password hashing, JWT issuing and request authentication.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// OWASP's 2023 recommendation for PBKDF2-HMAC-SHA256.
const (
	pbkdf2Iterations = 600_000
	saltBytes        = 16
	keyBytes         = 32
)

// HashPassword returns a self-describing hash string:
//
//	pbkdf2-sha256$<iterations>$<salt b64>$<key b64>
//
// The course version stored passwords in plain text in Elasticsearch and
// logged users in by searching for a document with a matching password field.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, keyBytes)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

var errBadHash = errors.New("malformed password hash")

// VerifyPassword reports whether password matches the stored hash, using a
// constant-time comparison.
func VerifyPassword(password, stored string) (bool, error) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false, errBadHash
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return false, errBadHash
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil {
		return false, errBadHash
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil {
		return false, errBadHash
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
