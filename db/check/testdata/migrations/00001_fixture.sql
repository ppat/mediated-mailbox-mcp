-- +goose Up
-- The test library's schema, applied after the real migration chain. The checks in db/check read it
-- as a chain, sqlc generates the test library's subsections against it, and the grant test creates
-- it inside a transaction it rolls back. Its grant names the test library's one user literally, since
-- nothing replaces a placeholder in it.

-- Account-keyed, because it has an account_id column, with a case-insensitive column.
CREATE TABLE fixture_senders (
    account_id text NOT NULL,
    domain citext NOT NULL,
    message_count bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, domain)
);

-- Not account-keyed, because it has no account_id column.
CREATE TABLE fixture_log (
    seq bigserial PRIMARY KEY,
    message text NOT NULL
);

-- Reads alone, so a statement that updates the table is one its user cannot run.
GRANT SELECT ON fixture_senders TO check_fixture_user;
