-- +goose Up
-- The schema of ADR-0016. No table holds a body, a snippet or an excerpt, and every table keys on
-- account_id apart from reorg_op_log, which is scoped through its plan. A future migration proposing
-- a body column is violating the design, not extending it.

CREATE TABLE accounts (
    account_id text PRIMARY KEY,
    provider text NOT NULL,
    backfill_pass1_complete boolean NOT NULL DEFAULT false,
    backfill_pass2_complete boolean NOT NULL DEFAULT false,
    sync_cursor text,
    -- When sync_cursor was last written, by delta sync.
    sync_cursor_at timestamptz,
    -- The last provider authentication attempt and its outcome, exposed by ADR-0034.
    last_auth_at timestamptz,
    last_auth_outcome text
);

-- Cross-process rate coordination (ADR-0025).
CREATE TABLE rate_state (
    account_id text PRIMARY KEY REFERENCES accounts,
    -- Units per second, managed by the controller.
    current_rate real NOT NULL,
    -- The conservative target (ADR-0024).
    target_rate real NOT NULL,
    -- Never exceeded.
    hard_cap real NOT NULL,
    -- For latency-based decrease.
    baseline_p50_ms real,
    last_throttle_at timestamptz,
    backoff_until timestamptz,
    leased_tokens real NOT NULL DEFAULT 0,
    lease_expires_at timestamptz,
    -- {class: {reserved, used}} per priority class, written by the limiter (ADR-0025).
    classes jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Drives the scan gate, memoization and the heuristics.
CREATE TABLE senders (
    account_id text NOT NULL REFERENCES accounts,
    domain citext NOT NULL,
    local_part_sample text[],
    display_names text[],
    message_count bigint NOT NULL DEFAULT 0,
    first_seen timestamptz,
    last_seen timestamptz,
    -- Newsletter against transactional signal.
    has_list_id_ratio real,
    label_distribution jsonb,
    scan_hit_count bigint NOT NULL DEFAULT 0,
    sender_class text NOT NULL DEFAULT 'normal',
    -- Heuristic candidates (ADR-0004).
    embedding vector(384),
    PRIMARY KEY (account_id, domain)
);

-- No body, no snippet, no excerpt column. By design.
CREATE TABLE messages (
    account_id text NOT NULL REFERENCES accounts,
    message_id text NOT NULL,
    thread_id text NOT NULL,
    from_email citext NOT NULL,
    -- Denormalized for the classification hot path.
    from_domain citext NOT NULL,
    from_name text,
    -- Masked at rest if a code was detected.
    subject text,
    subject_masked boolean NOT NULL DEFAULT false,
    sent_at timestamptz NOT NULL,
    labels text[] NOT NULL DEFAULT '{}',
    flags jsonb NOT NULL DEFAULT '{}',
    has_attachments boolean NOT NULL,
    attachment_types text[] NOT NULL DEFAULT '{}',
    list_id text,
    size_bytes int,
    auth_results jsonb,
    -- normal | restricted
    sender_class text NOT NULL,
    -- mfa_code | login_link
    content_flags text[] NOT NULL DEFAULT '{}',
    rule_ids text[] NOT NULL DEFAULT '{}',
    -- scanned | skipped_restricted | skipped_gate | pending (ADR-0007)
    scan_state text NOT NULL DEFAULT 'pending',
    scanned_at timestamptz,
    scanner_version int,
    PRIMARY KEY (account_id, message_id)
);

CREATE INDEX ON messages (account_id, from_domain);
CREATE INDEX ON messages (account_id, sent_at DESC);
CREATE INDEX ON messages USING gin (labels);
CREATE INDEX ON messages (account_id, sent_at) WHERE labels = '{}';
CREATE INDEX ON messages (account_id) WHERE scan_state = 'pending';
CREATE INDEX ON messages USING gin (subject gin_trgm_ops);

-- Makes ADR-0007's residual auditable.
CREATE TABLE scan_gate_decisions (
    account_id text NOT NULL,
    message_id text NOT NULL,
    -- SCAN | SKIP
    decision text NOT NULL,
    -- restricted | high_volume_no_hits | ...
    reason text NOT NULL,
    decided_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, message_id)
);

-- The heuristic review queue (ADR-0004).
CREATE TABLE policy_candidates (
    account_id text NOT NULL,
    domain citext NOT NULL,
    -- One entry per heuristic that fired, with its evidence.
    signals jsonb NOT NULL,
    score real NOT NULL,
    -- pending | confirmed | dismissed
    status text NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    -- Human only, and in the UI's write grant (ADR-0021).
    reviewed_by text,
    PRIMARY KEY (account_id, domain)
);

-- The sender policy as rows (ADR-0004), snapshotted by ADR-0041.
CREATE TABLE policy_rules (
    -- NULL for the base policy. An overlay names its account.
    account_id text REFERENCES accounts,
    -- candidate.{account}.{domain} when a confirmation minted it.
    rule_id text PRIMARY KEY,
    -- restricted
    class text NOT NULL,
    domain_suffix text[] NOT NULL,
    -- operator | candidate
    source text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- The operator identity, and in the UI's write grant (ADR-0021).
    created_by text NOT NULL
);
CREATE INDEX ON policy_rules (account_id);

CREATE TABLE masking_events (
    id bigserial PRIMARY KEY,
    -- Indexed below with masked_at.
    account_id text NOT NULL,
    message_id text NOT NULL,
    -- subject
    field text NOT NULL,
    rule_id text NOT NULL,
    tier int NOT NULL,
    masked_at timestamptz NOT NULL DEFAULT now()
    -- No matched text is stored.
);
CREATE INDEX ON masking_events (account_id, masked_at DESC);

CREATE TABLE reorg_plans (
    plan_id uuid PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts,
    -- DRAFT | APPROVED | APPLYING | APPLIED | ROLLED_BACK | REJECTED | APPLY_REFUSED
    status text NOT NULL,
    description text,
    -- The client actor at creation.
    proposer text,
    -- label_ops as [{op: create|rename|delete, label, to}], message_ops, stats.
    plan jsonb NOT NULL,
    -- {result: passed|failed, findings: []} at creation (ADR-0032).
    validation jsonb,
    -- The same shape, at apply.
    apply_validation jsonb,
    -- Set with APPLY_REFUSED.
    refusal_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- The decision time, for REJECTED as well.
    approved_at timestamptz,
    -- Human only, and in the UI's write grant (ADR-0021).
    approved_by text
);
CREATE INDEX ON reorg_plans (account_id, created_at DESC);

-- The plan's message operations as rows, one per message (ADR-0020).
CREATE TABLE reorg_plan_ops (
    account_id text NOT NULL REFERENCES accounts,
    plan_id uuid NOT NULL REFERENCES reorg_plans,
    message_id text NOT NULL,
    add_labels text[] NOT NULL DEFAULT '{}',
    remove_labels text[] NOT NULL DEFAULT '{}',
    -- 'from>to' per (removed or none, added or none) pair.
    flows text[] NOT NULL DEFAULT '{}',
    reason text,
    PRIMARY KEY (plan_id, message_id)
);
CREATE INDEX ON reorg_plan_ops (account_id, plan_id);

CREATE TABLE reorg_op_log (
    plan_id uuid NOT NULL REFERENCES reorg_plans,
    seq bigserial,
    message_id text NOT NULL,
    labels_before text[] NOT NULL,
    labels_after text[] NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, seq)
);

-- Every batch workload's runs (ADR-0022).
CREATE TABLE job_runs (
    account_id text NOT NULL REFERENCES accounts,
    -- A short opaque string.
    run_id text NOT NULL,
    -- backfill | sync | apply | heuristics
    workload text NOT NULL,
    -- pass1 | pass2 | tick | gap_recovery | apply | rollback
    pass text,
    -- running | succeeded | failed
    state text NOT NULL,
    -- Apply and rollback runs.
    plan_id uuid REFERENCES reorg_plans,
    -- The run this one resumed.
    resumed_from text,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    heartbeat_at timestamptz,
    -- {page, of} or {seq, of}
    checkpoint jsonb,
    -- The counters per workload. pass1 counts pages and messages. pass2 counts pages, decided,
    -- pending, scanned and skipped. sync counts added, modified, removed, window_start, window_end and
    -- reconciled. apply counts ops_done, ops_total and failures. heuristics counts candidates.
    counters jsonb NOT NULL DEFAULT '{}',
    -- Provider or scanner text, never a body.
    last_error text,
    PRIMARY KEY (account_id, run_id)
);

-- A run's timeline.
CREATE TABLE job_run_events (
    account_id text NOT NULL,
    run_id text NOT NULL,
    seq bigserial,
    -- start | progress | backoff | retry | failure | resume | finish
    kind text NOT NULL,
    at timestamptz NOT NULL DEFAULT now(),
    -- Progress events carry the checkpoint page.
    page int,
    -- Provider or scanner text, never a body.
    detail text,
    PRIMARY KEY (account_id, run_id, seq)
);

-- A run's per-item failures.
CREATE TABLE job_run_failures (
    account_id text NOT NULL,
    run_id text NOT NULL,
    seq bigserial,
    -- page | message | op
    item_kind text NOT NULL,
    -- The page number, or the message id for message and op items.
    item_id text NOT NULL,
    -- The page the item was processed on.
    page int,
    -- throttled | provider_error | gone | scanner_timeout | validation | authentication
    error_class text NOT NULL,
    -- Provider or scanner text, never a body.
    error_summary text,
    attempts int NOT NULL DEFAULT 1,
    first_at timestamptz NOT NULL,
    last_at timestamptz NOT NULL,
    -- recovered | pending | gone | abandoned
    disposition text NOT NULL DEFAULT 'pending',
    -- The recovering run.
    recovered_by text,
    PRIMARY KEY (account_id, run_id, seq)
);

-- No runtime role holds UPDATE or DELETE here. Append-only is the grant, not a convention.
CREATE TABLE audit_log (
    id bigserial PRIMARY KEY,
    ts timestamptz NOT NULL DEFAULT now(),
    account_id text NOT NULL,
    actor text NOT NULL,
    -- READ_BODY | DENY_BODY | MUTATE | DENY_MUTATE
    action text NOT NULL,
    message_id text,
    sensitivity jsonb,
    rule_ids text[]
);
CREATE INDEX ON audit_log (account_id, ts DESC);
