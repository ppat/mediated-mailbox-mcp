-- +goose Up
-- A migration after the baseline that declares neither that every chain the floor admits runs on it
-- nor a floor. A line close to the marker is not the marker.
-- compatibility: the previous chain runs on this migration

-- want compatibility-undeclared
CREATE INDEX ON fixture_senders (account_id, message_count);
