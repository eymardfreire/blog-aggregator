-- 007_add_posts_table.sql
CREATE TABLE posts (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    description TEXT,
    published_at TIMESTAMP,
    feed_id UUID REFERENCES feeds(id) ON DELETE CASCADE
);

-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at, updated_at, title, url, description, published_at, feed_id;

-- name: GetPostsByUserID :many
SELECT p.id, p.created_at, p.updated_at, p.title, p.url, p.description, p.published_at, p.feed_id
FROM posts p
JOIN feeds f ON p.feed_id = f.id
JOIN feed_follows ff ON f.id = ff.feed_id
WHERE ff.user_id = $1
ORDER BY p.created_at DESC
LIMIT $2;

-- name: GetNextFeedsToFetch :many
SELECT id, created_at, updated_at, name, url, description, published_at, feed_id
FROM posts
WHERE last_fetched_at IS NULL OR last_fetched_at < NOW() - INTERVAL '1 HOUR'
ORDER BY last_fetched_at ASC
LIMIT $1;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1;
