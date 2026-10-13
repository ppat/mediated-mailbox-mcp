-- name: ListSenders :many
SELECT
    domain,
    message_count
FROM fixture_senders
WHERE account_id = current_setting('app.account')
ORDER BY message_count DESC, domain ASC
LIMIT @page_size;
