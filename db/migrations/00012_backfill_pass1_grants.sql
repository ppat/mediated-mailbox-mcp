-- +goose Up
-- Backfill's first pass writes what its statements write (ADR-0075, ADR-0017). They sit in
-- db/messages/ingest, db/maskingevents/record, db/senders, db/jobruns/record and
-- db/accountstate/completion, which only backfill's list admits, and in db/accountstate, whose read of
-- the lowered target every role admitted there runs.
--
-- Pass 1 adds each message's metadata to the index and records the masks applied to its subject. It
-- rebuilds the statistics of each sender it saw from the messages the index holds, so it reads the
-- columns those statistics come from.
GRANT SELECT (
    account_id,
    message_id,
    from_email,
    from_domain,
    from_name,
    sent_at,
    labels,
    list_id,
    sender_class
),
INSERT (
    account_id,
    message_id,
    thread_id,
    from_email,
    from_domain,
    from_name,
    subject,
    subject_masked,
    sent_at,
    labels,
    flags,
    has_attachments,
    list_id,
    size_bytes,
    auth_results,
    sender_class
) ON messages TO mediated_mailbox_backfill;
GRANT INSERT (account_id, message_id, field, rule_id, tier) ON masking_events TO mediated_mailbox_backfill;
GRANT USAGE ON SEQUENCE masking_events_id_seq TO mediated_mailbox_backfill;
GRANT SELECT (
    account_id,
    domain,
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution,
    sender_class
),
INSERT (
    account_id,
    domain,
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution,
    sender_class
),
UPDATE (
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution,
    sender_class
) ON senders TO mediated_mailbox_backfill;

-- Every run is recorded with its checkpoint, counters, timeline and failed items (ADR-0022). A run
-- reads the latest run of its pass to resume from it.
GRANT SELECT (account_id, run_id, workload, pass, state, started_at, checkpoint, counters),
INSERT (
    account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters
),
UPDATE (state, finished_at, heartbeat_at, checkpoint, counters, last_error) ON job_runs
TO mediated_mailbox_backfill;
GRANT INSERT (account_id, run_id, kind, page, detail) ON job_run_events TO mediated_mailbox_backfill;
GRANT USAGE ON SEQUENCE job_run_events_seq_seq TO mediated_mailbox_backfill;
GRANT INSERT (
    account_id,
    run_id,
    item_kind,
    item_id,
    page,
    error_class,
    error_summary,
    attempts,
    first_at,
    last_at,
    disposition
) ON job_run_failures TO mediated_mailbox_backfill;
GRANT USAGE ON SEQUENCE job_run_failures_seq_seq TO mediated_mailbox_backfill;

-- Pass 1 reads its completion flag through the account's progress in db/accountstate, and sets it.
-- The lowered target is read by every role admitted to db/accountstate, which are backfill's, the
-- mediator's and the UI's. The UI also sets it at account setup, and the mediator will spend under it.
GRANT SELECT (
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
),
UPDATE (backfill_pass1_complete) ON account_state
TO mediated_mailbox_backfill;
GRANT SELECT (account_id, lowered_target_rate) ON account_state
TO mediated_mailbox_backfill, mediated_mailbox_mediate, mediated_mailbox_ui;
