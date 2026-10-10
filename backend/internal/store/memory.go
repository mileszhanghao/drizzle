package store

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
)

// Memory is an in-process Store for tests and quick local runs. Its text
// search is a simple case-insensitive "all words present" match.
type Memory struct {
	mu    sync.Mutex
	users map[string]User
	posts []Post
}

func NewMemory() *Memory {
	return &Memory{users: map[string]User{}}
}

func (m *Memory) CreateUser(_ context.Context, username, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[username]; ok {
		return ErrUsernameTaken
	}
	m.users[username] = User{Username: username, PasswordHash: hash, CreatedAt: time.Now()}
	return nil
}

func (m *Memory) GetUser(_ context.Context, username string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *Memory) CreatePost(_ context.Context, p Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[p.Username]; !ok {
		return ErrNotFound
	}
	m.posts = append(m.posts, p)
	return nil
}

func (m *Memory) ListPosts(_ context.Context, q Query) ([]Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	words := strings.Fields(strings.ToLower(q.Text))
	var out []Post
	for _, p := range m.posts {
		if q.Username != "" && p.Username != q.Username {
			continue
		}
		msg := strings.ToLower(p.Message)
		if !allContained(msg, words) {
			continue
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b Post) int { return b.CreatedAt.Compare(a.CreatedAt) })
	if limit := clampLimit(q.Limit); len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func allContained(s string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

func (m *Memory) DeletePost(_ context.Context, id, username string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.posts {
		if p.ID == id && p.Username == username {
			m.posts = slices.Delete(m.posts, i, i+1)
			return p.MediaKey, nil
		}
	}
	return "", ErrNotFound
}
