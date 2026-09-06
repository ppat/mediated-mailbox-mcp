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
CREATE TABLE accounts (
  account_id        text PRIMARY KEY,
  provider          text NOT NULL,
  backfill_pass1_complete boolean NOT NULL DEFAULT false,
  backfill_pass2_complete boolean NOT NULL DEFAULT false,
  sync_cursor       text,
  policy_overlay    text
);

CREATE TABLE rate_state (                 -- cross-process rate coordination (ADR-0025)
  account_id       text PRIMARY KEY REFERENCES accounts,
  current_rate     real NOT NULL,         -- units/sec, controller-managed
  target_rate      real NOT NULL,         -- the conservative target (ADR-0024)
  hard_cap         real NOT NULL,         -- never exceeded
  baseline_p50_ms  real,                  -- for latency-based decrease
  last_throttle_at timestamptz,
  backoff_until    timestamptz,
  leased_tokens    real NOT NULL DEFAULT 0,
  lease_expires_at timestamptz,
  updated_at       timestamptz NOT NULL DEFAULT now()
);

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
  sender_class     text NOT NULL,
  content_flags    text[] NOT NULL DEFAULT '{}',
  rule_ids         text[] NOT NULL DEFAULT '{}',
  scan_state       text NOT NULL DEFAULT 'pending',
  scanned_at       timestamptz,
  scanner_version  int,
  PRIMARY KEY (account_id, message_id)
) PARTITION BY LIST (account_id);
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
  signals      jsonb NOT NULL,            -- which heuristics fired + evidence
  score        real NOT NULL,
  status       text NOT NULL DEFAULT 'pending',  -- pending|confirmed|dismissed
  reviewed_at  timestamptz,
  PRIMARY KEY (account_id, domain)
);

CREATE TABLE masking_events (
  account_id  text NOT NULL,
  message_id  text NOT NULL,
  field       text NOT NULL,              -- subject
  rule_id     text NOT NULL,
  tier        int NOT NULL,
  masked_at   timestamptz NOT NULL DEFAULT now()
  -- no matched text stored
);

CREATE TABLE reorg_plans (
  plan_id     uuid PRIMARY KEY,
  account_id  text NOT NULL REFERENCES accounts,
  status      text NOT NULL,              -- DRAFT|APPROVED|APPLYING|APPLIED|ROLLED_BACK
  description text,
  plan        jsonb NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  approved_at timestamptz,
  approved_by text                        -- human only; in the UI's write grant (ADR-0021)
);

CREATE TABLE reorg_op_log (
  plan_id       uuid NOT NULL REFERENCES reorg_plans,
  seq           bigserial,
  message_id    text NOT NULL,
  labels_before text[] NOT NULL,
  labels_after  text[] NOT NULL,
  applied_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (plan_id, seq)
);

CREATE TABLE audit_log (
  ts          timestamptz NOT NULL DEFAULT now(),
  account_id  text NOT NULL,
  actor       text NOT NULL,
  action      text NOT NULL,              -- READ_BODY|DENY_BODY|MUTATE|DENY_MUTATE
  message_id  text,
  sensitivity jsonb,
  rule_ids    text[]
) PARTITION BY RANGE (ts);
```

The properties the shape enforces:

- **No body, snippet, or excerpt column exists anywhere.** The comment in the DDL is part of the
  decision: a future migration adding one is violating the design, not extending it.
- **Every table keys on `account_id`;** `messages` is list-partitioned by it. All access goes
  through a repository layer that requires an account, with row-level security as a second,
  independent layer.
- **Masked subjects are stored masked** — the index never holds a live code.
- **The partial indexes target unfiled volume** (`labels = '{}'`) **and scan backlog**
  (`scan_state = 'pending'`) directly.
- **The audit log is range-partitioned by time**, for durable retention.

## Alternatives considered

- **Store snippets/bodies encrypted "for convenience features later."** Rejected: it converts the
  structural guarantee into a key-management promise, and every later feature idea ("preview",
  "search inside bodies") would pull on it. Absence is the feature.
- **A single-tenant schema, multi-account by deploying more instances.** Rejected: the isolation
  property must hold *inside* one deployment (see the account model,
  [ADR-0026](../provider/0026-multi-account-contexts.md)); per-instance separation is an
  operational choice layered on top, not a substitute.
- **Denormalize sender statistics into `messages`.** Rejected: the gate and heuristics read
  sender-level aggregates constantly; a `senders` table keeps those reads cheap and their updates
  batched.

## Consequences

- The corpus is assumed to stay on the order of 100k messages per account. Millions would not
  change the store choice but would make partitioning, retention, and backfill planning real
  design work rather than a schema detail — recorded as a known limit in
  [DESIGN.md](../../../DESIGN.md#3-known-limits).
- Schema evolution is by migration; the DDL's no-body comment binds every future one.
