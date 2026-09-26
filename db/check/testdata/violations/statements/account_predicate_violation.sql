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

-- The base policy rules alone, by a null account with no account parameter.
-- want account-predicate
-- name: ListBaseRulesOnly :many
SELECT rule_id FROM policy_rules WHERE account_id IS NULL;

-- Every policy rule, with no predicate at all.
-- want account-predicate
-- name: ListEveryRule :many
SELECT rule_id FROM policy_rules;

-- The account-or-null form on a table whose null-account rows no account inherits.
-- want account-predicate
-- name: ListSendersOrNull :many
SELECT domain FROM fixture_senders WHERE account_id = @account_id OR account_id IS NULL;

-- The null test on another column, so every account's rules with that column null are returned.
-- want account-predicate
-- name: ListRulesOrNullClass :many
SELECT rule_id FROM policy_rules WHERE account_id = @account_id::text OR class IS NULL;

-- The negated null test, so every account's rules are returned.
-- want account-predicate
-- name: ListRulesOrAnyAccount :many
SELECT rule_id FROM policy_rules WHERE account_id = @account_id::text OR account_id IS NOT NULL;

-- A third disjunct widens the account-or-null form.
-- want account-predicate
-- name: ListRulesOrNullOrClass :many
SELECT rule_id FROM policy_rules WHERE class = @class OR account_id = @account_id::text OR account_id IS NULL;

-- The account-or-null form in an update, which would change the base rules every account inherits.
-- want account-predicate
-- name: RenameRulesForAccount :execrows
UPDATE policy_rules SET class = @class WHERE account_id = @account_id::text OR account_id IS NULL;

-- The account-or-null form in a delete, which would remove the base rules every account inherits.
-- want account-predicate
-- name: DeleteRulesForAccount :execrows
DELETE FROM policy_rules WHERE account_id = @account_id::text OR account_id IS NULL;

-- The account-or-null form in the query an insert copies from, which would copy base rules. Both the
-- read and the inserted rows are reported.
-- want account-predicate
-- want account-predicate
-- name: CopyRulesForAccount :exec
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
SELECT r.account_id, @new_rule_id, r.class, r.domain_suffix, r.source, r.created_by
FROM policy_rules r WHERE r.account_id = @account_id::text OR r.account_id IS NULL;

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

-- The account-or-null form ADR-0047 states for the base policy rules, in either order and inside a
-- conjunction.
-- name: ListRulesForAccount :many
SELECT rule_id FROM policy_rules WHERE account_id = @account_id::text OR account_id IS NULL;

-- name: ListRulesForAccountReversed :many
SELECT rule_id FROM policy_rules WHERE account_id IS NULL OR @account_id::text = account_id;

-- name: ListRestrictedRulesForAccount :many
SELECT rule_id FROM policy_rules WHERE class = @class AND (account_id = @account_id::text OR account_id IS NULL);

-- The accounts table's exception covers its listing alone, so a write to it still needs the
-- account predicate.
-- want account-predicate
-- name: RenameEveryProvider :execrows
UPDATE accounts SET provider = @provider;

-- A statement named like the listing, outside the accounts subsection, is not the listing.
-- want account-predicate
-- name: Accounts :many
SELECT account_id FROM accounts;
