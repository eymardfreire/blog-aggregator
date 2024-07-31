-- name: GetNextFeedsToFetch :many
SELECT id, created_at, updated_at, name, url, description, last_fetched_at
FROM feeds
WHERE last_fetched_at IS NULL OR last_fetched_at < NOW() - INTERVAL '1 HOUR'
ORDER BY last_fetched_at ASC
LIMIT $1;
