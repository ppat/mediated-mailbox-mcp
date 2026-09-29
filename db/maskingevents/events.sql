-- name: MaskingEventCount :one
-- The masking events recorded since a time, for the system endpoint's corpus block (docs/UI.md
-- section 8.1).
SELECT count(*) AS events
FROM masking_events AS e
WHERE e.account_id = @account_id AND e.masked_at >= @since;
