-- Statements over the test library's tables, each breaking the star-select check on purpose.

-- want star-select
-- name: ListSendersStar :many
SELECT * FROM fixture_senders WHERE account_id = @account_id;

-- want star-select
-- name: ListSendersQualifiedStar :many
SELECT s.* FROM fixture_senders s WHERE s.account_id = @account_id;

-- A count over every row is not a star select, so it must not be reported.
-- name: CountSenders :one
SELECT count(*) FROM fixture_senders WHERE account_id = @account_id;
