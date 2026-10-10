CREATE TABLE users
(
    username      TEXT PRIMARY KEY,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE posts
(
    id         UUID PRIMARY KEY,
    username   TEXT        NOT NULL REFERENCES users (username) ON DELETE CASCADE,
    message    TEXT        NOT NULL DEFAULT '',
    media_key  TEXT        NOT NULL,
    media_type TEXT        NOT NULL CHECK (media_type IN ('image', 'video')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Full-text index over the message, maintained by PostgreSQL itself.
    search     TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', message)) STORED
);

CREATE INDEX posts_search_idx ON posts USING GIN (search);
CREATE INDEX posts_user_created_idx ON posts (username, created_at DESC);
CREATE INDEX posts_created_idx ON posts (created_at DESC);
