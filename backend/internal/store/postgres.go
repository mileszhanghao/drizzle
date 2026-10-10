package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Postgres stores users and posts in PostgreSQL and searches post messages
// with built-in full-text search (a generated tsvector column + GIN index).
//
// The course version used Elasticsearch for both users and posts. User
// documents were written with the username as the document ID after a
// separate "does it exist?" query, so two concurrent sign-ups for the same
// name could both pass the check and the second write silently replaced the
// first account. Here a primary key makes sign-up atomic.
type Postgres struct {
	db *sql.DB
}

func OpenPostgres(ctx context.Context, url string) (*Postgres, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	p := &Postgres{db: db}
	if err := p.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return p, nil
}

func (p *Postgres) Close() error { return p.db.Close() }

// migrate applies embedded migrations in order, once each, under an advisory
// lock so that two instances starting together don't race.
func (p *Postgres) migrate(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(7426001)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var done bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *Postgres) CreateUser(ctx context.Context, username, hash string) error {
	_, err := p.db.ExecContext(ctx, `INSERT INTO users (username, password_hash) VALUES ($1, $2)`, username, hash)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
		return ErrUsernameTaken
	}
	return err
}

func (p *Postgres) GetUser(ctx context.Context, username string) (User, error) {
	var u User
	err := p.db.QueryRowContext(ctx, `SELECT username, password_hash, created_at FROM users WHERE username = $1`, username).
		Scan(&u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (p *Postgres) CreatePost(ctx context.Context, post Post) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO posts (id, username, message, media_key, media_type, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		post.ID, post.Username, post.Message, post.MediaKey, string(post.MediaType), post.CreatedAt)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23503" { // foreign_key_violation
		return ErrNotFound
	}
	return err
}

func (p *Postgres) ListPosts(ctx context.Context, q Query) ([]Post, error) {
	var where []string
	var args []any
	if q.Username != "" {
		args = append(args, q.Username)
		where = append(where, fmt.Sprintf("username = $%d", len(args)))
	}
	order := "created_at DESC"
	if strings.TrimSpace(q.Text) != "" {
		args = append(args, q.Text)
		n := len(args)
		// websearch_to_tsquery accepts user input safely: quotes, -exclusions, OR.
		where = append(where, fmt.Sprintf("search @@ websearch_to_tsquery('english', $%d)", n))
		order = fmt.Sprintf("ts_rank(search, websearch_to_tsquery('english', $%d)) DESC, created_at DESC", n)
	}
	args = append(args, clampLimit(q.Limit))
	query := `SELECT id, username, message, media_key, media_type, created_at FROM posts`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY %s LIMIT $%d", order, len(args))

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		var post Post
		var mediaType string
		if err := rows.Scan(&post.ID, &post.Username, &post.Message, &post.MediaKey, &mediaType, &post.CreatedAt); err != nil {
			return nil, err
		}
		post.MediaType = MediaType(mediaType)
		post.CreatedAt = post.CreatedAt.UTC()
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func (p *Postgres) DeletePost(ctx context.Context, id, username string) (string, error) {
	var key string
	err := p.db.QueryRowContext(ctx,
		`DELETE FROM posts WHERE id = $1 AND username = $2 RETURNING media_key`, id, username).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "22P02" { // invalid UUID text
		return "", ErrNotFound
	}
	return key, err
}
