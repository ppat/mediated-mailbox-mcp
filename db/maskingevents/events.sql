-- name: MaskingEventCount :one
-- The masking events recorded since a time, for the corpus endpoint (docs/UI.md section 8.1).
SELECT count(*) AS events
FROM masking_events AS e
WHERE e.account_id = current_setting('app.account') AND e.masked_at >= @since;
