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
-- rows a statement writes, so a row written for another account is refused. oauth_clients carries no
-- account column and has no policy, so its grants are its whole database barrier (ADR-0016).

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY accounts_account ON accounts
USING (account_id = current_setting('app.account'));
-- The roles that list accounts read all of its rows, one of ADR-0016's exceptions, so a listing
-- needs no account set (ADR-0091). They are the UI's, for its account selector, and the mediator's,
-- backfill's, delta sync's and the reorg workload's, for their account snapshots. Any other role that
-- reads accounts sees only its transaction's account. accounts_account still confines every write
-- to the transaction's account, since a permissive policy for SELECT alone takes no part in an
-- insert, an update's new row or a delete.
CREATE POLICY accounts_listing ON accounts FOR SELECT
TO mediated_mailbox_ui,
mediated_mailbox_mediate,
mediated_mailbox_backfill,
mediated_mailbox_sync,
mediated_mailbox_organize
USING (TRUE);

ALTER TABLE account_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY account_state_account ON account_state
USING (account_id = current_setting('app.account'));

ALTER TABLE rate_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY rate_state_account ON rate_state
USING (account_id = current_setting('app.account'));

ALTER TABLE rate_grants ENABLE ROW LEVEL SECURITY;
CREATE POLICY rate_grants_account ON rate_grants
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

-- A rule with a null account is the base policy every account inherits (ADR-0004), and a history row
-- with a null account is a change to it (ADR-0102). Every account reads both, and an account's
-- transaction writes only its own account's rows.
ALTER TABLE policy_rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY policy_rules_account ON policy_rules
USING (account_id = current_setting('app.account'));
CREATE POLICY policy_rules_base ON policy_rules FOR SELECT
USING (account_id IS NULL);

ALTER TABLE policy_changes ENABLE ROW LEVEL SECURITY;
CREATE POLICY policy_changes_account ON policy_changes
USING (account_id = current_setting('app.account'));
CREATE POLICY policy_changes_base ON policy_changes FOR SELECT
USING (account_id IS NULL);

-- The base policy is written only by the UI's role, and only in a base-policy transaction, which sets
-- app.base to on and the account to the empty string (ADR-0112). The transaction helper's second entry
-- point sets and reads back both. An account's transaction sets an account, so it writes no base row,
-- and a base-policy transaction's empty account matches no account's row, so it reads and writes none.
-- Neither setting is known on a connection that never set it, so both are read with missing_ok.
CREATE POLICY policy_rules_base_write ON policy_rules
TO mediated_mailbox_ui
USING (
    account_id IS NULL
    AND current_setting('app.base', TRUE) = 'on'
    AND coalesce(current_setting('app.account', TRUE), '') = ''
)
WITH CHECK (
    account_id IS NULL
    AND current_setting('app.base', TRUE) = 'on'
    AND coalesce(current_setting('app.account', TRUE), '') = ''
);
CREATE POLICY policy_changes_base_write ON policy_changes FOR INSERT
TO mediated_mailbox_ui
WITH CHECK (
    account_id IS NULL
    AND current_setting('app.base', TRUE) = 'on'
    AND coalesce(current_setting('app.account', TRUE), '') = ''
);

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
