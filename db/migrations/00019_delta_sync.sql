-- +goose Up
-- Delta sync writes what its statements write (ADR-0075, ADR-0018). They sit in the subsections its
-- list admits. db/messages/ingest, db/maskingevents/record, db/senders, db/jobruns/record,
-- db/messages/scan, db/scangatedecisions/record and db/accountstate/authentication it shares with
-- backfill, since a tick adds and masks mail as backfill's first pass does and decides and scans what
-- waits as its second pass does (ADR-0104). db/messages/change, db/accountstate/cursor and
-- db/oauthclients/secret only its list admits. db/accountstate and db/policyrules it reads.
--
-- A tick adds each message's metadata to the index, masks its subject and records the masks, and
-- rebuilds the statistics of each sender it saw from the messages the index holds. It sets a stored
-- message's labels and flags to the ones the provider reports, removes a message the provider no
-- longer holds, and removes a sender's statistics once none of its messages is stored (ADR-0018).
-- Admitted to db/messages/ingest, it can also mask a stored subject again under another scanner.
GRANT SELECT (
    account_id,
    message_id,
    from_email,
    from_domain,
    from_name,
    subject,
    subject_masked,
    subject_scanner_version,
    subject_scanner_revision,
    sent_at,
    labels,
    flags,
    list_id,
    size_bytes,
    sender_class,
    content_flags,
    scan_state,
    scanner_version,
    scanner_revision
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
    subject_scanner_version,
    subject_scanner_revision,
    sent_at,
    labels,
    flags,
    has_attachments,
    list_id,
    size_bytes,
    auth_results,
    sender_class,
    class_rule_id
),
UPDATE (
    subject,
    subject_masked,
    subject_scanner_version,
    subject_scanner_revision,
    labels,
    flags,
    sender_class,
    class_rule_id,
    scan_state,
    content_flags,
    rule_ids,
    scanned_at,
    scanner_version,
    scanner_revision
),
DELETE ON messages TO mediated_mailbox_sync;
GRANT INSERT (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision) ON masking_events
TO mediated_mailbox_sync;
GRANT USAGE ON SEQUENCE masking_events_id_seq TO mediated_mailbox_sync;
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
    scan_hit_count,
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
    scan_hit_count,
    sender_class
),
DELETE ON senders TO mediated_mailbox_sync;

-- Once backfill's second pass has ended, a tick runs the delisting transition, records the gate's
-- decision on each message it decides with its reason, records each verdict or skip, and adds each
-- flagged verdict to its sender's prior hits (ADR-0037, ADR-0093, ADR-0104).
GRANT SELECT (account_id, message_id, decision, reason),
INSERT (account_id, message_id, decision, reason),
UPDATE (decision, reason, decided_at) ON scan_gate_decisions TO mediated_mailbox_sync;

-- Every tick and every gap recovery is recorded with its checkpoint, counters, timeline and failed
-- items (ADR-0022). A tick reads the latest tick's checkpoint to continue its scanning from there.
GRANT SELECT (account_id, run_id, workload, pass, state, started_at, checkpoint, counters),
INSERT (
    account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters
),
UPDATE (state, finished_at, heartbeat_at, checkpoint, counters, last_error) ON job_runs
TO mediated_mailbox_sync;
GRANT INSERT (account_id, run_id, kind, page, detail) ON job_run_events TO mediated_mailbox_sync;
GRANT USAGE ON SEQUENCE job_run_events_seq_seq TO mediated_mailbox_sync;
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
) ON job_run_failures TO mediated_mailbox_sync;
GRANT USAGE ON SEQUENCE job_run_failures_seq_seq TO mediated_mailbox_sync;

-- A tick reads the account's progress, which tells it whether backfill's second pass has ended, and
-- the lowered target it spends under. It reads and writes the change cursor with its write time, and
-- records the latest authentication attempt its adapter reports, only over an older one (ADR-0097).
GRANT SELECT (
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome,
    lowered_target_rate
),
UPDATE (sync_cursor, sync_cursor_at, last_auth_at, last_auth_outcome) ON account_state
TO mediated_mailbox_sync;

-- Delta sync re-seals an OAuth client's secret by compare-and-set on the stored bytes, the one write
-- of oauth_clients beside the UI's setup (ADR-0089, ADR-0092, ADR-0016).
GRANT UPDATE (client_secret) ON oauth_clients TO mediated_mailbox_sync;

-- Delta sync loads policy through the shared policy loader, as backfill and the mediator do.
GRANT SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules TO mediated_mailbox_sync;
