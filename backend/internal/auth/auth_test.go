package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-that-is-at-least-32-bytes-long"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash contains the plain-text password")
	}
	if ok, err := VerifyPassword("correct horse battery", hash); err != nil || !ok {
		t.Fatalf("expected match, got %v %v", ok, err)
	}
	if ok, _ := VerifyPassword("wrong password", hash); ok {
		t.Fatal("wrong password matched")
	}
	other, _ := HashPassword("correct horse battery")
	if other == hash {
		t.Fatal("two hashes of the same password should differ (random salt)")
	}
	if _, err := VerifyPassword("x", "plaintext"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestValidateCredentials(t *testing.T) {
	cases := []struct {
		user, pass string
		want       error
	}{
		{"miles_z", "longenough", nil},
		{"ab", "longenough", ErrInvalidUsername},
		{"Bad Name!", "longenough", ErrInvalidUsername}, // accepted by the course version's regex
		{"a", "longenough", ErrInvalidUsername},
		{"valid_user", "short", ErrWeakPassword},
	}
	for _, c := range cases {
		if got := ValidateCredentials(c.user, c.pass); got != c.want {
			t.Errorf("ValidateCredentials(%q, %q) = %v, want %v", c.user, c.pass, got, c.want)
		}
	}
}

func TestTokens(t *testing.T) {
	if _, err := NewTokens("secret", time.Hour); err == nil {
		t.Fatal("short secret should be rejected")
	}
	tokens, _ := NewTokens(testSecret, time.Hour)
	token, err := tokens.Issue("miles")
	if err != nil {
		t.Fatal(err)
	}
	if user, err := tokens.Parse(token); err != nil || user != "miles" {
		t.Fatalf("Parse = %q, %v", user, err)
	}

	// Signed with a different key.
	other, _ := NewTokens("another-secret-that-is-at-least-32-bytes", time.Hour)
	forged, _ := other.Issue("miles")
	if _, err := tokens.Parse(forged); err == nil {
		t.Fatal("token signed with another key was accepted")
	}

	// alg=none must be rejected.
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: "miles", Issuer: issuer}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tokens.Parse(none); err == nil {
		t.Fatal("alg=none token was accepted")
	}

	// Expired.
	tokens.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	old, _ := tokens.Issue("miles")
	tokens.now = time.Now
	if _, err := tokens.Parse(old); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestRequireMiddleware(t *testing.T) {
	tokens, _ := NewTokens(testSecret, time.Hour)
	handler := tokens.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(Username(r.Context())))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d", rec.Code)
	}

	token, _ := tokens.Issue("miles")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "miles" {
		t.Fatalf("valid token: got %d %q", rec.Code, rec.Body.String())
	}
}
