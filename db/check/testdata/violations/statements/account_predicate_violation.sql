-- Statements over the test library's tables, each breaking the account predicate check on purpose.

-- want account-predicate
-- name: ListEverySender :many
SELECT domain FROM fixture_senders WHERE message_count > @minimum;

-- A predicate under OR does not restrict the rows.
-- want account-predicate
-- name: ListSendersEitherWay :many
SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.account') OR message_count > @minimum;

-- The account column compared with something other than the transaction's account.
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
WHERE a.account_id = current_setting('app.account');

-- The subquery's table is tied to the outer table only through OR, so it is reported.
-- want account-predicate
-- name: ListSendersAboveAnother :many
SELECT a.domain FROM fixture_senders a
WHERE a.account_id = current_setting('app.account')
  AND a.message_count > (SELECT max(b.message_count) FROM fixture_senders b WHERE b.account_id = a.account_id OR b.domain = @domain);

-- Two references tied only to each other, with the transaction's account behind neither, are both
-- reported.
-- want account-predicate
-- want account-predicate
-- name: ListSenderPairsUntied :many
SELECT a.domain, b.domain
FROM fixture_senders a JOIN fixture_senders b ON a.account_id = b.account_id;

-- A predicate inside a subquery under OR does not restrict the outer table.
-- want account-predicate
-- name: ListSendersOrExists :many
SELECT a.domain FROM fixture_senders a
WHERE a.message_count > 0 OR EXISTS (SELECT 1 FROM fixture_log l WHERE a.account_id = current_setting('app.account') AND l.seq > 0);

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
SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.account') OR account_id IS NULL;

-- The null test on another column, so every account's rules with that column null are returned.
-- want account-predicate
-- name: ListRulesOrNullClass :many
SELECT rule_id FROM policy_rules WHERE account_id = current_setting('app.account') OR class IS NULL;

-- The negated null test, so every account's rules are returned.
-- want account-predicate
-- name: ListRulesOrAnyAccount :many
SELECT rule_id FROM policy_rules WHERE account_id = current_setting('app.account') OR account_id IS NOT NULL;

-- A third disjunct widens the account-or-null form.
-- want account-predicate
-- name: ListRulesOrNullOrClass :many
SELECT rule_id FROM policy_rules WHERE class = @class OR account_id = current_setting('app.account') OR account_id IS NULL;

-- The account-or-null form in an update, which would change the base rules every account inherits.
-- want account-predicate
-- name: RenameRulesForAccount :execrows
UPDATE policy_rules SET class = @class WHERE account_id = current_setting('app.account') OR account_id IS NULL;

-- The account-or-null form in a delete, which would remove the base rules every account inherits.
-- want account-predicate
-- name: DeleteRulesForAccount :execrows
DELETE FROM policy_rules WHERE account_id = current_setting('app.account') OR account_id IS NULL;

-- The account-or-null form in the query an insert copies from, which would copy base rules. Both the
-- read and the inserted rows are reported.
-- want account-predicate
-- want account-predicate
-- name: CopyRulesForAccount :exec
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
SELECT r.account_id, @new_rule_id, r.class, r.domain_suffix, r.source, r.created_by
FROM policy_rules r WHERE r.account_id = current_setting('app.account') OR r.account_id IS NULL;

-- An account parameter in place of the transaction's account, which could name another account. The
-- reference is tied to nothing, and the comparison with a parameter is refused on its own.
-- want account-parameter
-- want account-predicate
-- name: ListSendersForParameter :many
SELECT domain FROM fixture_senders WHERE account_id = @account_id;

-- A parameter beside the transaction's account still names an account, so it is refused though the
-- reference is tied.
-- want account-parameter
-- name: ListSendersForBoth :many
SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.account') AND account_id = @account_id::text;

-- An account parameter among a list.
-- want account-parameter
-- want account-predicate
-- name: ListSendersForAny :many
SELECT domain FROM fixture_senders WHERE account_id = ANY(@accounts::text[]);

-- An insert taking the account from a parameter.
-- want account-predicate
-- name: InsertSenderForParameter :exec
INSERT INTO fixture_senders (account_id, domain) VALUES (@account_id, @domain);

-- An update moving a row to an account a parameter names.
-- want account-update
-- name: MoveSender :execrows
UPDATE fixture_senders SET account_id = @account_id WHERE account_id = current_setting('app.account') AND domain = @domain;

-- An update moving a row to an account a literal names.
-- want account-update
-- name: MoveSenderToLiteral :execrows
UPDATE senders SET account_id = 'acct-b' WHERE account_id = current_setting('app.account') AND domain = @domain;

-- An update moving a row to the account another column holds.
-- want account-update
-- name: MoveSenderToColumn :execrows
UPDATE fixture_senders SET account_id = domain::text WHERE account_id = current_setting('app.account');

-- An update setting the account to the transaction's own still sets the column, which no statement does.
-- want account-update
-- name: RewriteSenderAccount :execrows
UPDATE fixture_senders SET account_id = current_setting('app.account') WHERE account_id = current_setting('app.account');

-- An update moving an account's own rule into the base policy every account inherits.
-- want account-update
-- name: MoveRuleIntoBase :execrows
UPDATE policy_rules SET account_id = NULL WHERE account_id = current_setting('app.account') AND rule_id = @rule_id;

-- The same move of a history row.
-- want account-update
-- name: MoveChangeIntoBase :execrows
UPDATE policy_changes SET account_id = NULL WHERE account_id = current_setting('app.account') AND rule_id = @rule_id;

-- An upsert whose update moves the row to another account.
-- want account-update
-- name: UpsertSenderElsewhere :exec
INSERT INTO fixture_senders (account_id, domain) VALUES (current_setting('app.account'), @domain)
ON CONFLICT (account_id, domain) DO UPDATE SET account_id = 'acct-b';

-- An upsert whose update moves an account's rule into the base policy.
-- want account-update
-- name: UpsertRuleIntoBase :exec
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
VALUES (current_setting('app.account'), @rule_id, 'restricted', @domain_suffix, 'operator', @created_by)
ON CONFLICT (account_id, rule_id) DO UPDATE SET account_id = NULL;

-- Moving the accounts table's own row.
-- want account-update
-- name: RenameAccount :execrows
UPDATE accounts SET account_id = 'acct-b' WHERE account_id = current_setting('app.account');

-- The setting read with its missing_ok argument, which reads no account as null rather than raising,
-- is not the transaction's account as the helper reads it back.
-- want account-predicate
-- name: ListSendersMissingOk :many
SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.account', true);

-- Another setting is not the transaction's account.
-- want account-predicate
-- name: ListSendersOtherSetting :many
SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.other');

-- The operation log reached with no join to its plan.
-- want account-predicate
-- name: OpLogByPlan :many
SELECT message_id FROM reorg_op_log WHERE plan_id = @plan_id;

-- The operation log joined to a plan that is tied to nothing.
-- want account-predicate
-- want account-predicate
-- name: OpLogUntiedPlan :many
SELECT l.message_id FROM reorg_op_log AS l INNER JOIN reorg_plans AS p ON l.plan_id = p.plan_id WHERE l.plan_id = @plan_id;

-- The operation log joined to a tied plan on another column, which ties nothing.
-- want account-predicate
-- name: OpLogWrongJoin :many
SELECT l.message_id FROM reorg_op_log AS l INNER JOIN reorg_plans AS p ON l.message_id = p.description
WHERE p.account_id = current_setting('app.account');

-- An insert into the operation log naming its plan by a parameter.
-- want account-predicate
-- name: AppendOpByParameter :exec
INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after) VALUES (@plan_id, @message_id, '{}', '{}');

-- An insert into the operation log copying its plan from an untied plan.
-- want account-predicate
-- want account-predicate
-- name: AppendOpUntied :exec
INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after)
SELECT p.plan_id, @message_id, '{}', '{}' FROM reorg_plans AS p WHERE p.plan_id = @plan_id;

-- None of the following may be reported. A join tied through equal account columns, a subquery tied
-- to the outer query, an insert taking the account from the transaction, an insert from a query tied
-- to the transaction's account, and a table with no account column.
-- name: ListSenderPairsTied :many
SELECT a.domain, b.domain
FROM fixture_senders a JOIN fixture_senders b ON a.account_id = b.account_id AND a.message_count = b.message_count
WHERE a.account_id = current_setting('app.account');

-- name: ListSendersAboveAverage :many
SELECT a.domain FROM fixture_senders a
WHERE a.account_id = current_setting('app.account')
  AND a.message_count > (SELECT avg(b.message_count) FROM fixture_senders b WHERE b.account_id = a.account_id);

-- name: InsertSender :exec
INSERT INTO fixture_senders (account_id, domain) VALUES (current_setting('app.account'), @domain);

-- name: CopySender :exec
INSERT INTO fixture_senders (account_id, domain, message_count)
SELECT s.account_id, @new_domain, s.message_count FROM fixture_senders s WHERE s.account_id = current_setting('app.account') AND s.domain = @domain;

-- name: ListLog :many
SELECT message FROM fixture_log ORDER BY seq;

-- The account-or-null form ADR-0135 states for the base policy rules, in either order and inside a
-- conjunction.
-- name: ListRulesForAccount :many
SELECT rule_id FROM policy_rules WHERE account_id = current_setting('app.account') OR account_id IS NULL;

-- name: ListRulesForAccountReversed :many
SELECT rule_id FROM policy_rules WHERE account_id IS NULL OR current_setting('app.account') = account_id;

-- name: ListRestrictedRulesForAccount :many
SELECT rule_id FROM policy_rules WHERE class = @class AND (account_id = current_setting('app.account') OR account_id IS NULL);

-- The operation log through a plan tied to the transaction's account, and an insert taking its plan
-- from such a row.
-- name: OpLogThroughPlan :many
SELECT l.message_id FROM reorg_op_log AS l INNER JOIN reorg_plans AS p ON l.plan_id = p.plan_id
WHERE p.account_id = current_setting('app.account') AND l.plan_id = @plan_id;

-- name: AppendOpThroughPlan :exec
INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after)
SELECT p.plan_id, @message_id, '{}', '{}' FROM reorg_plans AS p
WHERE p.account_id = current_setting('app.account') AND p.plan_id = @plan_id;

-- The accounts table's exception covers its listing alone, so a write to it still needs the
-- account predicate.
-- want account-predicate
-- name: RenameEveryProvider :execrows
UPDATE accounts SET provider = @provider;

-- A statement named like the listing, outside the accounts subsection, is not the listing.
-- want account-predicate
-- name: Accounts :many
SELECT account_id FROM accounts;

-- A statement kind the check does not read is refused whatever it holds, so none reaches an
-- account-keyed table unexamined.

-- A merge with no account tie at all.
-- want statement-kind
-- name: MergeSendersUntied :exec
MERGE INTO senders AS s USING senders AS o ON s.domain = o.domain
WHEN MATCHED THEN UPDATE SET message_count = 0;

-- A merge inserting the account a parameter names.
-- want statement-kind
-- name: MergeSenderFromParameter :exec
MERGE INTO fixture_senders AS s USING (SELECT @domain::text AS d) AS v ON s.domain = v.d
WHEN NOT MATCHED THEN INSERT (account_id, domain) VALUES (@account_id, v.d);

-- A merge moving a row to another account.
-- want statement-kind
-- name: MergeSenderElsewhere :exec
MERGE INTO fixture_senders AS s USING (SELECT @domain::text AS d) AS v
ON s.account_id = current_setting('app.account') AND s.domain = v.d
WHEN MATCHED THEN UPDATE SET account_id = 'acct-b';

-- A truncation of an account-keyed table.
-- want statement-kind
-- name: TruncateSenders :exec
TRUNCATE senders;

-- A sampled table is the table itself, so its reference needs the tie.
-- want account-predicate
-- name: SampleSenders :many
SELECT domain FROM senders TABLESAMPLE SYSTEM (10);

-- A query inside a row function's arguments is read like any other.
-- want account-predicate
-- name: UnnestEveryDomain :many
SELECT d FROM unnest((SELECT array_agg(s.domain) FROM senders AS s)) AS d;

-- An insert into a table with no account column has its source read like any other query.
-- want account-predicate
-- name: CopyDomainsToLog :exec
INSERT INTO fixture_log (message) SELECT domain FROM fixture_senders;

-- None of the following may be reported: the same three forms tied to the transaction's account.
-- name: SampleOwnSenders :many
SELECT domain FROM senders TABLESAMPLE SYSTEM (10) WHERE account_id = current_setting('app.account');

-- name: UnnestOwnDomains :many
SELECT d FROM unnest((SELECT array_agg(s.domain) FROM senders AS s WHERE s.account_id = current_setting('app.account'))) AS d;

-- name: CopyOwnDomainsToLog :exec
INSERT INTO fixture_log (message) SELECT domain FROM fixture_senders WHERE account_id = current_setting('app.account');

-- A table in a clause the check does not read is refused, whatever the clause, so a clause it never
-- learned cannot carry an untied table through.
-- want unread-position
-- name: SendersLimitedByMessages :many
SELECT domain FROM senders WHERE account_id = current_setting('app.account')
ORDER BY domain LIMIT (SELECT count(*) FROM messages);

-- want unread-position
-- name: SendersDistinctOnMessages :many
SELECT DISTINCT ON ((SELECT max(m.from_domain) FROM messages AS m)) domain FROM senders
WHERE account_id = current_setting('app.account');

-- want unread-position
-- name: SendersUnionSortedByMessages :many
SELECT domain FROM senders WHERE account_id = current_setting('app.account')
UNION SELECT domain FROM senders WHERE account_id = current_setting('app.account')
ORDER BY (SELECT count(*) FROM messages);

-- want unread-position
-- name: InsertSendersWhileFewMessages :exec
INSERT INTO fixture_senders (account_id, domain)
SELECT current_setting('app.account'), d FROM unnest(@ds::text[]) AS u (d)
GROUP BY d HAVING count(*) < (SELECT count(*) FROM messages);

-- A common table expression named like a table does not hide the real table in a clause the check
-- does not read.
-- want unread-position
-- name: SendersLimitedBehindShadow :many
SELECT domain FROM senders
WHERE account_id = current_setting('app.account')
  AND EXISTS (WITH messages AS (SELECT 1 AS one) SELECT one FROM messages)
ORDER BY domain
LIMIT (SELECT count(*) FROM messages);
