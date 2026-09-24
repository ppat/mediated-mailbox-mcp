-- +goose Up
-- The runtime roles' grants (ADR-0075). Each deployable's role gets only what its own statements
-- need. ADR-0021 states the UI's grant, reads on most tables with docs/UI.md naming which, its
-- decision columns and the policy-rule insert. Those are what the UI's statements need, so they are
-- granted with the schema. Every other role's grants arrive with its statements. Nothing automated
-- refuses a grant beyond what a role's statements need, so each migration that grants is reviewed
-- against the statements it serves.
--
-- No runtime role holds UPDATE, DELETE or TRUNCATE on audit_log (ADR-0016). A grant that writes it is
-- INSERT alone.
--
-- The migration role owns every table, so no runtime role can change the schema (ADR-0048).

-- The UI reads the tables its screens and endpoints read (docs/UI.md), which is every table. Its
-- write grant is the columns its two verbs set and the rule a confirmation inserts (ADR-0021).
GRANT SELECT ON
accounts,
rate_state,
senders,
messages,
scan_gate_decisions,
policy_candidates,
policy_rules,
masking_events,
reorg_plans,
reorg_plan_ops,
reorg_op_log,
job_runs,
job_run_events,
job_run_failures,
audit_log
TO mediated_mailbox_ui;
GRANT UPDATE (status, approved_at, approved_by) ON reorg_plans TO mediated_mailbox_ui;
GRANT UPDATE (status, reviewed_at, reviewed_by) ON policy_candidates TO mediated_mailbox_ui;
GRANT INSERT ON policy_rules TO mediated_mailbox_ui;
