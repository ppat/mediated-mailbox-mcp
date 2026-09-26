-- +goose Up
-- The mediator loads policy through the shared policy loader, whose statement in db/policyrules runs
-- under the role of each deployable that imports it (ADR-0075). The mediator's role gets SELECT on the
-- same columns backfill's does, and who created a rule and when stay out of its reach. Row-level
-- security still shows it only the base rules and its transaction's account's rules.
GRANT SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules TO mediated_mailbox_mediate;
