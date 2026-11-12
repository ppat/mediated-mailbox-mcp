-- +goose Up
-- A migration whose file name does not start with its version, so its declaration cannot be read
-- against the chain, though it holds the marker.
-- compatibility: every chain the floor admits runs on this migration

-- want compatibility-undeclared
CREATE INDEX ON fixture_senders (message_count);
