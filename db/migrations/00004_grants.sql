-- +goose Up
-- The runtime roles' grants (ADR-0118), one section per runtime role in the order db/bootstrap creates
-- the roles, then one section per shared library whose statements run under the role of each
-- deployable or job kind that imports it, naming every role it covers. Each role gets only what its own
-- statements need, under ADR-0118's three lines. Nothing automated refuses a grant beyond what a
-- role's statements need, so a migration that grants is reviewed against the statements it serves.
--
-- No runtime role holds UPDATE, DELETE or TRUNCATE on audit_log or policy_changes (ADR-0016, ADR-0102).
-- A grant that writes either is INSERT alone.
--
-- The migration role owns every table, so no runtime role can change the schema (ADR-0048).

-- The mediator (ADR-0118).
--
-- Its read operations and its system status read the index and the recorded state through the
-- subsections its list admits, db/messages, db/maskingevents, db/senders, db/jobruns and
-- db/accountstate. The mediator writes none of these tables, holds no grant on the sealed credential,
-- and holds none on the stored sender class, which a record forbids it to act on (ADR-0002). The
-- statements reading that class sit in db/messages/classification and db/jobruns/classification,
-- which the mediator's list does not admit.
--
-- The sender domain term, the grouping by sender domain and the read of the distinct senders the
-- service layer classifies read messages.from_domain (ADR-0108, ADR-0109). The gate's read in
-- db/messages names the content rules that set a message's flags, which the body request's audit row
-- records (ADR-0002). Its message reads read each message's attachment media, from which the service
-- layer derives the types it serves (ADR-0123).
GRANT SELECT (
    account_id,
    message_id,
    thread_id,
    from_email,
    from_domain,
    from_name,
    subject,
    sent_at,
    labels,
    flags,
    has_attachments,
    content_flags,
    rule_ids,
    scan_state
) ON messages TO mediated_mailbox_mediate;
GRANT SELECT (account_id, message_id, media_type, extension) ON attachment_media TO mediated_mailbox_mediate;
GRANT SELECT (id, account_id, message_id, field, rule_id, tier, masked_at) ON masking_events
TO mediated_mailbox_mediate;
-- The sender listing reads every column of the sender statistics but the stored sender class, which a
-- record forbids the mediator to act on, and the prior scan hits, which no read serves (ADR-0002,
-- ADR-0118). The rebuild of the statistics and the counts of prior scan hits sit in
-- db/senders/statistics, which the mediator's list does not admit.
GRANT SELECT (
    account_id,
    domain,
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution
) ON senders TO mediated_mailbox_mediate;
-- The account's progress for the system status, and the lowered target it will spend under. At the end
-- of each body request the mediator records the authentication attempt its adapter reports, only over
-- an older one, through db/accountstate/authentication (ADR-0097).
GRANT SELECT (
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome,
    lowered_target_rate
),
UPDATE (last_auth_at, last_auth_outcome) ON account_state TO mediated_mailbox_mediate;
-- The runs, their timelines and their failures' dispositions, which the system status and the UI's
-- reads in db/jobruns, the tables' own subsection, read. Reads beyond the mediator's own need are
-- accepted by ADR-0118's three lines, and none of these columns is one a record forbids it.
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
GRANT SELECT (account_id, run_id, seq, kind, at, page, detail) ON job_run_events TO mediated_mailbox_mediate;
GRANT SELECT (account_id, run_id, disposition, recovered_by) ON job_run_failures TO mediated_mailbox_mediate;
GRANT SELECT (account_id, plan_id, description, status) ON reorg_plans TO mediated_mailbox_mediate;
-- Each serve and each denial of a body is recorded through db/auditlog/record, which only the
-- mediator's list admits, as an insert and nothing else (ADR-0002, ADR-0016).
GRANT INSERT (account_id, actor, action, message_id, sensitivity, rule_ids) ON audit_log TO mediated_mailbox_mediate;
GRANT USAGE ON SEQUENCE audit_log_id_seq TO mediated_mailbox_mediate;

-- Backfill (ADR-0118, ADR-0017).
--
-- Its statements sit in db/messages/ingest, db/messages/scan, db/maskingevents/record,
-- db/senders/statistics, db/scangatedecisions/record, db/jobruns/record, db/accountstate/completion
-- and db/accountstate/authentication, and in db/accountstate, whose read of the lowered target every
-- role admitted there runs.
--
-- Pass 1 adds each message's metadata to the index with the pair its subject was masked under and its
-- attachments' media, which never change once stored, so no role updates or deletes them
-- (ADR-0123), and records the masks applied to its subject. It finds whether any stored subject carries another pair,
-- masks such a subject again, and masks whole the stored subject of a message the provider no longer
-- has, reading it first (ADR-0096). It writes the policy rule behind each message's class with the
-- class (ADR-0016). Pass 2 reads the messages waiting for a scan with the inputs the scan gate reads
-- and records each message's scan verdict or skip state. The delisting transition reads the domains
-- of the messages stored as restricted or skipped as restricted, and marks those messages of a domain
-- the policy now classifies normal back to a normal sender class and pending scan, clearing the rule
-- with the class (ADR-0037). Backfill's run-start step, before the first pass, returns each scanned message whose
-- verdict carries another pair to pending with its verdict cleared.
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
    sender_class,
    class_rule_id,
    scan_state,
    content_flags,
    rule_ids,
    scanned_at,
    scanner_version,
    scanner_revision
) ON messages TO mediated_mailbox_backfill;
GRANT INSERT (account_id, message_id, media_type, extension) ON attachment_media TO mediated_mailbox_backfill;
GRANT INSERT (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision) ON masking_events
TO mediated_mailbox_backfill;
GRANT USAGE ON SEQUENCE masking_events_id_seq TO mediated_mailbox_backfill;
-- Pass 1 rebuilds the statistics of each sender it saw from the messages the index holds. A verdict
-- carrying a content flag adds to its sender's prior hits, which the gate reads, and a run counts them
-- again from the flagged verdicts the index holds (ADR-0093, ADR-0096).
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
) ON senders TO mediated_mailbox_backfill;
-- Every gate decision is recorded with its reason, and a message decided again after the delisting
-- transition has its decision replaced (ADR-0093). The upsert reads the conflict columns and the
-- values it replaces them with.
GRANT SELECT (account_id, message_id, decision, reason),
INSERT (account_id, message_id, decision, reason),
UPDATE (decision, reason, decided_at) ON scan_gate_decisions TO mediated_mailbox_backfill;
-- Every run is recorded with its checkpoint, counters, timeline and failed items (ADR-0117). A run
-- reads the latest run of its pass to resume from it. Each account's job, at every ensure, reads its
-- latest success from its runs' ends, starts and progress events (ADR-0119).
GRANT SELECT (account_id, run_id, workload, pass, state, started_at, finished_at, checkpoint, counters),
INSERT (
    account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters
),
UPDATE (state, finished_at, heartbeat_at, checkpoint, counters, last_error) ON job_runs
TO mediated_mailbox_backfill;
GRANT SELECT (account_id, run_id, kind, at),
INSERT (account_id, run_id, kind, page, detail) ON job_run_events TO mediated_mailbox_backfill;
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
-- Each pass reads its completion flag through the account's progress and sets it when it ends. The
-- run-start step that returns stale verdicts to pending marks the second pass to start over, and the
-- second pass run that starts clears the mark (ADR-0096). Backfill reads the lowered target it spends
-- under, and records the authentication attempt its adapter reports at the end of each unit of work,
-- only over an older one (ADR-0097).
GRANT SELECT (
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    backfill_pass2_restart,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome,
    lowered_target_rate
),
UPDATE (
    backfill_pass1_complete,
    backfill_pass2_complete,
    backfill_pass2_restart,
    last_auth_at,
    last_auth_outcome
) ON account_state TO mediated_mailbox_backfill;

-- Delta sync (ADR-0118, ADR-0018).
--
-- Its statements sit in the subsections its list admits. db/messages/ingest, db/maskingevents/record,
-- db/senders/statistics, db/jobruns/record, db/messages/scan, db/scangatedecisions/record and
-- db/accountstate/authentication it shares with backfill, since a tick adds and masks mail as
-- backfill's first pass does and decides and scans what waits as its second pass does (ADR-0104).
-- db/messages/change, db/accountstate/cursor and db/oauthclients/secret only its list admits.
-- db/accountstate it reads.
--
-- A tick adds each message's metadata to the index with its attachments' media, which it never
-- updates (ADR-0123), masks its subject and records the masks, and
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
-- A message the tick removes takes its attachment media with it through their foreign key, which runs
-- as the tables' owner, so delta sync's role needs no delete on them (ADR-0123).
GRANT INSERT (account_id, message_id, media_type, extension) ON attachment_media TO mediated_mailbox_sync;
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
-- items (ADR-0117). A tick reads the latest tick's checkpoint to continue its scanning from there.
-- Each account's job, at every ensure, reads its latest success from its ticks' ends, starts and
-- progress events (ADR-0119).
GRANT SELECT (account_id, run_id, workload, pass, state, started_at, finished_at, checkpoint, counters),
INSERT (
    account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters
),
UPDATE (state, finished_at, heartbeat_at, checkpoint, counters, last_error) ON job_runs
TO mediated_mailbox_sync;
GRANT SELECT (account_id, run_id, kind, at),
INSERT (account_id, run_id, kind, page, detail) ON job_run_events TO mediated_mailbox_sync;
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

-- The reorganization workload (ADR-0118). Its own grants arrive with its statements. It spends from the
-- budget and holds an account snapshot, through the shared libraries' sections below.

-- The heuristics workload (ADR-0118). Its grants arrive with its statements, so it holds none.

-- The UI (ADR-0084, ADR-0021).
--
-- It reads the tables its screens and endpoints read (docs/UI.md). Its write grant is the columns its
-- two decision verbs set and the rule a confirmation inserts. Its list admits db/messages, whose
-- message reads read the attachment media beside the messages (ADR-0123).
GRANT SELECT ON
accounts,
rate_state,
senders,
messages,
attachment_media,
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
-- Account setup writes an account's listing row, with the client it connects through, and moves an
-- account to another client of its provider (ADR-0091, ADR-0106). accounts_account confines both
-- writes to the transaction's account. Its statements sit in db/accounts/setup.
GRANT INSERT (account_id, provider, oauth_client), UPDATE (oauth_client) ON accounts TO mediated_mailbox_ui;
-- The UI's system endpoint and jobs cards show an account's progress (ADR-0084, ADR-0091). Account
-- setup writes the account's state row, its sealed credential, the mailbox it remembers and its
-- lowered target, replaces the credential on a re-authorization, and records the code exchange's
-- authentication attempt through db/accountstate/authentication's latest-wins statement (ADR-0097).
-- Account settings reads the mailbox and the lowered target and writes the target. The grant never
-- covers reading credential, so the UI cannot read back what it sealed (ADR-0081, ADR-0084).
GRANT SELECT (
    account_id,
    mailbox,
    lowered_target_rate,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
),
INSERT (account_id, credential, mailbox, lowered_target_rate),
UPDATE (credential, mailbox, lowered_target_rate, last_auth_at, last_auth_outcome)
ON account_state TO mediated_mailbox_ui;
-- The UI's client setup lists each client's identity, adds a client, replaces a client's identifier,
-- secret and project ID, and removes a client no account connects through. Its statements sit in
-- db/oauthclients/setup and read no secret. The one part of the UI that opens a client's secret, for a
-- consent's code exchange, reads the sealed secret through db/oauthclients, which only that part's list
-- admits, and the raw SQL analyser refuses a statement the UI's shipped code runs by hand (ADR-0071,
-- ADR-0081). Code written to get around both, such as a statement run through reflection, stays
-- with review. The table has no row-level security, so the grant is the whole database barrier
-- (ADR-0016, ADR-0084, ADR-0106).
GRANT SELECT (name, provider, client_id, client_secret, project_id),
INSERT (name, provider, client_id, client_secret, project_id),
UPDATE (client_id, client_secret, project_id),
DELETE ON oauth_clients TO mediated_mailbox_ui;
-- Policy management adds, edits and lifts rules, and an import does all three (ADR-0084, ADR-0110).
-- An edit changes a rule's domain suffixes alone, since its scope and identifier never change. Its
-- statements sit in db/policyrules/manage for an account's rules and db/policyrules/base for the base
-- policy's, which only the UI's list admits. A confirmation of a candidate inserts its rule.
GRANT INSERT, UPDATE (domain_suffix), DELETE ON policy_rules TO mediated_mailbox_ui;
-- Each policy write appends its history row, and the policy screens read the history. Insert alone,
-- so evidence of who lifted a restriction survives the UI's compromise (ADR-0102).
GRANT SELECT, INSERT ON policy_changes TO mediated_mailbox_ui;
GRANT USAGE ON SEQUENCE policy_changes_id_seq TO mediated_mailbox_ui;

-- The rate limiter's statements, in db/ratestate, run under the role of each deployable or job kind
-- that spends from the budget (ADR-0118), the mediator, backfill and delta sync. The grants also name
-- mediated_mailbox_organize, the role of reorg apply and rollback (ADR-0118), a job kind that spends
-- from the budget. They insert the account's rate state when it has none, read it, and update every
-- column but the account. They read, insert and delete grants, and move a grant stamped ahead of the
-- clock back to it.
GRANT SELECT, INSERT (account_id, current_rate, target_rate, hard_cap),
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
) ON rate_state
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT, INSERT (account_id, class, tokens, issued_at), UPDATE (issued_at), DELETE ON rate_grants
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;

-- The policy loader's statement, in db/policyrules, runs under the role of each deployable or job kind that loads
-- policy (ADR-0118), the mediator, backfill and delta sync. It reads the account, the identifier, the
-- class and the domain suffixes of the base rules and the account's own, so the roles get SELECT on
-- exactly those columns. Who created a rule and when stay out of their reach. Row-level security still
-- shows each only the base rules and its transaction's account's rules.
GRANT SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync;

-- The account snapshot's statements in db/accounts, db/accountstate/credential and db/oauthclients run
-- under the role of each deployable or job kind that calls a provider, whose list admits accountload
-- (ADR-0118, ADR-0090), the mediator, backfill and delta sync. The grants also name
-- mediated_mailbox_organize, the role of reorg apply and rollback (ADR-0118), a job kind that calls a
-- provider. Each lists the accounts with the client each names, reads an account's sealed credential,
-- writes a rotated credential back by compare-and-set on the bytes it last knew (ADR-0082, ADR-0089),
-- and reads every OAuth client.
GRANT SELECT (account_id, provider, oauth_client) ON accounts
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT (account_id, credential), UPDATE (credential) ON account_state
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT (name, provider, client_id, client_secret) ON oauth_clients
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
