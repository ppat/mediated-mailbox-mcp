-- name: AddSenderMessages :execrows
UPDATE fixture_senders
SET message_count = message_count + @added
WHERE account_id = current_setting('app.account') AND domain = @domain;
