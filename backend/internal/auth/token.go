package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MinSecretBytes guards against weak signing keys. The course version
// hard-coded the key "secret", so anyone could forge a token for any user.
const MinSecretBytes = 32

const issuer = "drizzle"

// Tokens issues and verifies HS256 JWTs.
type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokens(secret string, ttl time.Duration) (*Tokens, error) {
	if len(secret) < MinSecretBytes {
		return nil, errors.New("JWT secret must be at least 32 bytes")
	}
	return &Tokens{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

func (t *Tokens) Issue(username string) (string, error) {
	now := t.now()
	claims := jwt.RegisteredClaims{
		Subject:   username,
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

// Parse validates the signature, algorithm, issuer and expiry and returns the username.
func (t *Tokens) Parse(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("token has no subject")
	}
	return claims.Subject, nil
}

type ctxKey struct{}

// Require rejects requests without a valid bearer token and puts the
// username in the request context.
func (t *Tokens) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}
		username, err := t.Parse(raw)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, username)))
	})
}

// Username returns the authenticated user set by Require.
func Username(ctx context.Context) string {
	name, _ := ctx.Value(ctxKey{}).(string)
	return name
}
