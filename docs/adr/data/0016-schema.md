# 0016. The schema: no body columns anywhere, everything scoped by account

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [G2](../../../USE_CASES.md#g2--historical-understanding), [P3](../../../USE_CASES.md#p3--multi-account), [O2](../../../USE_CASES.md#o2--observable)

## Context

The store is Postgres ([ADR-0015](./0015-postgres-not-a-kv-store.md)); the schema must carry the
metadata corpus, the sender statistics the scan gate needs, the review and approval workflows, rate
coordination, and the audit trail — while making the two structural properties (no bodies at rest,
no cross-account reads) schema-level facts rather than application-level habits.

## Decision

```sql
CREATE TABLE accounts (                  -- what every listing needs, readable in full (ADR-0091)
  account_id        text PRIMARY KEY,
  provider          text NOT NULL
);

CREATE TABLE account_state (             -- everything else an account carries (ADR-0091)
  account_id        text PRIMARY KEY REFERENCES accounts,
  credential        bytea,               -- sealed (ADR-0081, ADR-0088); opaque, its meaning the provider adapter's
                                         -- (an OAuth refresh token for Gmail, an API token for Fastmail, ADR-0012)
  lowered_target_rate real,              -- a lower target the operator set, NULL for none (ADR-0024)
  backfill_pass1_complete boolean NOT NULL DEFAULT false,
  backfill_pass2_complete boolean NOT NULL DEFAULT false,
  sync_cursor       text,
  sync_cursor_at    timestamptz,           -- when sync_cursor was last written
  last_auth_at      timestamptz,           -- last provider authentication attempt
  last_auth_outcome text                   -- its outcome, exposed by ADR-0034
);
-- sync_cursor_at is written by delta sync, last_auth_* by the provider adapter on every
-- authentication attempt; an account's policy overlay is its rows in policy_rules

CREATE TABLE oauth_clients (             -- an installation's OAuth client, for a provider that has one (ADR-0080, ADR-0083); statements in db/oauthclients
  provider          text PRIMARY KEY,
  client_id         text NOT NULL,
  client_secret     bytea NOT NULL       -- sealed (ADR-0081, ADR-0088)
);
-- a provider that authenticates without an OAuth client has no row, and no account refers to one

CREATE TABLE rate_state (                 -- cross-process rate coordination (ADR-0025)
  account_id       text PRIMARY KEY REFERENCES accounts,
  current_rate     real NOT NULL,         -- units/sec, controller-managed
  target_rate      real NOT NULL,         -- the conservative target (ADR-0024), shown only
  hard_cap         real NOT NULL,         -- never exceeded, shown only
  baseline_p50_ms  real,                  -- the latency baseline in use, shown only
  last_throttle_at timestamptz,
  backoff_until    timestamptz,
  throttles        integer NOT NULL DEFAULT 0,  -- throttles since the last success, for the backoff
  bucket_level     real NOT NULL DEFAULT 0,     -- the token bucket's level (ADR-0024)
  bucket_filled_at timestamptz,                 -- the instant that level was reached
  class_asked_at   timestamptz[],               -- when each class last asked, interactive, sync, batch
  last_granted_at  timestamptz,                 -- the instant of the latest grant
  latency_window_start timestamptz,             -- the current one-minute latency window
  latency_samples  integer[],                   -- its samples, in milliseconds
  latency_medians  real[],                      -- the last ten window medians
  classes          jsonb,                  -- {class: {reserved, used}} per priority class, written by the limiter (ADR-0025), shown only
  updated_at       timestamptz NOT NULL DEFAULT now()
);
-- The issuer recomputes target and cap from the provider's declared ceiling on every issue and
-- never reads the shown-only columns back, so no write to the row can raise the cap (ADR-0024)

CREATE TABLE rate_grants (                -- every grant of the last second, which are also the live leases (ADR-0024, ADR-0025)
  grant_id    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  account_id  text NOT NULL REFERENCES accounts,
  class       text NOT NULL,              -- interactive | sync | batch
  tokens      real NOT NULL,
  issued_at   timestamptz NOT NULL        -- the lease expires one second later
);
CREATE INDEX ON rate_grants (account_id, issued_at);
-- One row per grant, written in the same transaction as the bucket's level. The issuer deletes
-- rows more than a second old. A lost row widens issuance without the rules seeing it (ADR-0024)

CREATE TABLE senders (                    -- drives the scan gate, memoization, heuristics
  account_id        text NOT NULL REFERENCES accounts,
  domain            citext NOT NULL,
  local_part_sample text[],
  display_names     text[],
  message_count     bigint NOT NULL DEFAULT 0,
  first_seen        timestamptz,
  last_seen         timestamptz,
  has_list_id_ratio real,                 -- newsletter vs transactional signal
  label_distribution jsonb,
  scan_hit_count    bigint NOT NULL DEFAULT 0,
  sender_class      text NOT NULL DEFAULT 'normal',
  embedding         vector(384),          -- pgvector, heuristic candidates (ADR-0004)
  PRIMARY KEY (account_id, domain)
);

CREATE TABLE messages (
  account_id       text NOT NULL REFERENCES accounts,
  message_id       text NOT NULL,
  thread_id        text NOT NULL,
  from_email       citext NOT NULL,
  from_domain      citext NOT NULL,       -- denormalized: classification hot path
  from_name        text,
  subject          text,                  -- masked at rest if a code was detected
  subject_masked   boolean NOT NULL DEFAULT false,
  sent_at          timestamptz NOT NULL,
  labels           text[] NOT NULL DEFAULT '{}',
  flags            jsonb NOT NULL DEFAULT '{}',
  has_attachments  boolean NOT NULL,
  attachment_types text[] NOT NULL DEFAULT '{}',
  list_id          text,
  size_bytes       int,
  auth_results     jsonb,
  sender_class     text NOT NULL,          -- normal | restricted
  content_flags    text[] NOT NULL DEFAULT '{}',  -- mfa_code | login_link
  rule_ids         text[] NOT NULL DEFAULT '{}',
  scan_state       text NOT NULL DEFAULT 'pending',  -- scanned | skipped_restricted | skipped_gate | pending (ADR-0007)
  scanned_at       timestamptz,
  scanner_version  int,
  PRIMARY KEY (account_id, message_id)
);
-- NOTE: no body, no snippet, no excerpt column. By design.
-- A future migration proposing one is violating the design, not extending it.

CREATE INDEX ON messages (account_id, from_domain);
CREATE INDEX ON messages (account_id, sent_at DESC);
CREATE INDEX ON messages USING gin (labels);
CREATE INDEX ON messages (account_id, sent_at) WHERE labels = '{}';
CREATE INDEX ON messages (account_id) WHERE scan_state = 'pending';
CREATE INDEX ON messages USING gin (subject gin_trgm_ops);

CREATE TABLE scan_gate_decisions (        -- makes ADR-0007's residual auditable
  account_id  text NOT NULL,
  message_id  text NOT NULL,
  decision    text NOT NULL,              -- SCAN | SKIP
  reason      text NOT NULL,              -- restricted | high_volume_no_hits | ...
  decided_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (account_id, message_id)
);

CREATE TABLE policy_candidates (          -- heuristic review queue (ADR-0004)
  account_id   text NOT NULL,
  domain       citext NOT NULL,
  signals      jsonb NOT NULL,            -- one entry per heuristic that fired, with its evidence
  score        real NOT NULL,
  status       text NOT NULL DEFAULT 'pending',  -- pending|confirmed|dismissed
  created_at   timestamptz NOT NULL DEFAULT now(),
  reviewed_at  timestamptz,
  reviewed_by  text,                      -- human only; in the UI's write grant (ADR-0084)
  PRIMARY KEY (account_id, domain)
);

CREATE TABLE policy_rules (               -- the sender policy as rows (ADR-0004), snapshotted by ADR-0041
  account_id     text REFERENCES accounts, -- NULL for the base policy; an overlay names its account
  rule_id        text PRIMARY KEY,        -- candidate.{account}.{domain} when a confirmation minted it
  class          text NOT NULL,           -- restricted
  domain_suffix  text[] NOT NULL,
  source         text NOT NULL,           -- operator | candidate
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text NOT NULL            -- the operator identity; in the UI's write grant (ADR-0084)
);
CREATE INDEX ON policy_rules (account_id);

CREATE TABLE masking_events (
  id          bigserial PRIMARY KEY,
  account_id  text NOT NULL,             -- indexed below with masked_at
  message_id  text NOT NULL,
  field       text NOT NULL,              -- subject
  rule_id     text NOT NULL,
  tier        int NOT NULL,
  masked_at   timestamptz NOT NULL DEFAULT now()
  -- no matched text stored
);
CREATE INDEX ON masking_events (account_id, masked_at DESC);

CREATE TABLE reorg_plans (
  plan_id     uuid PRIMARY KEY,
  account_id  text NOT NULL REFERENCES accounts,
  status      text NOT NULL,              -- DRAFT|APPROVED|APPLYING|APPLIED|ROLLED_BACK|REJECTED|APPLY_REFUSED
  description text,
  proposer    text,                       -- the client actor at creation
  plan        jsonb NOT NULL,             -- label_ops as [{op: create|rename|delete, label, to}], message_ops, stats
  validation       jsonb,                 -- {result: passed|failed, findings: []} at creation (ADR-0032)
  apply_validation jsonb,                 -- same shape, at apply
  refusal_reason   text,                  -- set with APPLY_REFUSED
  created_at  timestamptz NOT NULL DEFAULT now(),
  approved_at timestamptz,                -- the decision time, for REJECTED as well
  approved_by text                        -- human only; in the UI's write grant (ADR-0084)
);
CREATE INDEX ON reorg_plans (account_id, created_at DESC);

CREATE TABLE reorg_plan_ops (             -- the plan's message operations as rows, one per message (ADR-0020)
  account_id    text NOT NULL REFERENCES accounts,
  plan_id       uuid NOT NULL REFERENCES reorg_plans,
  message_id    text NOT NULL,
  add_labels    text[] NOT NULL DEFAULT '{}',
  remove_labels text[] NOT NULL DEFAULT '{}',
  flows         text[] NOT NULL DEFAULT '{}',  -- 'from>to' per (removed or none, added or none) pair
  reason        text,
  PRIMARY KEY (plan_id, message_id)
);
CREATE INDEX ON reorg_plan_ops (account_id, plan_id);

CREATE TABLE reorg_op_log (
  plan_id       uuid NOT NULL REFERENCES reorg_plans,
  seq           bigserial,
  message_id    text NOT NULL,
  labels_before text[] NOT NULL,
  labels_after  text[] NOT NULL,
  applied_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (plan_id, seq)
);

CREATE TABLE job_runs (                   -- every batch workload's runs (ADR-0022)
  account_id    text NOT NULL REFERENCES accounts,
  run_id        text NOT NULL,            -- short opaque string
  workload      text NOT NULL,            -- backfill | sync | apply | heuristics
  pass          text,                     -- pass1 | pass2 | tick | gap_recovery | apply | rollback
  state         text NOT NULL,            -- running | succeeded | failed
  plan_id       uuid REFERENCES reorg_plans,  -- apply and rollback runs
  resumed_from  text,                     -- the run this one resumed
  started_at    timestamptz NOT NULL,
  finished_at   timestamptz,
  heartbeat_at  timestamptz,
  checkpoint    jsonb,                    -- {page, of} or {seq, of}
  counters      jsonb NOT NULL DEFAULT '{}',  -- per workload: pass1 pages, messages; pass2 pages, decided,
                                          --   pending, scanned, skipped; sync added, modified, removed,
                                          --   window_start, window_end, reconciled; apply ops_done,
                                          --   ops_total, failures; heuristics candidates
  last_error    text,                     -- provider or scanner text, never a body
  PRIMARY KEY (account_id, run_id)
);

CREATE TABLE job_run_events (             -- a run's timeline
  account_id text NOT NULL,
  run_id     text NOT NULL,
  seq        bigserial,
  kind       text NOT NULL,               -- start | progress | backoff | retry | failure | resume | finish
  at         timestamptz NOT NULL DEFAULT now(),
  page       int,                         -- progress events carry the checkpoint page
  detail     text,                        -- provider or scanner text, never a body
  PRIMARY KEY (account_id, run_id, seq)
);

CREATE TABLE job_run_failures (           -- a run's per-item failures
  account_id    text NOT NULL,
  run_id        text NOT NULL,
  seq           bigserial,
  item_kind     text NOT NULL,            -- page | message | op
  item_id       text NOT NULL,            -- the page number, or the message id for message and op items
  page          int,                      -- the page the item was processed on
  error_class   text NOT NULL,            -- throttled | provider_error | gone | scanner_timeout | validation | authentication
  error_summary text,                     -- provider or scanner text, never a body
  attempts      int NOT NULL DEFAULT 1,
  first_at      timestamptz NOT NULL,
  last_at       timestamptz NOT NULL,
  disposition   text NOT NULL DEFAULT 'pending',  -- recovered | pending | gone | abandoned
  recovered_by  text,                     -- the recovering run
  PRIMARY KEY (account_id, run_id, seq)
);

CREATE TABLE audit_log (
  id          bigserial PRIMARY KEY,
  ts          timestamptz NOT NULL DEFAULT now(),
  account_id  text NOT NULL,
  actor       text NOT NULL,
  action      text NOT NULL,              -- READ_BODY|DENY_BODY|MUTATE|DENY_MUTATE
  message_id  text,
  sensitivity jsonb,
  rule_ids    text[]
);
-- No runtime role holds UPDATE or DELETE here. Append-only is the grant, not a convention.
CREATE INDEX ON audit_log (account_id, ts DESC);
```

The properties the shape enforces:

- **No body, snippet, or excerpt column exists anywhere.** The comment in the DDL is part of the
  decision: a future migration adding one is violating the design, not extending it.
- **Every table keys on `account_id`.** All access goes through a repository layer that requires
  an account, every statement against an account-keyed table carries an account predicate
  ([ADR-0047](./0047-schema-first-data-access.md)), and row-level security stands behind both as a
  third, independent layer. The policies read the transaction-local setting `app.account`, which
  every process sets before reading, by an ordinary statement and never by database-resident code
  ([ADR-0060](../engineering/0060-no-code-in-the-database.md)). Four exceptions are stated.
  - `accounts` holds only each account's identifier and provider and is read in full by the roles
    that list accounts, while its writes stay confined
    ([ADR-0091](./0091-accounts-listed-apart-from-their-state.md)).
  - `oauth_clients` belongs to no account and carries no account column, so grants alone decide
    who reaches it. The four roles that call a provider read it in full. The UI's role reads its
    `provider` and `client_id`, since it runs the consent with the client and sets a client up
    again, and never reads its `client_secret`. Only the UI's role, when a client is set up, and
    delta sync's, when it re-seals a secret, write it
    ([ADR-0080](./0080-accounts-and-credentials-live-in-the-database.md),
    [ADR-0083](../provider/0083-gmail-through-an-installation-oauth-client.md),
    [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md),
    [ADR-0092](../operability/0092-key-replacement-by-keyring-and-re-seal.md)).
  - `policy_rules` rows with a null account are the base policy every account inherits
    ([ADR-0004](../classification/0004-sender-list-decides.md)).
  - `reorg_op_log` carries no account column at all and is scoped through its plan, by a policy
    whose predicate reaches the plan's account. That policy's parent lookup is evaluated once per
    statement rather than once per row, so scoping it costs materially less than the foreign key
    the same writes already carry.
- **The deny state of that third layer is silent, and the application compensates.** Once a
  connection has set `app.account`, Postgres keeps the parameter known and resets it to the empty
  string between transactions rather than to unrecognised. A statement that then omits the setting
  raises on a connection drawn fresh and returns no rows without error on one drawn warm, so under
  a pool the behaviour depends on which connection was drawn. No policy expression can raise,
  because [ADR-0060](../engineering/0060-no-code-in-the-database.md) bars the database-resident
  function that would. The shared transaction helper therefore reads the setting back after
  setting it and fails the transaction when it is empty. The general asymmetry behind this is
  worth carrying. An insert a policy refuses raises, while a select or update a policy empties
  returns quietly.
- **The stored spellings of every enumerated column are the ones the DDL comments show**, in
  lowercase snake case where a record names the state in capitals (`skipped_gate` for
  [ADR-0007](../redaction/0007-composite-scan-gate.md)'s `SKIPPED_GATE`). Plan statuses keep
  their capitals as [ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md) writes
  them.
- **The batch workloads' runs, timeline events, and per-item failures are rows**
  ([ADR-0022](../operability/0022-four-workloads.md)), and their free-text columns hold provider
  or scanner text and never a body, under the same comment that binds every table.
- **The UI's decisions are recorded in the columns its verbs set and the rule row its confirm
  verb inserts** ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)), so a
  decision is readable from `reorg_plans` and `policy_candidates` without an audit row.
- **Masked subjects are stored masked** — the index never holds a live code.
- **The partial indexes target unfiled volume** (`labels = '{}'`) **and scan backlog**
  (`scan_state = 'pending'`) directly.
- **The audit log is append-only to every runtime role.** No runtime role holds update or delete
  on it, whichever component writes it. This is a property of the table's grants rather than of
  any one role, so it keeps holding as components are added, and it is what makes evidence written
  before a compromise survive that compromise without depending on anything outside the cluster
  ([ADR-0028](../operability/0028-trust-anchor-hardening.md)). It bounds rather than absolutises
  the claim, because an attacker holding the process governs what is written from that moment on.

## Alternatives considered

- **Store snippets/bodies encrypted "for convenience features later."** Rejected: it converts the
  structural guarantee into a key-management promise, and every later feature idea ("preview",
  "search inside bodies") would pull on it. Absence is the feature.
- **A single-tenant schema, multi-account by deploying more instances.** Rejected: the isolation
  property must hold *inside* one deployment (see the account model,
  [ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md));
  per-instance separation is an operational choice layered on top, not a substitute.
- **Denormalize sender statistics into `messages`.** Rejected: the gate and heuristics read
  sender-level aggregates constantly; a `senders` table keeps those reads cheap and their updates
  batched.

## Consequences

- The plan's message operations are held twice, as the plan JSON the client submits and as the
  `reorg_plan_ops` rows the engine writes when the plan is saved, so the UI can group tens of
  thousands of operations by flow, sender, and sensitivity in SQL
  ([ADR-0056](../operability/0056-ui-organized-around-the-operators-work.md)).
- The corpus is assumed to stay on the order of 100k messages per account, and nothing is
  partitioned, because at that size partitioning earns nothing that the account predicate and
  row-level security do not already carry. Millions would not change the store choice but would
  make partitioning, retention, and backfill planning real design work rather than a schema
  detail, and would also make the enumerated statement shape of
  [ADR-0066](./0066-data-access-generated-from-sql.md) stop being free. That assumption is
  recorded as a known limit in [DESIGN.md](../../../DESIGN.md#3-known-limits).
- Retention of the audit log has no owner while no runtime role can delete from it. Nothing in the
  running system trims the table, and whether it is ever trimmed is an open decision in
  [ROADMAP.md](../../../ROADMAP.md).
- Schema evolution is by migration; the DDL's no-body comment binds every future one.
