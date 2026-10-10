-- +goose Up
-- The schema of ADR-0016, every table in its final shape and in that record's order. No table holds a
-- body, a snippet or an excerpt, and every table keys on account_id apart from reorg_op_log, which is
-- scoped through its plan, and oauth_clients, which belongs to no account. A future migration
-- proposing a body column is violating the design, not extending it.
--
-- Each closed vocabulary a column holds is checked, in ADR-0016's three tiers. A token added later
-- costs a forward migration that replaces the constraint in the release that writes the token.

-- An installation's OAuth clients, any number for a provider that authenticates through one, each
-- named by the person running the installation (ADR-0080, ADR-0083, ADR-0106). It carries no account
-- column, so grants alone decide who reaches it (ADR-0016). A provider that authenticates otherwise
-- has no row, and no account refers to one.
CREATE TABLE oauth_clients (
    name text PRIMARY KEY,
    provider text NOT NULL,
    client_id text NOT NULL,
    -- The client's secret, sealed to the current public key and bound to this row (ADR-0081, ADR-0088).
    client_secret bytea NOT NULL,
    -- The provider's project the client belongs to, Google Cloud's project ID for Gmail, which client
    -- setup writes and the installation screen links to. Not secret, and NULL when the setup was given
    -- none.
    project_id text,
    -- What an account's reference names, so the reference carries the provider.
    UNIQUE (name, provider),
    -- One client is never stored twice under two names.
    UNIQUE (provider, client_id)
);

-- What every listing needs, each account's identifier, provider and the client it connects through
-- (ADR-0091). The reference carries the provider, so an account cannot name another provider's
-- client, and a client an account connects through cannot be removed. The schema does not know which
-- providers use a client, so an account of such a provider that names none is refused service by the
-- account snapshot, never by the schema (ADR-0106).
CREATE TABLE accounts (
    account_id text PRIMARY KEY,
    -- Not checked, since a check would put a storage change in every new backend (P2).
    provider text NOT NULL,
    -- NULL for a provider without an OAuth client.
    oauth_client text,
    FOREIGN KEY (oauth_client, provider) REFERENCES oauth_clients (name, provider),
    -- No account identifier is exactly ., .. or /, which the mediator's API root cannot address as a
    -- path segment (ADR-0087). Account setup refuses them first, with its own wording.
    CONSTRAINT accounts_account_id_addressable CHECK (account_id NOT IN ('.', '..', '/'))
);

-- Everything else an account carries, one row per account (ADR-0091). An account with no state row is
-- not connected.
CREATE TABLE account_state (
    account_id text PRIMARY KEY REFERENCES accounts,
    -- The account's provider credential, sealed to the current public key and bound to this row
    -- (ADR-0081, ADR-0088). A deployable replaces it only if it still holds the bytes the deployable
    -- last read or wrote (ADR-0089).
    credential bytea,
    -- The mailbox the consent that first stored the credential confirmed. Re-authorization refuses a
    -- grant for any other (ADR-0080).
    mailbox text,
    -- A lower target the operator set, a fraction of the provider's declared ceiling, NULL for none
    -- (ADR-0024).
    lowered_target_rate real,
    backfill_pass1_complete boolean NOT NULL DEFAULT false,
    backfill_pass2_complete boolean NOT NULL DEFAULT false,
    -- The next pass 2 run starts over. The run-start step that returns stale verdicts to pending sets
    -- it, and the pass 2 run that starts clears it (ADR-0096, ADR-0120).
    backfill_pass2_restart boolean NOT NULL DEFAULT false,
    sync_cursor text,
    -- When sync_cursor was last written, by delta sync.
    sync_cursor_at timestamptz,
    -- The latest provider authentication attempt recorded and its outcome (ADR-0097), exposed by
    -- ADR-0034.
    last_auth_at timestamptz,
    last_auth_outcome text CHECK (last_auth_outcome IN ('succeeded', 'refused', 'failed'))
);

-- Cross-process rate coordination (ADR-0025). The issuer recomputes the target and the hard cap from
-- the provider's declared ceiling on every issue and never reads target_rate, hard_cap,
-- baseline_p50_ms or classes back. It writes them only for display, so no write to the row can raise
-- the cap (ADR-0024).
CREATE TABLE rate_state (
    account_id text PRIMARY KEY REFERENCES accounts,
    -- Units per second, managed by the controller.
    current_rate real NOT NULL,
    -- The conservative target (ADR-0024), shown only.
    target_rate real NOT NULL,
    -- Never exceeded, shown only.
    hard_cap real NOT NULL,
    -- The latency baseline in use, shown only.
    baseline_p50_ms real,
    last_throttle_at timestamptz,
    backoff_until timestamptz,
    -- Throttles since the last success, for the backoff.
    throttles integer NOT NULL DEFAULT 0,
    -- The token bucket's level, and the instant that level was reached.
    bucket_level real NOT NULL DEFAULT 0,
    bucket_filled_at timestamptz,
    -- When each class last asked, interactive, sync and batch in that order.
    class_asked_at timestamptz[],
    -- The instant of the latest grant.
    last_granted_at timestamptz,
    -- The current one-minute latency window, its samples in milliseconds, and the last ten window
    -- medians.
    latency_window_start timestamptz,
    latency_samples integer[],
    latency_medians real[],
    -- {class: {reserved, used}} per priority class, written by the limiter (ADR-0025), shown only.
    classes jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Every grant of the last second, which are also the live leases (ADR-0024, ADR-0025). The issuer
-- writes a grant in the same transaction as the bucket's level and deletes rows more than a second
-- old. A lost row widens issuance without the rules seeing it (ADR-0024). A lease expires one second
-- after it is issued.
CREATE TABLE rate_grants (
    grant_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts,
    class text NOT NULL CHECK (class IN ('interactive', 'sync', 'batch')),
    tokens real NOT NULL,
    issued_at timestamptz NOT NULL
);
CREATE INDEX ON rate_grants (account_id, issued_at);

-- Drives the scan gate and the heuristics.
CREATE TABLE senders (
    account_id text NOT NULL REFERENCES accounts,
    -- Stored lowercase, as the one Go normalizer writes it (ADR-0016). The check refuses an ASCII
    -- capital and means the same on every database, since the C collation folds only A to Z.
    domain text NOT NULL CHECK (domain = lower(domain COLLATE "C")),
    local_part_sample text[],
    display_names text[],
    message_count bigint NOT NULL DEFAULT 0,
    first_seen timestamptz,
    last_seen timestamptz,
    -- Newsletter against transactional signal.
    has_list_id_ratio real,
    label_distribution jsonb,
    scan_hit_count bigint NOT NULL DEFAULT 0,
    sender_class text NOT NULL DEFAULT 'normal' CHECK (sender_class IN ('normal', 'restricted')),
    PRIMARY KEY (account_id, domain)
);

-- No body, no snippet, no excerpt column. By design.
CREATE TABLE messages (
    account_id text NOT NULL REFERENCES accounts,
    message_id text NOT NULL,
    thread_id text NOT NULL,
    -- Case-insensitive, where display case must be kept and matching needs no index.
    from_email citext NOT NULL,
    -- Denormalized for the classification hot path, and stored lowercase as senders.domain is.
    from_domain text NOT NULL CHECK (from_domain = lower(from_domain COLLATE "C")),
    from_name text,
    -- Masked at rest if a code was detected.
    subject text,
    subject_masked boolean NOT NULL DEFAULT false,
    -- The scanner version and the scanner configuration's revision the subject was masked under
    -- (ADR-0096, ADR-0120).
    subject_scanner_version int,
    subject_scanner_revision text,
    sent_at timestamptz NOT NULL,
    labels text[] NOT NULL DEFAULT '{}',
    flags jsonb NOT NULL DEFAULT '{}',
    has_attachments boolean NOT NULL,
    attachment_types text[] NOT NULL DEFAULT '{}',
    list_id text,
    size_bytes int,
    auth_results jsonb,
    sender_class text NOT NULL CHECK (sender_class IN ('normal', 'restricted')),
    content_flags text[] NOT NULL DEFAULT '{}' CHECK (content_flags <@ '{mfa_code,login_link}'),
    -- The content rules that set content_flags (ADR-0009).
    rule_ids text[] NOT NULL DEFAULT '{}',
    -- The identifier of the policy rule that set sender_class, NULL when none did. An identifier, which
    -- two scopes may hold (ADR-0110). Whatever writes sender_class writes it too (ADR-0016, ADR-0037).
    class_rule_id text,
    -- ADR-0093's states.
    scan_state text NOT NULL DEFAULT 'pending'
    CHECK (scan_state IN ('scanned', 'skipped_restricted', 'skipped_gate', 'pending')),
    scanned_at timestamptz,
    scanner_version int,
    -- The scanner configuration's revision a verdict was made under (ADR-0009).
    scanner_revision text,
    PRIMARY KEY (account_id, message_id)
);

-- No index over labels or the subject. Array operators, ILIKE and trigram matching have no leakproof
-- form, so under row-level security no runtime role could use one (ADR-0016). The partial indexes
-- serve a statement that states their predicate exactly.
CREATE INDEX ON messages (account_id, from_domain);
CREATE INDEX ON messages (account_id, sent_at DESC);
CREATE INDEX ON messages (account_id, sent_at) WHERE labels = '{}';
CREATE INDEX ON messages (account_id) WHERE scan_state = 'pending';

-- Makes ADR-0093's residual auditable.
CREATE TABLE scan_gate_decisions (
    account_id text NOT NULL,
    message_id text NOT NULL,
    decision text NOT NULL CHECK (decision IN ('SCAN', 'SKIP')),
    -- restricted | high_volume_no_hits | ..., a vocabulary tuning grows, so not checked.
    reason text NOT NULL,
    decided_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, message_id)
);

-- The heuristic review queue (ADR-0004).
CREATE TABLE policy_candidates (
    account_id text NOT NULL,
    -- Stored lowercase, as senders.domain is.
    domain text NOT NULL CHECK (domain = lower(domain COLLATE "C")),
    -- One entry per heuristic that fired, with its evidence, in the shape ADR-0016 gives.
    signals jsonb NOT NULL,
    score real NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'dismissed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    -- Human only, and in the UI's write grant (ADR-0084).
    reviewed_by text,
    PRIMARY KEY (account_id, domain)
);

-- The sender policy as rows (ADR-0004), snapshotted by ADR-0041. A rule's identifier is unique within
-- its scope, the base policy or one account's own rules, so the base policy and an account, or two
-- accounts, may each hold a rule of one identifier (ADR-0110). The base policy is the scope whose
-- account is null, and NULLS NOT DISTINCT makes it one scope.
CREATE TABLE policy_rules (
    -- NULL for the base policy. An overlay names its account.
    account_id text REFERENCES accounts,
    -- candidate.{account}.{domain} when a confirmation minted it.
    rule_id text NOT NULL,
    class text NOT NULL CHECK (class IN ('restricted')),
    domain_suffix text[] NOT NULL,
    source text NOT NULL CHECK (source IN ('operator', 'candidate')),
    created_at timestamptz NOT NULL DEFAULT now(),
    -- The operator identity, and in the UI's write grant (ADR-0084).
    created_by text NOT NULL,
    CONSTRAINT policy_rules_scope_rule_id_key UNIQUE NULLS NOT DISTINCT (account_id, rule_id)
);
CREATE INDEX ON policy_rules (account_id);

-- Every change to policy_rules, appended in the same transaction (ADR-0102). A base rule's change has
-- a null account. No runtime role holds UPDATE or DELETE here, as on audit_log.
CREATE TABLE policy_changes (
    id bigserial PRIMARY KEY,
    -- NULL for a base rule's change. An overlay rule's names its account.
    account_id text REFERENCES accounts,
    ts timestamptz NOT NULL DEFAULT now(),
    -- The operator identity (ADR-0084).
    actor text NOT NULL,
    action text NOT NULL CHECK (action IN ('added', 'edited', 'lifted', 'confirmed')),
    rule_id text NOT NULL,
    -- Empty for added and confirmed.
    suffixes_before text[] NOT NULL DEFAULT '{}',
    -- Empty for lifted.
    suffixes_after text[] NOT NULL DEFAULT '{}'
);
CREATE INDEX ON policy_changes (account_id, ts DESC);

CREATE TABLE masking_events (
    id bigserial PRIMARY KEY,
    -- Indexed below with masked_at.
    account_id text NOT NULL,
    message_id text NOT NULL,
    field text NOT NULL CHECK (field IN ('subject')),
    rule_id text NOT NULL,
    tier int NOT NULL,
    -- The scanner the masking ran under (ADR-0120).
    scanner_version int,
    scanner_revision text,
    masked_at timestamptz NOT NULL DEFAULT now()
    -- No matched text is stored.
);
CREATE INDEX ON masking_events (account_id, masked_at DESC);

CREATE TABLE reorg_plans (
    plan_id uuid PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts,
    -- ADR-0020's set.
    status text NOT NULL CHECK (
        status IN ('DRAFT', 'APPROVED', 'APPLYING', 'APPLIED', 'ROLLED_BACK', 'REJECTED', 'APPLY_REFUSED')
    ),
    description text,
    -- The client actor at creation.
    proposer text,
    -- label_ops as [{op: create|rename|delete, label, to}], message_ops, stats.
    plan jsonb NOT NULL,
    -- {result: passed|failed, findings:[]} at creation (ADR-0032).
    validation jsonb,
    -- The same shape, at apply.
    apply_validation jsonb,
    -- Set with APPLY_REFUSED.
    refusal_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- The decision time, for REJECTED as well.
    approved_at timestamptz,
    -- Human only, and in the UI's write grant (ADR-0084).
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

-- Every background job kind's runs (ADR-0117). The run indexes are 00006's.
CREATE TABLE job_runs (
    account_id text NOT NULL REFERENCES accounts,
    -- A short opaque string.
    run_id text NOT NULL,
    -- The job kind.
    workload text NOT NULL,
    -- Never null, so the latest-run reads compare it by equality, which an index serves.
    pass text NOT NULL,
    state text NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
    -- Apply and rollback runs.
    plan_id uuid REFERENCES reorg_plans,
    -- The run this one resumed.
    resumed_from text,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    heartbeat_at timestamptz,
    -- The workload's checkpoint, in the shapes ADR-0016 gives.
    checkpoint jsonb,
    -- The counters per workload, in the shapes ADR-0016 gives.
    counters jsonb NOT NULL DEFAULT '{}',
    -- Provider or scanner text, never a body.
    last_error text,
    PRIMARY KEY (account_id, run_id),
    -- A closed set of the pairs the built job kinds record. Each job kind adds its own pairs in the
    -- migration that brings its role.
    CONSTRAINT job_runs_workload_pass_check CHECK (
        (workload, pass) IN (('backfill', 'pass1'), ('backfill', 'pass2'), ('sync', 'tick'), ('sync', 'gap_recovery'))
    )
);

-- A run's timeline.
CREATE TABLE job_run_events (
    account_id text NOT NULL,
    run_id text NOT NULL,
    seq bigserial,
    -- start | progress | backoff | retry | failure | resume | finish, a vocabulary new job kinds grow,
    -- so not checked.
    kind text NOT NULL,
    at timestamptz NOT NULL DEFAULT now(),
    -- Progress events carry the checkpoint page.
    page int,
    -- Provider or scanner text, never a body.
    detail text,
    PRIMARY KEY (account_id, run_id, seq)
);

-- A run's per-item failures. Its vocabularies grow with new job kinds, so none is checked.
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
