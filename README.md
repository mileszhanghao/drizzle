# SocialAI

A photo and video sharing app with AI image generation. Users sign up, post images or videos with captions, search posts by caption, and can generate an image from a text prompt before posting it.

**Stack:** Go 1.24 (standard library `net/http` routing, `crypto/pbkdf2`) · PostgreSQL 16 with full-text search · JWT (golang-jwt v5) · OpenAI Images API · React 19 · TypeScript · Vite · Ant Design · TanStack Query · Docker · GitHub Actions

## Background

I first built this in LaiOffer's Social AI course (2025): a Go backend using Elasticsearch for users and posts, Google Cloud Storage for media, JWT auth, deployment to Google App Engine, and a React front end that generated images with DALL·E. In 2026 I rebuilt it to fix the problems I found when re-reading the original code. The rebuild was developed with AI pair-programming assistance (Claude).

## What changed from the course version, and why

| Problem in the course version | What the rebuild does |
|---|---|
| **Passwords were stored in plain text** in Elasticsearch, and sign-in searched for a user document whose `password` field matched. | Passwords are hashed with PBKDF2-HMAC-SHA256 (600,000 iterations, random salt) and checked with a constant-time comparison. Unknown user and wrong password return the same message. |
| **The JWT signing key was the hard-coded string `"secret"`**, so anyone could sign a token for any user. | The key comes from `JWT_SECRET`, and the server refuses to start if it is shorter than 32 bytes. Tokens are checked for algorithm (HS256 only), issuer and expiry. Tests cover forged signatures, `alg=none` and expired tokens. |
| **The OpenAI API key was compiled into the browser bundle** (`dangerouslyAllowBrowser: true`), so anyone visiting the site could copy it. | Image generation runs on the server, behind login, with a per-user hourly quota to cap cost. The browser never sees the key. |
| **Sign-up could overwrite an existing account.** It checked whether the username existed, then wrote the user document with the username as its ID. Two sign-ups at the same moment could both pass the check, and the second write replaced the first account. | Usernames are a PostgreSQL primary key, so sign-up is a single atomic insert. A test runs 20 concurrent sign-ups for one name and expects exactly one to succeed. |
| The username check used the regex `^[a-z0-9]$`, which matches exactly one character, and rejected names that matched it. So `Bad Name!` passed and `a` failed. | `^[a-z0-9_]{3,20}$`, applied correctly, with a test for the old failure case. |
| The media type was decided by the file extension, so any file renamed to `.jpg` was accepted. There was no size limit. | The type is detected from the file's bytes (JPEG, PNG, GIF, WebP, MP4, WebM only), with a 50 MB limit. Stored names are server-generated UUIDs, which rules out path traversal. |
| Deleting a post left its media file behind. | Delete removes the database row (owner only) and the file. A failed insert also cleans up the uploaded file. |
| Depended on libraries that are no longer maintained (`form3tech-oss/jwt-go`, `auth0/go-jwt-middleware` v1, `olivere/elastic`). | Two small, maintained dependencies: `golang-jwt/jwt/v5` and the `lib/pq` PostgreSQL driver. Routing uses the standard library. |

### Why PostgreSQL instead of Elasticsearch

The course used Elasticsearch for both users and posts. Users need uniqueness and transactions, which a search engine is not built for. That is exactly how the account-overwrite bug happened. Post search here is caption search over a modest dataset, which PostgreSQL's built-in full-text search handles well: a generated `tsvector` column with a GIN index, `websearch_to_tsquery` for user input (`hiking -rain`, quoted phrases), English stemming (searching "hike" finds "Hiking"), and relevance ranking. That is one datastore instead of two. If the app needed fuzzy matching or very large-scale search, Elasticsearch could be added for posts alone.

## Architecture

```
React (Vite) ──/api──► Go net/http server
                          ├─ auth/     PBKDF2 hashing, JWT issue/verify, auth middleware
                          ├─ store/    Store interface → PostgreSQL (migrations embedded) | in-memory
                          ├─ media/    content-type sniffing, blob store interface → local disk
                          ├─ ai/       OpenAI Images client
                          └─ httpapi/  handlers, validation, per-user AI quota
```

Media storage sits behind an interface. The local-disk implementation could be swapped for Google Cloud Storage or S3 without touching the handlers.

## Running it

```bash
docker compose up --build        # PostgreSQL + app on http://localhost:8080
```

To enable AI image generation, set `OPENAI_API_KEY` before starting.

For local development:

```bash
cd backend && JWT_SECRET=$(openssl rand -hex 32) go run ./cmd/server   # in-memory store unless DATABASE_URL is set
cd web && npm install && npm run dev                                   # UI on :5173, proxies /api and /media to :8080
```

## API

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/signup` | – | `{ "username", "password" }` |
| POST | `/api/signin` | – | Returns `{ "token", "username" }` |
| GET | `/api/posts?q=&user=` | Bearer | List posts, optionally by caption search and/or user |
| POST | `/api/posts` | Bearer | Multipart: `message`, `media` (image or video) |
| DELETE | `/api/posts/{id}` | Bearer | Delete your own post and its file |
| POST | `/api/ai/images` | Bearer | `{ "prompt" }` → PNG preview as a data URL |
| GET | `/media/{key}` | – | Serve a stored file |

## Tests

```bash
cd backend && go test -race ./...                                   # Postgres tests run when TEST_DATABASE_URL is set
cd web && npm test
```

The store tests are one contract suite that runs against both the in-memory store and real PostgreSQL. In CI, PostgreSQL runs as a service container. The HTTP tests start the full router and cover sign-up and sign-in, auth enforcement, upload validation (a text file named `photo.jpg`, path traversal), ownership checks on delete, and the AI endpoint's quota and failure handling.
