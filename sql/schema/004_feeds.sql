-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

DROP TABLE IF EXISTS feeds CASCADE;

CREATE TABLE feeds (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS feeds CASCADE;
