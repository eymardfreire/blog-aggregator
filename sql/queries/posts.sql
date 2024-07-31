-- queries/posts.sql

-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at, updated_at, title, url, description, published_at, feed_id;

-- name: GetPostsByUserID :many
SELECT id, created_at, updated_at, title, url, description, published_at, feed_id
FROM posts
WHERE feed_id IN (
    SELECT feed_id
    FROM feed_follows
    WHERE user_id = $1
)
ORDER BY published_at DESC
LIMIT $2;
