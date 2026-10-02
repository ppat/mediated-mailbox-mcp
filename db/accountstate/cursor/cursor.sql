-- name: SyncCursor :one
-- The account's change cursor and when delta sync last wrote it, both null before its first tick
-- (ADR-0016). A gap's recovery starts its window from the write time (ADR-0105).
SELECT
    s.sync_cursor,
    s.sync_cursor_at
FROM account_state AS s
WHERE s.account_id = @account_id;

-- name: AdvanceCursor :execrows
-- Stores the account's change cursor and records the instant it was written, in the transaction that
-- made the changes it follows durable, so the cursor never runs ahead of the index (ADR-0018). An
-- account with no state row counts no row.
UPDATE account_state
SET sync_cursor = @sync_cursor, sync_cursor_at = now()
WHERE account_id = @account_id;
