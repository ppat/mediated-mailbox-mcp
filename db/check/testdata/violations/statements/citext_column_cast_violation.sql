-- Statements over the test library's tables, each casting a case-insensitive column to text on purpose.
-- They sit apart from the parameter casts, so a check that stopped refusing these and started refusing
-- a cast to the case-insensitive type could not balance its findings within one file.

-- The case-insensitive column cast to text and compared with a parameter, cast or not, is refused,
-- because the comparison becomes case-sensitive just the same.
-- want citext-parameter-cast
-- name: ListSendersInDomainsColumnCast :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain::text = any(@domains::text[]);

-- want citext-parameter-cast
-- name: ListSendersOutsideDomainsColumnCast :many
SELECT message_count FROM fixture_senders WHERE account_id = @account_id AND domain::text != all(@domains);
