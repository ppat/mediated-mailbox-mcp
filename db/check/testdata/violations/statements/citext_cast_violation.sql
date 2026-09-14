-- Statements over the test library's tables, each breaking the case-insensitive cast check on purpose.

-- want citext-parameter-cast
-- name: GetSenderByDomain :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain = @domain::text;

-- want citext-parameter-cast
-- name: GetSenderByDomainAlias :one
SELECT s.message_count FROM fixture_senders s WHERE s.account_id = @account_id AND sqlc.narg(domain)::text = s.domain;

-- want citext-parameter-cast
-- name: ListSendersInDomains :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain IN ($2::text, $3::text);

-- A cast parameter compared against a column whose type cannot be resolved is refused, because the
-- column may be case-insensitive.
-- want citext-parameter-cast
-- name: GetSenderThroughCTE :one
WITH scoped AS (SELECT domain, message_count FROM fixture_senders WHERE account_id = @account_id)
SELECT message_count FROM scoped WHERE domain = @domain::text;

-- The same comparison without a cast, and a cast compared against a text column, must not be
-- reported.
-- name: GetSenderByDomainNoCast :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain = @domain;

-- name: GetSenderByAccountCast :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id::text AND domain = @domain;
