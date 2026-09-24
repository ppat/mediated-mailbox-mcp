-- +goose Up
-- Row-level security, the third isolation layer of ADR-0016, behind the account in every data-access
-- signature and the account predicate in every statement (ADR-0047). Every policy reads the
-- transaction-local setting app.account, which db/tx sets and reads back before any data access.
--
-- current_setting is called without its missing_ok argument. On a connection that never set the
-- account a statement raises, and on one that set it in an earlier transaction the setting reads as
-- the empty string and the statement returns no rows. db/tx fails the transaction in both cases.
--
-- The policies apply to every role but the tables' owner, the migration role. Each also checks the
-- rows a statement writes, so a row written for another account is refused.

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY accounts_account ON accounts
USING (account_id = current_setting('app.account'));

ALTER TABLE rate_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY rate_state_account ON rate_state
USING (account_id = current_setting('app.account'));

ALTER TABLE senders ENABLE ROW LEVEL SECURITY;
CREATE POLICY senders_account ON senders
USING (account_id = current_setting('app.account'));

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY messages_account ON messages
USING (account_id = current_setting('app.account'));

ALTER TABLE scan_gate_decisions ENABLE ROW LEVEL SECURITY;
CREATE POLICY scan_gate_decisions_account ON scan_gate_decisions
USING (account_id = current_setting('app.account'));

ALTER TABLE policy_candidates ENABLE ROW LEVEL SECURITY;
CREATE POLICY policy_candidates_account ON policy_candidates
USING (account_id = current_setting('app.account'));

-- A rule with a null account is the base policy every account inherits (ADR-0004), the first of
-- ADR-0016's two exceptions. Every account reads it, and no runtime role writes it, so a write names
-- the transaction's own account. The UI's insert is the one rule a confirmation of its account's
-- candidate emits (ADR-0021). What writes base rules is an open decision whose migration adds it.
ALTER TABLE policy_rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY policy_rules_account ON policy_rules
USING (account_id = current_setting('app.account'));
CREATE POLICY policy_rules_base ON policy_rules FOR SELECT
USING (account_id IS NULL);

ALTER TABLE masking_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY masking_events_account ON masking_events
USING (account_id = current_setting('app.account'));

ALTER TABLE reorg_plans ENABLE ROW LEVEL SECURITY;
CREATE POLICY reorg_plans_account ON reorg_plans
USING (account_id = current_setting('app.account'));

ALTER TABLE reorg_plan_ops ENABLE ROW LEVEL SECURITY;
CREATE POLICY reorg_plan_ops_account ON reorg_plan_ops
USING (account_id = current_setting('app.account'));

-- The operation log carries no account column, ADR-0016's second exception, so it is scoped through
-- its plan. The subquery does not refer to the row, so it runs once per statement rather than once
-- per row. It runs with the querying role's privileges, so a role reading or writing the log also
-- needs to read reorg_plans(plan_id, account_id) (ADR-0075), and reorg_plans' own policy applies to
-- it as well.
ALTER TABLE reorg_op_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY reorg_op_log_plan ON reorg_op_log
USING (
    plan_id IN (
        SELECT reorg_plans.plan_id FROM reorg_plans
        WHERE reorg_plans.account_id = current_setting('app.account')
    )
);

ALTER TABLE job_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY job_runs_account ON job_runs
USING (account_id = current_setting('app.account'));

ALTER TABLE job_run_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY job_run_events_account ON job_run_events
USING (account_id = current_setting('app.account'));

ALTER TABLE job_run_failures ENABLE ROW LEVEL SECURITY;
CREATE POLICY job_run_failures_account ON job_run_failures
USING (account_id = current_setting('app.account'));

ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_log_account ON audit_log
USING (account_id = current_setting('app.account'));
