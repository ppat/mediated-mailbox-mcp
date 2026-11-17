-- +goose Up
-- The application user's grants (ADR-0143), the one user the API server, the UI and every job kind of
-- the worker connect as. Every grantee is the placeholder the migrator replaces with the application
-- user's configured name, since the project names no database user (ADR-0048).
--
-- The grants hold ADR-0143's line. The user's writes are exactly what the data-access library's
-- statements write, and it holds no DDL right, since the migrator's user owns every table (ADR-0048).
-- It holds no UPDATE, DELETE or TRUNCATE on audit_log or policy_changes, so a grant that writes either
-- is INSERT alone (ADR-0016, ADR-0102). Reads beyond what the statements read are acceptable, so every
-- table but the schema's own bookkeeping is readable whole. Nothing automated refuses a grant beyond
-- the line, so a migration that grants is reviewed against the statements it serves. One user's
-- grants cannot tell one program's code from another's, which the import lists do (ADR-0071).
--
-- The tables are in ADR-0016's order, each with the writes its statements make and the subsections
-- holding them.

GRANT SELECT ON
oauth_clients,
accounts,
account_state,
rate_state,
rate_grants,
senders,
messages,
attachment_media,
scan_gate_decisions,
policy_candidates,
policy_rules,
policy_changes,
masking_events,
reorg_plans,
reorg_plan_ops,
reorg_op_log,
job_runs,
job_run_events,
job_run_failures,
audit_log
TO application_user;

-- The UI's client setup in db/oauthclients/setup adds a client, replaces a client's identifier, secret
-- and project ID, and removes a client no account connects through. The connections' re-seal in
-- db/oauthclients/secret rewrites a client's secret by compare-and-set on the stored bytes. No
-- statement changes a client's name or provider (ADR-0016, ADR-0084, ADR-0092, ADR-0106).
GRANT INSERT (name, provider, client_id, client_secret, project_id),
UPDATE (client_id, client_secret, project_id),
DELETE ON oauth_clients TO application_user;

-- The UI's account setup in db/accounts/setup writes an account's listing row with the client it
-- connects through, and moves an account to another client of its provider (ADR-0091, ADR-0106).
GRANT INSERT (account_id, provider, oauth_client), UPDATE (oauth_client) ON accounts TO application_user;

-- Account setup writes an account's state row with its sealed credential, the mailbox it remembers and
-- its lowered target, replaces the credential on a re-authorization, and account settings writes the
-- target. The connections write a rotated credential back by compare-and-set in
-- db/accountstate/credential (ADR-0082, ADR-0089). Backfill sets and clears its completion flags and
-- the mark that starts its second pass over in db/accountstate/completion (ADR-0096), delta sync
-- writes its change cursor in db/accountstate/cursor, and every program records the latest
-- authentication attempt, only over an older one, in db/accountstate (ADR-0097).
GRANT INSERT (account_id, credential, mailbox, lowered_target_rate),
UPDATE (
    credential,
    mailbox,
    lowered_target_rate,
    backfill_pass1_complete,
    backfill_pass2_complete,
    backfill_pass2_restart,
    sync_cursor,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
) ON account_state TO application_user;

-- The rate limiter's statements in db/ratestate/limiter insert the account's rate state when it has
-- none and update every column but the account. They insert and delete grants, and move a grant
-- stamped ahead of the clock back to it (ADR-0024, ADR-0025).
GRANT INSERT (account_id, current_rate, target_rate, hard_cap),
UPDATE (
    current_rate,
    target_rate,
    hard_cap,
    baseline_p50_ms,
    last_throttle_at,
    backoff_until,
    throttles,
    bucket_level,
    bucket_filled_at,
    class_asked_at,
    last_granted_at,
    latency_window_start,
    latency_samples,
    latency_medians,
    classes,
    updated_at
) ON rate_state TO application_user;
GRANT INSERT (account_id, class, tokens, issued_at), UPDATE (issued_at), DELETE ON rate_grants
TO application_user;

-- Backfill and delta sync rebuild the statistics of each sender they saw from the messages the index
-- holds, and add a flagged verdict to its sender's prior hits, in db/senders/statistics (ADR-0093,
-- ADR-0096). Delta sync removes a sender's statistics once none of its messages is stored, in
-- db/messages/change (ADR-0018).
GRANT INSERT (
    account_id,
    domain,
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution
),
UPDATE (
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution,
    scan_hit_count
),
DELETE ON senders TO application_user;

-- Adding a message's metadata to the index, with the pair its subject was masked under, and masking a
-- stored subject again sit in db/messages/ingest (ADR-0096, ADR-0133). A scan verdict, a skip, a
-- refused body with its bound, the delisting transition and the returns to pending sit in
-- db/messages/scan (ADR-0037, ADR-0132). Delta sync sets a stored message's labels and flags to the
-- ones the provider reports and removes a message the provider no longer holds, in db/messages/change
-- (ADR-0018). A message's attachment media are inserted with it and never change, and a removed
-- message takes them with it through their foreign key, which runs as the tables' owner (ADR-0123).
GRANT INSERT (
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
    auth_results
),
UPDATE (
    subject,
    subject_masked,
    subject_scanner_version,
    subject_scanner_revision,
    labels,
    flags,
    scan_state,
    content_flags,
    rule_ids,
    scanned_at,
    scanner_version,
    scanner_revision,
    refused_bound
),
DELETE ON messages TO application_user;
GRANT INSERT (account_id, message_id, media_type, extension) ON attachment_media TO application_user;

-- Every gate decision is recorded with its reason, and a message decided again after the delisting
-- transition has its decision replaced, in db/scangatedecisions/record (ADR-0093).
GRANT INSERT (account_id, message_id, decision, reason),
UPDATE (decision, reason, decided_at) ON scan_gate_decisions TO application_user;

-- The UI's review of a candidate sets its status, and a confirmation inserts its rule (ADR-0084,
-- ADR-0021).
GRANT UPDATE (status, reviewed_at, reviewed_by) ON policy_candidates TO application_user;

-- Policy management adds, edits and lifts rules, and an import does all three, in
-- db/policyrules/manage for an account's rules and db/policyrules/base for the base policy's (ADR-0084,
-- ADR-0110, ADR-0112). An edit changes a rule's domain suffixes alone, since its scope and identifier
-- never change.
GRANT INSERT, UPDATE (domain_suffix), DELETE ON policy_rules TO application_user;

-- Each policy write appends its history row. Insert alone, so the history survives a compromise of
-- the process that wrote it (ADR-0102).
GRANT INSERT ON policy_changes TO application_user;
GRANT USAGE ON SEQUENCE policy_changes_id_seq TO application_user;

-- Each subject mask applied is recorded in db/maskingevents/record (ADR-0003).
GRANT INSERT (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision) ON masking_events
TO application_user;
GRANT USAGE ON SEQUENCE masking_events_id_seq TO application_user;

-- The UI's approval of a plan sets its status (ADR-0020, ADR-0084). The API server's code is held from
-- this write by its import lists and the raw SQL analyser, not by a grant (ADR-0143).
GRANT UPDATE (status, approved_at, approved_by) ON reorg_plans TO application_user;

-- Every run is recorded with its checkpoint, counters, timeline and failed items, in
-- db/jobruns/record (ADR-0117, ADR-0119).
GRANT INSERT (
    account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters
),
UPDATE (state, finished_at, heartbeat_at, checkpoint, counters, last_error) ON job_runs
TO application_user;
GRANT INSERT (account_id, run_id, kind, page, detail) ON job_run_events TO application_user;
GRANT USAGE ON SEQUENCE job_run_events_seq_seq TO application_user;
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
) ON job_run_failures TO application_user;
GRANT USAGE ON SEQUENCE job_run_failures_seq_seq TO application_user;

-- Each serve and each denial of a body is recorded through db/auditlog/record, as an insert and nothing
-- else (ADR-0002, ADR-0016).
GRANT INSERT (
    account_id, actor, action, message_id, stage, reason, sender_class, class_rule_id, class_rule_scope, content_flags,
    content_rule_ids, serve_rule_ids, scan_state, scanner_version, scanner_revision
) ON audit_log TO application_user;
GRANT USAGE ON SEQUENCE audit_log_id_seq TO application_user;

-- The start check's statement in db/schemacompatibility runs on each pool a process opens, before the
-- pool is used (ADR-0140). It reads the newest migration goose records and the compatibility floor,
-- which describe the schema and no account.
GRANT SELECT (version_id) ON goose_db_version TO application_user;
GRANT SELECT (oldest_chain_version) ON schema_compatibility TO application_user;
