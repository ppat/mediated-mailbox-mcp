-- name: ListSenders :many
SELECT domain, message_count
FROM fixture_senders
WHERE account_id = @account_id
ORDER BY message_count DESC, domain
LIMIT @page_size;
