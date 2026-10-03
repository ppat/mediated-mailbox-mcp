-- Statements in a directory named as the base policy's subsection, each breaking the account
-- predicate check there on purpose. A base statement reaches the base policy's rows alone, by the null
-- account, and never an account's (ADR-0112).

-- An account parameter scopes nothing in a base statement.
-- want account-predicate
-- name: ListAccountRules :many
SELECT rule_id FROM policy_rules WHERE account_id = @account_id::text;

-- The account-or-null form reaches an account's rules.
-- want account-predicate
-- name: ListRulesForAccount :many
SELECT rule_id FROM policy_rules WHERE account_id = @account_id::text OR account_id IS NULL;

-- A null test on a table whose null-account rows no account inherits scopes nothing.
-- want account-predicate
-- name: ListUnownedSenders :many
SELECT domain FROM fixture_senders WHERE account_id IS NULL;

-- An insert taking the account from a parameter could write an account's rule.
-- want account-predicate
-- name: AddRuleForAccount :exec
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
VALUES (@account_id, @rule_id, 'restricted', @domain_suffix, 'operator', @created_by);

-- An update tied to an account parameter.
-- want account-predicate
-- name: EditAccountRule :execrows
UPDATE policy_rules SET domain_suffix = @domain_suffix WHERE account_id = @account_id::text AND rule_id = @rule_id;

-- A delete with no scope at all.
-- want account-predicate
-- name: LiftEveryRule :execrows
DELETE FROM policy_rules WHERE rule_id = @rule_id;

-- None of the following may be reported. The base policy's own statements, scoped by the null account.
-- name: BaseRules :many
SELECT rule_id, domain_suffix FROM policy_rules WHERE account_id IS NULL ORDER BY rule_id;

-- name: AddBaseRule :exec
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
VALUES (NULL, @rule_id, 'restricted', @domain_suffix, 'operator', @created_by);

-- name: EditBaseRule :execrows
UPDATE policy_rules SET domain_suffix = @domain_suffix WHERE account_id IS NULL AND rule_id = @rule_id;

-- name: LiftBaseRule :execrows
DELETE FROM policy_rules WHERE account_id IS NULL AND rule_id = @rule_id;

-- name: RecordBaseChange :exec
INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES (NULL, @actor, @action, @rule_id);
