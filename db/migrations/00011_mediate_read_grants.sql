-- +goose Up
-- The mediator's read operations and its system status read the index and the recorded state through
-- the subsections its list admits, db/messages, db/maskingevents, db/jobruns, db/accountstate and
-- db/ratestate. Each grant covers the columns the statements of those subsections read, under
-- ADR-0075's three lines. The mediator writes none of these tables, holds no grant on the sealed
-- credential, and holds none on the stored sender class, which a record forbids it to act on
-- (ADR-0002). The statement reading that class sits in db/messages/classification, which the
-- mediator's list does not admit. The mediator's role already reads rate_state, for the rate limiter.
GRANT SELECT (
    account_id,
    message_id,
    thread_id,
    from_email,
    from_name,
    subject,
    sent_at,
    labels,
    flags,
    has_attachments,
    attachment_types,
    content_flags,
    scan_state
) ON messages TO mediated_mailbox_mediate;
GRANT SELECT (id, account_id, message_id, field, rule_id, tier, masked_at) ON masking_events
TO mediated_mailbox_mediate;
GRANT SELECT (
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
) ON account_state TO mediated_mailbox_mediate;
GRANT SELECT (
    account_id,
    run_id,
    workload,
    pass,
    state,
    plan_id,
    resumed_from,
    started_at,
    finished_at,
    heartbeat_at,
    checkpoint,
    counters,
    last_error
) ON job_runs TO mediated_mailbox_mediate;
GRANT SELECT (account_id, run_id, kind, at, page) ON job_run_events TO mediated_mailbox_mediate;
GRANT SELECT (account_id, plan_id, description, status) ON reorg_plans TO mediated_mailbox_mediate;
