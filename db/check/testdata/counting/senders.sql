-- name: AddSenderMessages :execrows
UPDATE fixture_senders
SET message_count = message_count + @added
WHERE account_id = @account_id AND domain = @domain;
