-- Statements over the test library's tables, each folding a sender domain's case in SQL on purpose.

-- A domain column lowered.
-- want domain-sql-case
-- name: GetSenderByLoweredDomain :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND lower(domain::text) = @domain::text;

-- A parameter lowered to match a domain column.
-- want domain-sql-case
-- name: GetSenderByLoweredParameter :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain = lower(@domain);

-- A domain column searched for a parameter folded to upper case, through a function call.
-- want domain-sql-case
-- name: SearchSendersUpper :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND strpos(domain::text, upper(@search::text)) > 0;

-- A domain column lowered as a grouping key, read through a common table expression, once in its output
-- and once in its grouping.
-- want domain-sql-case
-- want domain-sql-case
-- name: CountByLoweredDomain :many
WITH scoped AS (SELECT s.domain, s.message_count FROM fixture_senders AS s WHERE s.account_id = @account_id)
SELECT lower(scoped.domain::text) AS sender, sum(scoped.message_count) AS messages FROM scoped GROUP BY lower(scoped.domain::text);

-- A domain column case-folded and capitalized.
-- want domain-sql-case
-- name: GetSenderByCasefoldedDomain :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND casefold(domain) = @domain;

-- want domain-sql-case
-- name: GetSenderByCapitalizedParameter :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain = initcap(@domain);

-- A domain column matched without regard to case, by ILIKE, NOT ILIKE and the case-insensitive
-- regular expression matches.
-- want domain-sql-case
-- name: SearchSendersILike :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain ILIKE @pattern;

-- want domain-sql-case
-- name: SearchSendersNotILike :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain NOT ILIKE @pattern;

-- want domain-sql-case
-- name: SearchSendersRegexCase :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain ~* @pattern;

-- want domain-sql-case
-- name: SearchSendersNotRegexCase :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain !~* @pattern;

-- A domain compared as stored, a parameter named domain, a domain matched with case by LIKE, and a
-- column of another name lowered or matched without case, must not be reported.
-- name: GetSenderByStoredDomain :one
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain = @domain;

-- name: SearchSendersStored :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND strpos(domain::text, @search::text) > 0;

-- name: SearchSendersLike :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain LIKE @pattern;

-- name: SearchAccountsILike :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND account_id ILIKE 'per%';

-- name: LoweredAccount :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND lower(account_id) = 'personal';
