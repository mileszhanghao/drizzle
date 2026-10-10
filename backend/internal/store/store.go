// Package store persists users and posts.
package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrUsernameTaken = errors.New("username already taken")
)

type User struct {
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type MediaType string

const (
	MediaImage MediaType = "image"
	MediaVideo MediaType = "video"
)

type Post struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	MediaKey  string    `json:"-"`
	MediaType MediaType `json:"mediaType"`
	CreatedAt time.Time `json:"createdAt"`
}

// Query filters posts. Empty fields match everything.
type Query struct {
	// Text is a full-text query over post messages (e.g. `hiking -rain`).
	Text     string
	Username string
	Limit    int
}

type Store interface {
	// CreateUser fails with ErrUsernameTaken if the name exists. It must be
	// atomic: two concurrent sign-ups for one name cannot both succeed.
	CreateUser(ctx context.Context, username, passwordHash string) error
	GetUser(ctx context.Context, username string) (User, error)
	CreatePost(ctx context.Context, p Post) error
	ListPosts(ctx context.Context, q Query) ([]Post, error)
	// DeletePost removes a post owned by username and returns its media key.
	// It returns ErrNotFound if the post does not exist or belongs to someone else.
	DeletePost(ctx context.Context, id, username string) (string, error)
}

// NewID returns a random RFC 4122 version 4 UUID.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func clampLimit(n int) int {
	if n <= 0 || n > 100 {
		return 50
	}
	return n
}
