-- Selects every column of fixture_senders in table order, so sqlc reuses the table's struct as the
-- result type and declares it in models.go. The declares-no-types assertion must report it.
-- name: GetSender :one
SELECT account_id, domain, message_count
FROM fixture_senders
WHERE account_id = @account_id AND domain = @domain;
