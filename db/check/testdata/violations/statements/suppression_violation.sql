-- A statement over the test library's tables carrying the generator's suppression annotation on purpose.

-- want suppression-annotation
-- name: CountSendersSuppressed :one
-- @sqlc-vet-disable
SELECT count(*) FROM fixture_senders WHERE account_id = @account_id;
