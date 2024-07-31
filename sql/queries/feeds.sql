-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at, updated_at, name, url, user_id;

-- name: GetAllFeeds :many
SELECT id, created_at, updated_at, name, url, user_id
FROM feeds;

-- name: GetFeedsByUserID :many
SELECT id, created_at, updated_at, name, url, user_id
FROM feeds
WHERE user_id = $1;

-- name: GetNextFeedsToFetch :many
SELECT id, created_at, updated_at, name, url, user_id, last_fetched_at
FROM feeds
ORDER BY last_fetched_at NULLS FIRST
LIMIT $1;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1;
