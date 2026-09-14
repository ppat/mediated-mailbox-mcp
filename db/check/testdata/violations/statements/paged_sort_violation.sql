-- Statements over the test library's tables, each breaking the paged sort check on purpose. The row identity
-- of fixture_senders is domain, its primary key without the account column.

-- want paged-sort-identity
-- name: ListSendersByCount :many
SELECT domain, message_count FROM fixture_senders WHERE account_id = @account_id
ORDER BY message_count DESC
LIMIT @page_size;

-- want paged-sort-identity
-- name: ListSendersIdentityFirst :many
SELECT domain, message_count FROM fixture_senders WHERE account_id = @account_id
ORDER BY domain, message_count DESC
LIMIT @page_size;

-- want paged-sort-identity
-- name: ListSendersUnsorted :many
SELECT domain, message_count FROM fixture_senders WHERE account_id = @account_id
LIMIT @page_size OFFSET @page_offset;

-- Ending with the identity by ordinal, and a grouped read ending with its grouping column, must not
-- be reported. A read without a limit is not paged.
-- name: ListSendersOrdinal :many
SELECT message_count, domain FROM fixture_senders WHERE account_id = @account_id
ORDER BY 1 DESC, 2
LIMIT @page_size;

-- name: CountByDomainPaged :many
SELECT domain, count(*) AS senders FROM fixture_senders WHERE account_id = @account_id
GROUP BY domain
ORDER BY senders DESC, domain
LIMIT @page_size;

-- name: ListSendersAll :many
SELECT domain FROM fixture_senders WHERE account_id = @account_id ORDER BY message_count;
