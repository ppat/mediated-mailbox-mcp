-- name: PolicyRules :many
-- The rules the account's policy is composed from, the base rules every account inherits and the
-- account's own, in one read (ADR-0004, ADR-0026). A base rule carries a null account, so the
-- account predicate is the account or null (ADR-0047). The account is null for a base rule.
SELECT
    account_id,
    rule_id,
    class,
    domain_suffix
FROM policy_rules
WHERE account_id = @account_id::text OR account_id IS NULL
ORDER BY rule_id;
