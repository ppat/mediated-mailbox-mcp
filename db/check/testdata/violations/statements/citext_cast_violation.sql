-- Statements over the test library's tables, each breaking the case-insensitive cast check on purpose.

-- want citext-parameter-cast
-- name: GetSenderByDomain :one
SELECT message_count FROM fixture_senders WHERE account_id = current_setting('app.account') AND domain = @domain::text;

-- want citext-parameter-cast
-- name: GetSenderByDomainAlias :one
SELECT s.message_count FROM fixture_senders s WHERE s.account_id = current_setting('app.account') AND sqlc.narg(domain)::text = s.domain;

-- want citext-parameter-cast
-- name: ListSendersInDomains :many
SELECT message_count FROM fixture_senders WHERE account_id = current_setting('app.account') AND domain IN ($2::text, $3::text);

-- A cast parameter compared against a column whose type cannot be resolved is refused, because the
-- column may be case-insensitive.
-- want citext-parameter-cast
-- name: GetSenderThroughCTE :one
WITH scoped AS (SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account'))
SELECT message_count FROM scoped WHERE domain = @domain::text;

-- A case-insensitive column passed through a subquery's output and compared with a cast parameter is
-- refused as the column itself is.
-- want citext-parameter-cast
-- name: GetSenderThroughSubquery :one
SELECT t.message_count
FROM (SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account')) AS t
WHERE t.domain = @domain::text;

-- The same comparison without a cast, a parameter cast to the case-insensitive type, and a cast compared
-- against a text column, must not be reported. The parameters cast to the case-insensitive type are
-- compared with the index's address column, since the domain check refuses that cast beside a domain.
-- name: ListMessagesFromAddressesCitext :many
SELECT message_id FROM messages WHERE account_id = current_setting('app.account') AND from_email = any(@addresses::citext[]);

-- name: GetMessageFromAddressCitext :one
SELECT message_id FROM messages WHERE account_id = current_setting('app.account') AND from_email = @address::citext;

-- name: GetSenderByDomainNoCast :one
SELECT message_count FROM fixture_senders WHERE account_id = current_setting('app.account') AND domain = @domain;

-- name: GetSenderByAccountCast :one
SELECT message_count FROM fixture_senders WHERE account_id = current_setting('app.account') AND domain = @domain;

-- A text column passed through a subquery's output and compared with a parameter cast to text must not
-- be reported, since the subquery's output keeps the column's type.
-- name: ListMessagesFromDomainThroughSubquery :many
SELECT t.message_id
FROM (SELECT message_id, from_domain FROM messages WHERE account_id = current_setting('app.account')) AS t
WHERE t.from_domain = @domain::text;
