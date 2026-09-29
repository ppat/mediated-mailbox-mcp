-- name: MaskingEventPage :many
-- One page of the account's masking events, the latest first, masked at or after a time when one is
-- given, and after the position a cursor names or from the start when it names none. An event records
-- the rule and tier that fired and never the text it matched (ADR-0003).
SELECT
    e.id,
    e.message_id,
    e.field,
    e.rule_id,
    e.tier,
    e.masked_at
FROM masking_events AS e
WHERE
    e.account_id = @account_id
    AND (@since::timestamptz IS NULL OR e.masked_at >= @since::timestamptz)
    AND (
        @before_masked_at::timestamptz IS NULL
        OR (e.masked_at, e.id) < (@before_masked_at::timestamptz, @before_id::bigint)
    )
ORDER BY e.masked_at DESC, e.id DESC
LIMIT @page_size;
