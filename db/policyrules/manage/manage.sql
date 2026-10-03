-- name: ComposedRules :many
-- The rules the account's policy screen lists, the base rules every account inherits and the
-- account's own, with what the screen shows of each (docs/UI.md section 8.7). A base rule carries a
-- null account. The screen sorts, searches and pages them, and counts what each matches, in the UI's
-- server, which reads the account's senders beside them.
SELECT
    r.rule_id,
    r.account_id,
    r.class,
    r.domain_suffix,
    r.source,
    r.created_at,
    r.created_by
FROM policy_rules AS r
WHERE r.account_id = @account_id::text OR r.account_id IS NULL
ORDER BY r.rule_id, r.account_id NULLS FIRST;

-- name: AddRule :exec
-- Adds one of the account's own rules, which the operator's policy management writes, or a confirmed
-- candidate's (ADR-0004, ADR-0084). The table's key refuses an identifier the account already holds
-- (ADR-0110).
INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
VALUES (@account_id, @rule_id, @class, @domain_suffix, @source, @created_by);

-- name: EditRule :execrows
-- Sets one of the account's own rules' domain suffixes, only while they are still the suffixes the
-- edit was made against, so an edit over a change the operator never saw changes nothing.
UPDATE policy_rules
SET domain_suffix = @domain_suffix
WHERE account_id = @account_id AND rule_id = @rule_id AND domain_suffix = @suffixes_before::text[];

-- name: LiftRule :execrows
-- Removes one of the account's own rules, only while its suffixes are still the ones the lift was
-- confirmed against.
DELETE FROM policy_rules
WHERE account_id = @account_id AND rule_id = @rule_id AND domain_suffix = @suffixes_before::text[];
