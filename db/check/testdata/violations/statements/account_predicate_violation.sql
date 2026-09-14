-- Statements over the test library's tables, each breaking the account predicate check on purpose.

-- want account-predicate
-- name: ListEverySender :many
SELECT domain FROM fixture_senders WHERE message_count > @minimum;

-- A predicate under OR does not restrict the rows.
-- want account-predicate
-- name: ListSendersEitherWay :many
SELECT domain FROM fixture_senders WHERE account_id = @account_id OR message_count > @minimum;

-- The account column compared with something other than a parameter.
-- want account-predicate
-- name: ListSendersWithoutAccount :many
SELECT domain FROM fixture_senders WHERE account_id = domain::text;

-- want account-predicate
-- name: ResetEveryCount :execrows
UPDATE fixture_senders SET message_count = 0 WHERE domain = @domain;

-- want account-predicate
-- name: DeleteSender :execrows
DELETE FROM fixture_senders WHERE domain = @domain;

-- want account-predicate
-- name: InsertSenderWithoutAccount :exec
INSERT INTO fixture_senders (account_id, domain) VALUES ('fixed', @domain);

-- The second reference of a self-join is tied to nothing.
-- want account-predicate
-- name: ListSenderPairs :many
SELECT a.domain, b.domain
FROM fixture_senders a JOIN fixture_senders b ON a.message_count = b.message_count
WHERE a.account_id = @account_id;

-- The subquery's table is tied to the outer table only through OR, so it is reported.
-- want account-predicate
-- name: ListSendersAboveAnother :many
SELECT a.domain FROM fixture_senders a
WHERE a.account_id = @account_id
  AND a.message_count > (SELECT max(b.message_count) FROM fixture_senders b WHERE b.account_id = a.account_id OR b.domain = @domain);

-- Two references tied only to each other, with no parameter behind either, are both reported.
-- want account-predicate
-- want account-predicate
-- name: ListSenderPairsUntied :many
SELECT a.domain, b.domain
FROM fixture_senders a JOIN fixture_senders b ON a.account_id = b.account_id;

-- A predicate inside a subquery under OR does not restrict the outer table.
-- want account-predicate
-- name: ListSendersOrExists :many
SELECT a.domain FROM fixture_senders a
WHERE a.message_count > 0 OR EXISTS (SELECT 1 FROM fixture_log l WHERE a.account_id = @account_id AND l.seq > 0);

-- None of the following may be reported. A join tied through equal account columns, a subquery tied
-- to the outer query, an insert taking the account from a parameter, an insert from a query tied to
-- a parameter, and a table with no account column.
-- name: ListSenderPairsTied :many
SELECT a.domain, b.domain
FROM fixture_senders a JOIN fixture_senders b ON a.account_id = b.account_id AND a.message_count = b.message_count
WHERE a.account_id = @account_id;

-- name: ListSendersAboveAverage :many
SELECT a.domain FROM fixture_senders a
WHERE a.account_id = @account_id
  AND a.message_count > (SELECT avg(b.message_count) FROM fixture_senders b WHERE b.account_id = a.account_id);

-- name: InsertSender :exec
INSERT INTO fixture_senders (account_id, domain) VALUES (@account_id, @domain);

-- name: CopySender :exec
INSERT INTO fixture_senders (account_id, domain, message_count)
SELECT s.account_id, @new_domain, s.message_count FROM fixture_senders s WHERE s.account_id = @account_id AND s.domain = @domain;

-- name: ListLog :many
SELECT message FROM fixture_log ORDER BY seq;
