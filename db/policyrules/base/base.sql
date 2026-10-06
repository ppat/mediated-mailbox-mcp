-- name: BaseRules :many
-- The base policy's rules, with what the base policy screen shows of each (docs/UI.md section 8.14).
-- Run only in a base-policy transaction, which names no account (ADR-0112).
SELECT
    r.rule_id,
    r.class,
    r.domain_suffix,
    r.source,
    r.created_at,
    r.created_by
FROM policy_rules AS r
WHERE r.account_id IS NULL
ORDER BY r.rule_id;

-- name: AddBaseRule :exec
-- Adds a base rule, which every account inherits (ADR-0004). The table's key refuses an identifier
-- the base policy already holds (ADR-0110).
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
VALUES (NULL, @rule_id, @class, @domain_suffix, @source, @created_by);

-- name: EditBaseRule :execrows
-- Sets a base rule's domain suffixes, only while they are still the suffixes the edit was made
-- against.
UPDATE policy_rules
SET domain_suffix = @domain_suffix
WHERE account_id IS NULL AND rule_id = @rule_id AND domain_suffix = @suffixes_before::text[];

-- name: LiftBaseRule :execrows
-- Removes a base rule, only while its suffixes are still the ones the lift was confirmed against.
DELETE FROM policy_rules
WHERE account_id IS NULL AND rule_id = @rule_id AND domain_suffix = @suffixes_before::text[];

-- name: RecordBaseChange :exec
-- Appends one change to a base rule to the policy history, in the transaction that made it (ADR-0102).
INSERT INTO policy_changes (account_id, actor, action, rule_id, suffixes_before, suffixes_after)
VALUES (NULL, @actor, @action, @rule_id, @suffixes_before, @suffixes_after);

-- name: BaseHistory :many
-- The base policy's changes, newest first, under a range and narrowed to one rule's when a rule is
-- given (docs/UI.md sections 8.14 and 17.4). A null bound is no bound, and an empty rule every rule.
SELECT
    c.id,
    c.ts,
    c.actor,
    c.action,
    c.rule_id,
    c.suffixes_before,
    c.suffixes_after
FROM policy_changes AS c
WHERE
    c.account_id IS NULL
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@rule_id::text = '' OR c.rule_id = @rule_id::text)
ORDER BY c.ts DESC, c.id DESC;

-- name: BaseLatestChange :one
-- When the base policy last changed, null before its first change, for the base policy screen's L0
-- strip.
SELECT max(c.ts)::timestamptz AS latest
FROM policy_changes AS c
WHERE c.account_id IS NULL;
