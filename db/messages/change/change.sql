-- name: ApplyLabelsAndFlags :execrows
-- Sets a stored message's labels and flags to the ones the provider reports, which delta sync applies
-- for a message the change feed names (ADR-0018). A message that already carries them is left as it
-- is and counts no row, so a change applied twice changes nothing the second time.
UPDATE messages
SET labels = @labels, flags = @flags
WHERE
    account_id = @account_id
    AND message_id = @message_id
    AND (labels IS DISTINCT FROM @labels OR flags IS DISTINCT FROM @flags);

-- name: RemoveMessages :many
-- Removes from the index each of the given messages the change feed reports the provider no longer
-- holds, so the index tracks the live mailbox (ADR-0018). It returns each removed message's sender
-- domain, whose statistics the caller rebuilds. A message the index does not hold is no row.
DELETE FROM messages
WHERE account_id = @account_id AND message_id = any(@message_ids::text[])
RETURNING from_domain;

-- name: DropEmptySender :execrows
-- Removes the statistics of the account's sender at domain once the index stores none of its
-- messages, after delta sync removed its last one, since rebuilding them from no messages writes
-- nothing (ADR-0016).
DELETE FROM senders AS s
WHERE
    s.account_id = @account_id
    AND s.domain = @domain
    AND NOT EXISTS (
        SELECT 1
        FROM messages AS m
        WHERE m.account_id = @account_id AND m.from_domain = @domain
    );

-- name: MessagesSince :many
-- The identifiers of the account's stored messages dated at or after since, which a gap recovery
-- compares with what its listing of the window returned, to find a message removed during the gap
-- (ADR-0105).
SELECT m.message_id
FROM messages AS m
WHERE m.account_id = @account_id AND m.sent_at >= @since
ORDER BY m.message_id;
