package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The same contract tests run against every Store implementation. The
// Postgres run needs TEST_DATABASE_URL (CI starts a postgres service).
func TestMemoryStore(t *testing.T) { runContract(t, func(t *testing.T) Store { return NewMemory() }) }

func TestPostgresStore(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	runContract(t, func(t *testing.T) Store {
		ctx := context.Background()
		// Fresh schema per test.
		db, err := sql.Open("postgres", url)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DROP TABLE IF EXISTS posts, users, schema_migrations`); err != nil {
			t.Fatal(err)
		}
		db.Close()
		p, err := OpenPostgres(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		return p
	})
}

func runContract(t *testing.T, newStore func(*testing.T) Store) {
	ctx := context.Background()

	t.Run("users", func(t *testing.T) {
		s := newStore(t)
		if err := s.CreateUser(ctx, "miles", "hash1"); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateUser(ctx, "miles", "hash2"); !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("duplicate sign-up: got %v", err)
		}
		u, err := s.GetUser(ctx, "miles")
		if err != nil || u.PasswordHash != "hash1" {
			t.Fatalf("GetUser = %+v, %v (original account must not be overwritten)", u, err)
		}
		if _, err := s.GetUser(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing user: got %v", err)
		}
	})

	t.Run("concurrent sign-ups for one name: exactly one wins", func(t *testing.T) {
		s := newStore(t)
		var wins atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if s.CreateUser(ctx, "race", "h") == nil {
					wins.Add(1)
				}
			}()
		}
		wg.Wait()
		if wins.Load() != 1 {
			t.Fatalf("%d sign-ups succeeded, want 1", wins.Load())
		}
	})

	t.Run("posts: search, filter, delete", func(t *testing.T) {
		s := newStore(t)
		for _, u := range []string{"alice", "bob"} {
			if err := s.CreateUser(ctx, u, "h"); err != nil {
				t.Fatal(err)
			}
		}
		base := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
		mk := func(user, msg string, minutes int) Post {
			p := Post{ID: NewID(), Username: user, Message: msg, MediaKey: "k-" + msg, MediaType: MediaImage,
				CreatedAt: base.Add(time.Duration(minutes) * time.Minute)}
			if err := s.CreatePost(ctx, p); err != nil {
				t.Fatal(err)
			}
			return p
		}
		hike := mk("alice", "Sunrise hiking at Mount Rainier", 1)
		mk("alice", "Coffee in Capitol Hill", 2)
		mk("bob", "Hiking the Olympics", 3)

		all, _ := s.ListPosts(ctx, Query{})
		if len(all) != 3 || all[0].Username != "bob" {
			t.Fatalf("expected 3 posts newest first, got %+v", all)
		}
		byAlice, _ := s.ListPosts(ctx, Query{Username: "alice"})
		if len(byAlice) != 2 {
			t.Fatalf("alice has 2 posts, got %d", len(byAlice))
		}
		hiking, _ := s.ListPosts(ctx, Query{Text: "hiking"})
		if len(hiking) != 2 {
			t.Fatalf("expected 2 hiking posts, got %d", len(hiking))
		}
		aliceHiking, _ := s.ListPosts(ctx, Query{Text: "hiking", Username: "alice"})
		if len(aliceHiking) != 1 || aliceHiking[0].ID != hike.ID {
			t.Fatalf("expected alice's hike, got %+v", aliceHiking)
		}

		if _, err := s.DeletePost(ctx, hike.ID, "bob"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("bob deleted alice's post: %v", err)
		}
		key, err := s.DeletePost(ctx, hike.ID, "alice")
		if err != nil || key != hike.MediaKey {
			t.Fatalf("DeletePost = %q, %v", key, err)
		}
		if _, err := s.DeletePost(ctx, "not-a-uuid", "alice"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("bad id: got %v", err)
		}
		if err := s.CreatePost(ctx, Post{ID: NewID(), Username: "ghost", MediaKey: "k", MediaType: MediaImage, CreatedAt: base}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("post for unknown user: got %v", err)
		}
	})
}
