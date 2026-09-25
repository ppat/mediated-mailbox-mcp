-- +goose Up
-- The policy loader is a shared library, and its statements run under the role of each deployable that
-- imports it (ADR-0075). Backfill is the first. Its statement in db/policyrules reads the account, the
-- identifier, the class and the domain suffixes of the base rules and the account's own, so the role
-- gets SELECT on exactly those columns. Who created a rule and when stay out of its reach. Row-level
-- security still shows it only the base rules and its transaction's account's rules.
GRANT SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules TO mediated_mailbox_backfill;
