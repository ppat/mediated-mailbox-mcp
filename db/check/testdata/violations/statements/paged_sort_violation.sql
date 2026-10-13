-- Statements over the test library's tables, each breaking the paged sort check on purpose. The row identity
-- of fixture_senders is domain, its primary key without the account column.

-- want paged-sort-identity
-- name: ListSendersByCount :many
SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account')
ORDER BY message_count DESC
LIMIT @page_size;

-- want paged-sort-identity
-- name: ListSendersIdentityFirst :many
SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account')
ORDER BY domain, message_count DESC
LIMIT @page_size;

-- want paged-sort-identity
-- name: ListSendersUnsorted :many
SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account')
LIMIT @page_size OFFSET @page_offset;

-- A grouped read over a subquery's output, its sort ending with an aggregate rather than the grouping
-- column the subquery passes through.
-- want paged-sort-identity
-- name: CountByDomainThroughSubqueryUnsorted :many
SELECT t.domain, sum(t.message_count) AS messages
FROM (SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account')) AS t
GROUP BY t.domain
ORDER BY t.domain, messages DESC
LIMIT @page_size;

-- Ending with the identity by ordinal, and a grouped read ending with its grouping column, must not
-- be reported. A read without a limit is not paged.
-- name: ListSendersOrdinal :many
SELECT message_count, domain FROM fixture_senders WHERE account_id = current_setting('app.account')
ORDER BY 1 DESC, 2
LIMIT @page_size;

-- name: CountByDomainPaged :many
SELECT domain, count(*) AS senders FROM fixture_senders WHERE account_id = current_setting('app.account')
GROUP BY domain
ORDER BY senders DESC, domain
LIMIT @page_size;

-- name: ListSendersAll :many
SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.account') ORDER BY message_count;

-- A grouped read over a subquery's output ending with the grouping column the subquery passes through
-- must not be reported.
-- name: CountByDomainThroughSubquery :many
SELECT t.domain, sum(t.message_count) AS messages
FROM (SELECT domain, message_count FROM fixture_senders WHERE account_id = current_setting('app.account')) AS t
GROUP BY t.domain
ORDER BY messages DESC, t.domain
LIMIT @page_size;
