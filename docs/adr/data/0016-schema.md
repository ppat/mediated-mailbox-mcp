# 0016. The schema: no body columns anywhere, everything scoped by account

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [G2](../../../USE_CASES.md#g2--historical-understanding), [P3](../../../USE_CASES.md#p3--multi-account), [O2](../../../USE_CASES.md#o2--observable)

## Context

The store is Postgres ([ADR-0015](./0015-postgres-not-a-kv-store.md)). The schema must carry the
metadata corpus, the sender statistics the scan gate needs, the review and approval workflows, rate
coordination, and the audit trail, while making the two structural properties (no bodies at rest,
no cross-account reads) schema-level facts rather than application-level habits.

## Decision

```sql
CREATE TABLE oauth_clients (             -- the installation's OAuth clients, any number for a provider that has them (ADR-0080, ADR-0106, ADR-0107); statements in db/oauthclients
  name              text PRIMARY KEY,    -- given by the person running the installation, unique in it (ADR-0106)
  provider          text NOT NULL,
  client_id         text NOT NULL,
  client_secret     bytea NOT NULL,      -- sealed (ADR-0081, ADR-0088)
  project_id        text,                -- the provider's project the client belongs to, Google Cloud's
                                         -- project ID for Gmail, written by OAuth client setup;
                                         -- NULL when the setup was given none. Not secret
  UNIQUE (name, provider),               -- what an account's reference names, so it carries the provider
  UNIQUE (provider, client_id)
);
-- a provider that authenticates without an OAuth client has no row, and no account refers to one

CREATE TABLE accounts (                  -- what every listing needs, readable in full (ADR-0091)
  account_id        text PRIMARY KEY,
  provider          text NOT NULL,
  oauth_client      text,                -- the client the account connects through, NULL for a provider
                                         -- without one (ADR-0106)
  FOREIGN KEY (oauth_client, provider) REFERENCES oauth_clients (name, provider)
);
-- the reference carries the provider, so an account cannot name another provider's client, and a
-- client an account connects through cannot be removed (ADR-0106)

CREATE TABLE account_state (             -- everything else an account carries (ADR-0091)
  account_id        text PRIMARY KEY REFERENCES accounts,
  credential        bytea,               -- sealed (ADR-0081, ADR-0088); opaque, its meaning the provider adapter's
                                         -- (an OAuth refresh token for Gmail, an API token for Fastmail, ADR-0012)
  mailbox           text,                -- the mailbox address the account remembers; re-authorization
                                         -- refuses a credential for any other (ADR-0080)
  lowered_target_rate real,              -- a lower target the operator set, a fraction of the provider's
                                         -- declared ceiling, NULL for none (ADR-0024)
  backfill_pass1_complete boolean NOT NULL DEFAULT false,
  backfill_pass2_complete boolean NOT NULL DEFAULT false,
  backfill_pass2_restart  boolean NOT NULL DEFAULT false,  -- the next pass 2 run starts over (ADR-0120)
  sync_cursor       text,
  sync_cursor_at    timestamptz,           -- when sync_cursor was last written
  last_auth_at      timestamptz,           -- latest authentication attempt recorded (ADR-0097)
  last_auth_outcome text                   -- succeeded | refused | failed, exposed by ADR-0034
    CHECK (last_auth_outcome IN ('succeeded', 'refused', 'failed'))
);
-- sync_cursor_at is written by delta sync, last_auth_* by each provider-calling deployable from
-- what its provider adapter reports, and by the UI from the code exchange of a consent it
-- completes, as ADR-0097 decides; an account's policy overlay is its rows in policy_rules

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
  class       text NOT NULL CHECK (class IN ('interactive', 'sync', 'batch')),
  tokens      real NOT NULL,
  issued_at   timestamptz NOT NULL        -- the lease expires one second later
);
CREATE INDEX ON rate_grants (account_id, issued_at);
-- One row per grant, written in the same transaction as the bucket's level. The issuer deletes
-- rows more than a second old. A lost row widens issuance without the rules seeing it (ADR-0024)

CREATE TABLE senders (                    -- drives the scan gate and the heuristics
  account_id        text NOT NULL REFERENCES accounts,
  domain            text NOT NULL CHECK (domain = lower(domain COLLATE "C")),
  local_part_sample text[],
  display_names     text[],
  message_count     bigint NOT NULL DEFAULT 0,
  first_seen        timestamptz,
  last_seen         timestamptz,
  has_list_id_ratio real,                 -- newsletter vs transactional signal
  label_distribution jsonb,
  scan_hit_count    bigint NOT NULL DEFAULT 0,
  sender_class      text NOT NULL DEFAULT 'normal' CHECK (sender_class IN ('normal', 'restricted')),
  PRIMARY KEY (account_id, domain)
);

CREATE TABLE messages (
  account_id       text NOT NULL REFERENCES accounts,
  message_id       text NOT NULL,
  thread_id        text NOT NULL,
  from_email       citext NOT NULL,
  from_domain      text NOT NULL          -- denormalized: classification hot path
    CHECK (from_domain = lower(from_domain COLLATE "C")),
  from_name        text,
  subject          text,                  -- masked at rest if a code was detected
  subject_masked   boolean NOT NULL DEFAULT false,
  subject_scanner_version  int,           -- the scanner version the subject was masked under (ADR-0120)
  subject_scanner_revision text,          -- and the scanner configuration's revision
  sent_at          timestamptz NOT NULL,
  labels           text[] NOT NULL DEFAULT '{}',
  flags            jsonb NOT NULL DEFAULT '{}',
  has_attachments  boolean NOT NULL,
  list_id          text,
  size_bytes       int,
  auth_results     jsonb,
  sender_class     text NOT NULL CHECK (sender_class IN ('normal', 'restricted')),
  content_flags    text[] NOT NULL DEFAULT '{}'
    CHECK (content_flags <@ '{mfa_code,login_link}'),
  rule_ids         text[] NOT NULL DEFAULT '{}',  -- the content rules that set content_flags
  class_rule_id    text,                   -- the identifier of the policy rule that set sender_class, NULL when none
                                           -- did; an identifier, which two scopes may hold (ADR-0110)
  scan_state       text NOT NULL DEFAULT 'pending'   -- ADR-0093's states
    CHECK (scan_state IN ('scanned', 'skipped_restricted', 'skipped_gate', 'pending')),
  scanned_at       timestamptz,
  scanner_version  int,
  scanner_revision text,                   -- the scanner configuration's revision (ADR-0009)
  PRIMARY KEY (account_id, message_id)
);
-- NOTE: no body, no snippet, no excerpt column. By design.
-- A future migration proposing one is violating the design, not extending it.

CREATE INDEX ON messages (account_id, from_domain);
CREATE INDEX ON messages (account_id, sent_at DESC);
CREATE INDEX ON messages (account_id, sent_at) WHERE labels = '{}';
CREATE INDEX ON messages (account_id) WHERE scan_state = 'pending';
-- no index over labels or subject: no leakproof comparison serves either under row-level security

CREATE TABLE attachment_media (           -- the inputs of a message's attachment types (ADR-0123)
  account_id  text NOT NULL,
  message_id  text NOT NULL,
  media_type  text NOT NULL               -- lowercased, without parameters, '' when malformed
    CHECK (media_type ~ '^([a-z0-9][a-z0-9!#$&^_.+-]{0,126}/[a-z0-9][a-z0-9!#$&^_.+-]{0,126})?$'),
  extension   text NOT NULL               -- the filename's last extension, lowercased, '' when none or unusable
    CHECK (extension ~ '^[a-z0-9]{0,16}$'),
  PRIMARY KEY (account_id, message_id, media_type, extension),
  FOREIGN KEY (account_id, message_id) REFERENCES messages ON DELETE CASCADE
);
-- NOTE: no filename column. The filename is body-derived and is never stored (ADR-0001).

CREATE TABLE scan_gate_decisions (        -- makes ADR-0093's residual auditable
  account_id  text NOT NULL,
  message_id  text NOT NULL,
  decision    text NOT NULL CHECK (decision IN ('SCAN', 'SKIP')),
  reason      text NOT NULL,              -- restricted | high_volume_no_hits | ...
  decided_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (account_id, message_id)
);

CREATE TABLE policy_candidates (          -- heuristic review queue (ADR-0004)
  account_id   text NOT NULL,
  domain       text NOT NULL CHECK (domain = lower(domain COLLATE "C")),
  signals      jsonb NOT NULL,            -- one entry per heuristic that fired, with its evidence, as
                                          --   [{heuristic, evidence}], heuristic one of display_name,
                                          --   domain_clustering, institution_keyword, transactional_pattern,
                                          --   embedding_similarity (ADR-0004's heuristics in its table's
                                          --   order), and evidence per heuristic: display_name name, domain;
                                          --   domain_clustering domain; institution_keyword keyword;
                                          --   transactional_pattern none, since it fires only when its three
                                          --   facts all hold; embedding_similarity domain, score.
                                          --   domain is the listed domain the display name matched
                                          --   (display_name), the listed domain the sender clusters with
                                          --   (domain_clustering), or the nearest confirmed sender's domain
                                          --   (embedding_similarity); score is the similarity against the
                                          --   confirmed-sensitive centroid ADR-0004 compares with
  score        real NOT NULL,
  status       text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'confirmed', 'dismissed')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  reviewed_at  timestamptz,
  reviewed_by  text,                      -- human only; in the UI's write grant (ADR-0084)
  PRIMARY KEY (account_id, domain)
);

CREATE TABLE policy_rules (               -- the sender policy as rows (ADR-0004), snapshotted by ADR-0041
  account_id     text REFERENCES accounts, -- NULL for the base policy; an overlay names its account
  rule_id        text NOT NULL,           -- unique within its scope (ADR-0110); candidate.{account}.{domain}
                                          -- when a confirmation minted it
  class          text NOT NULL CHECK (class IN ('restricted')),
  domain_suffix  text[] NOT NULL,
  source         text NOT NULL CHECK (source IN ('operator', 'candidate')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text NOT NULL,           -- the operator identity; in the UI's write grant (ADR-0084)
  UNIQUE NULLS NOT DISTINCT (account_id, rule_id)  -- the base policy is one scope, each account another
);
CREATE INDEX ON policy_rules (account_id);

CREATE TABLE policy_changes (             -- every change to policy_rules, appended in the same transaction (ADR-0102)
  id             bigserial PRIMARY KEY,
  account_id     text REFERENCES accounts, -- NULL for a base rule's change; an overlay rule's names its account
  ts             timestamptz NOT NULL DEFAULT now(),
  actor          text NOT NULL,           -- the operator identity (ADR-0084)
  action         text NOT NULL CHECK (action IN ('added', 'edited', 'lifted', 'confirmed')),
  rule_id        text NOT NULL,
  suffixes_before text[] NOT NULL DEFAULT '{}',  -- empty for added and confirmed
  suffixes_after  text[] NOT NULL DEFAULT '{}'   -- empty for lifted
);
CREATE INDEX ON policy_changes (account_id, ts DESC);
-- No runtime role holds UPDATE or DELETE here, as on audit_log.

CREATE TABLE masking_events (
  id          bigserial PRIMARY KEY,
  account_id  text NOT NULL,             -- indexed below with masked_at
  message_id  text NOT NULL,
  field       text NOT NULL CHECK (field IN ('subject')),
  rule_id     text NOT NULL,
  tier        int NOT NULL,
  scanner_version  int,                   -- the scanner the masking ran under (ADR-0120)
  scanner_revision text,
  masked_at   timestamptz NOT NULL DEFAULT now()
  -- no matched text stored
);
CREATE INDEX ON masking_events (account_id, masked_at DESC);

CREATE TABLE reorg_plans (
  plan_id     uuid PRIMARY KEY,
  account_id  text NOT NULL REFERENCES accounts,
  status      text NOT NULL CHECK (status IN ('DRAFT', 'APPROVED', 'APPLYING', 'APPLIED',
                'ROLLED_BACK', 'REJECTED', 'APPLY_REFUSED')),  -- ADR-0020's set; rollback's own status
                                          -- is added in the migration that brings rollback
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

CREATE TABLE job_runs (                   -- every background job kind's runs (ADR-0117)
  account_id    text NOT NULL REFERENCES accounts,
  run_id        text NOT NULL,            -- short opaque string
  workload      text NOT NULL,            -- the job kind: backfill | sync | apply | heuristics
  pass          text NOT NULL,            -- pass1 | pass2 | tick | gap_recovery | apply | rollback, and the
                                          --   one heuristics names in the migration that brings its role
  state         text NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
  plan_id       uuid REFERENCES reorg_plans,  -- apply and rollback runs
  resumed_from  text,                     -- the run this one resumed
  started_at    timestamptz NOT NULL,
  finished_at   timestamptz,
  heartbeat_at  timestamptz,
  checkpoint    jsonb,                    -- {page, of} or {seq, of}, and backfill's pass 1 records {page, token,
                                          --   of, version, revision, stale}, the provider's token for the next
                                          --   page, with of only when its enumeration reports a total (ADR-0095),
                                          --   the scanner it masks under, and once its enumeration has ended how
                                          --   many stored subjects are still masked under another scanner, left
                                          --   out when none is (ADR-0120), and pass 2 {page, after}, the last message
                                          --   identifier its pages read, and a sync tick {after}, the last
                                          --   message identifier its scanning read (ADR-0104)
  counters      jsonb NOT NULL DEFAULT '{}',  -- per workload: pass1 pages, messages, remasked, refetched; pass2 pages,
                                          --   decided, pending, scanned, skipped; sync added, modified, removed,
                                          --   window_start, window_end, reconciled, and a tick's decided,
                                          --   pending, scanned, skipped (ADR-0104, ADR-0105); apply ops_done,
                                          --   ops_total, failures; heuristics candidates
  last_error    text,                     -- provider or scanner text, never a body
  PRIMARY KEY (account_id, run_id),
  CHECK ((workload, pass) IN (('backfill', 'pass1'), ('backfill', 'pass2'),
                              ('sync', 'tick'), ('sync', 'gap_recovery')))
                                          -- a closed set; each job kind adds its own pairs
                                          --   in the migration that brings its role
);
CREATE INDEX ON job_runs (account_id, workload, pass, started_at DESC, run_id);
CREATE INDEX ON job_runs (account_id) WHERE state = 'running';
CREATE INDEX ON job_runs (account_id, finished_at);
CREATE INDEX ON job_runs (account_id, started_at DESC);

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
  id               bigserial PRIMARY KEY,
  ts               timestamptz NOT NULL DEFAULT now(),
  account_id       text NOT NULL,
  actor            text NOT NULL,
  action           text NOT NULL
    CHECK (action IN ('READ_BODY', 'DENY_BODY', 'MUTATE', 'DENY_MUTATE')),
  message_id       text,
  -- what a body decision rests on (ADR-0001, ADR-0002)
  stage            text CHECK (stage IN ('gate', 'serve')),
  reason           text,                  -- closed and checked: the reasons a body decision gives,
                                          --   released included, whose spellings are settled with
                                          --   the reasons the client surface returns
  sender_class     text CHECK (sender_class IN ('normal', 'restricted')),
  class_rule_id    text,                  -- the policy rule behind sender_class
  class_rule_scope text CHECK (class_rule_scope IN ('base', 'account')),
  content_flags    text[] CHECK (content_flags <@ '{mfa_code,login_link}'),
  content_rule_ids text[],                -- the scanner rules behind the stored flags
  serve_rule_ids   text[],                -- the serve-time pattern check's rules, on a serve-time denial
  scan_state       text
    CHECK (scan_state IN ('scanned', 'skipped_restricted', 'skipped_gate', 'pending')),
  scanner_version  int,                   -- the scanner the serve-time check ran under
  scanner_revision text,
  CHECK (action NOT IN ('READ_BODY', 'DENY_BODY')
         OR (reason IS NOT NULL AND sender_class IS NOT NULL AND scan_state IS NOT NULL)),
  CHECK ((class_rule_id IS NULL) = (class_rule_scope IS NULL))
);
-- No runtime role holds UPDATE or DELETE here. Append-only is the grant, not a convention.
-- A mutation's own columns are added in the migration that brings mutation, and rows written before keep their meaning.
CREATE INDEX ON audit_log (account_id, ts DESC);
CREATE INDEX ON audit_log (account_id, message_id, ts DESC);
```

The shape enforces these properties.

- **No body, snippet, or excerpt column exists anywhere.** The comment in the DDL is part of the
  decision. A future migration adding one is violating the design, not extending it. An
  attachment's filename is not stored either. Its media type and extension are, in
  `attachment_media`, normalized to a short, inert form. They are body-derived sender text, held
  only as the input of
  [ADR-0123](../provider/0123-attachment-types-are-words-of-a-closed-vocabulary.md)'s mapping,
  which derives the types a client sees when read, and never served or matched raw, as that record
  states.
- **Every table keys on `account_id`.** All access goes through a repository layer that requires
  an account, every statement against an account-keyed table carries an account predicate
  ([ADR-0047](./0047-schema-first-data-access.md)), and row-level security stands behind both as a
  third, independent layer. The policies read the transaction-local setting `app.account`, which
  every process sets before reading, by an ordinary statement and never by database-resident code
  ([ADR-0060](../engineering/0060-no-code-in-the-database.md)). Four exceptions are stated.
  - `accounts` holds only each account's identifier, provider and the OAuth client it connects
    through, and is read in full by the roles that list accounts, while its writes stay confined
    ([ADR-0091](./0091-accounts-listed-apart-from-their-state.md)).
  - `oauth_clients` belongs to no account and carries no account column, so grants alone decide
    who reaches it. The four roles that call a provider read it in full. The UI's role reads it in
    full too, its `name`, `provider`, `client_id` and `project_id` since it runs the consent with a
    client, sets a client up again and shows where each client lives, and its sealed
    `client_secret` for the one part of the UI that opens it, for a consent's code exchange
    ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)). Only
    the UI's role, when a client is set up, and delta sync's, when it re-seals a secret, write it
    ([ADR-0080](./0080-accounts-and-credentials-live-in-the-database.md),
    [ADR-0106](../provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md),
    [ADR-0107](../provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md),
    [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md),
    [ADR-0092](../operability/0092-key-replacement-by-keyring-and-re-seal.md)).
  - `policy_rules` rows with a null account are the base policy every account inherits
    ([ADR-0004](../classification/0004-sender-list-decides.md)), and `policy_changes` rows with a
    null account are the changes to it
    ([ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)). Every
    account reads them, and only the UI's role writes them, in a transaction that names no account
    and sets the base policy's own setting, `app.base`
    ([ADR-0112](./0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)).
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
  setting it and fails the transaction when it is empty. Its one transaction whose account is
  deliberately empty is the base policy's, which runs only the base policy's statements
  ([ADR-0112](./0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)). The
  asymmetry behind this holds generally. An insert a policy refuses raises, while a select or
  update a policy empties returns quietly.
- **The stored spellings of every enumerated column are the ones the DDL shows**, in
  lowercase snake case where a record names the state in capitals (`skipped_gate` for
  [ADR-0093](../redaction/0093-composite-scan-gate.md)'s `SKIPPED_GATE`). Plan statuses keep
  their capitals as [ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md) writes
  them.
- **Closed vocabularies are checked, in three tiers.** A check is a constraint, not code in the
  database ([ADR-0060](../engineering/0060-no-code-in-the-database.md)), and each check standing in
  for a rule costs a test and its mutation demonstration once.
  - *Irreversible if a wrong spelling slips*: the audit row's closed columns and `policy_changes`'
    action. Both tables are append-only, so a wrong spelling written there stays for good. A new
    token costs one migration that replaces the constraint in the token's own release, written as
    `NOT VALID` then `VALIDATE` so inserts keep flowing, and widening a set never fails validation.
  - *Asked for by a reader*: `job_runs`' job kind and pass as a closed set of pairs, and its state.
    A job kind arrives with a role and grants anyway, so extending the check rides that migration.
  - *Cheap, next to safety*: the sender class, scan state and content flags of `messages`, the
    sender class of `senders`, the gate's decision, the last authentication outcome, a rule's class
    and source, a candidate's and a plan's status, a grant's class and a masking event's field.
    Readers already fail closed on an unknown value, so the gain is a loud write failure in place of
    quiet over-redaction. The columns of `attachment_media` sit in this tier too, though theirs is
    a shape rather than a vocabulary. Each check holds its column to the normalized form
    [ADR-0123](../provider/0123-attachment-types-are-words-of-a-closed-vocabulary.md) states, so a
    writer that bypassed the normalization cannot store longer or other text the sender wrote.
  - *Not checked*: `accounts.provider`, since a check would put a storage change in every new
    backend ([P2](../../../USE_CASES.md#p2--backend-swap)), and the vocabularies tuning and new job
    kinds grow on rebuildable rows an update can fix, the gate decision's reason, a run event's
    kind, a failure's class, item kind and disposition, and a masking event's tier.
- **Sender domains are stored as lowercase `text`.** Every stored domain is written through one
  normalizer in Go, `index.StoredDomain`, and every caller that binds a domain to a statement binds
  its output. The normalizer applies Go's simple lowercase mapping, with U+0130 (`İ`) taken to `i`
  followed by U+0307 as Unicode's SpecialCasing.txt does, the one unconditional entry there whose
  lowercase differs from the simple mapping's `i`. It applies none of that file's conditional
  entries, such as the final sigma, because the sender classifier's UTS #46 mapping
  ([ADR-0004](../classification/0004-sender-list-decides.md)) maps each character the same wherever
  it stands and takes no language, and takes U+0130 the same way. So a stored domain classifies as
  the address it came from, which classifying senders by domain relies on
  ([ADR-0108](../operability/0108-index-reads-select-by-an-index-query-of-the-surfaces-own.md)).
  No statement folds a domain's case in SQL, because PostgreSQL's `lower()` differs from Go's and
  depends on the database's collation, so a statement that lowered in SQL would match some stored
  values on one database and miss them on another, and with one spelling of each domain stored a
  grouping has nothing for it to merge. The parse-tree pass over the statement files refuses the
  forms its scope names ([db/README.md](../../../db/README.md#the-checks)), and review holds the
  rest of the rule. The check `x = lower(x COLLATE "C")` is a guard, not the definition. It refuses
  an ASCII capital, the plausible bug of a writer skipping the normalizer, and means the same on
  every database, since the `C` collation folds only A to Z, and it refuses nothing the normalizer
  produces. A domain written in punycode is stored in punycode and one written in Unicode in
  Unicode. Case-insensitive text stays only where display case must be kept and matching needs no
  index, which is `from_email`.
- **The audit row records what a body decision rests on, in typed columns**: the stage that
  decided, the reason, the sender class with the rule behind it and that rule's scope, the content
  flags with the scanner rules behind them kept apart from the serve-time check's, the scan state,
  and the scanner the serve-time check ran under. On evidence no runtime role can ever correct, a
  closed vocabulary fails at insert, and a check reads a column plainly. The mediator writes the
  audit row before it releases anything, so a token the check does not know fails the request
  closed and loudly and never serves a body without its record.
- **The background job kinds' runs, timeline events, and per-item failures are rows**
  ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)), and their free-text columns hold provider
  or scanner text and never a body, under the same comment that binds every table.
- **The UI's decisions are recorded in the columns its verbs set and the rule row its confirm
  verb inserts** ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)), so a
  decision is readable from `reorg_plans` and `policy_candidates` without an audit row.
- **A message names the rule behind each sensitivity axis apart.** A message's `class_rule_id` is
  the identifier of the policy rule that set its `sender_class`, which the base policy and the
  account may both hold ([ADR-0004](../classification/0004-sender-list-decides.md),
  [ADR-0110](../mutation/0110-a-policy-file-holds-one-scope-and-importing-it-replaces-that-scope.md)),
  written when the message is stored.
  It is NULL when no rule set the class, which is a sender no rule lists, a classification made
  while no policy has loaded, and an address that cannot be classified. A message's `rule_ids` are
  the content rules that set its `content_flags`, written by the scan
  ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md)). Whatever writes a message's
  `sender_class` writes its `class_rule_id` with it, so the delisting transition clears the rule
  with the class it resets ([ADR-0037](../redaction/0037-delisting-transition.md)). The sender
  statistics' `sender_class` in `senders` carries no rule. Otherwise a message's `class_rule_id`
  stays as written, like the class it explains, so a rule edited or replaced while its domain stays
  restricted leaves the old identifier. A message stored before the migration that adds
  `class_rule_id` holds NULL as well, and nothing fills it in.
- **Masked subjects are stored masked** — the index never holds a live code.
- **The partial indexes target unfiled volume** (`labels = '{}'`) **and scan backlog**
  (`scan_state = 'pending'`) directly. A partial index serves a runtime role when the statement
  states its predicate exactly, even where the predicate's operator is not leakproof.
- **The run indexes serve the reads that repeat**, every tick's latest run, the jobs screen's
  latest run per job kind and pass, the event stream, the runs dataset, and the job mechanism's due
  decisions, and the audit index serves a message's audit rows. `job_runs.pass` is never null, so
  the latest-run reads compare it by equality, which an index serves.
- **The policy history is append-only to every runtime role**, for the same reason as the audit
  log below, so who lifted a restriction and when survives the UI's compromise
  ([ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)).
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
- **A single-tenant schema, multi-account by deploying more instances.** No case was tabled for it.
  Rejected: the isolation property must hold *inside* one deployment (see the account model,
  [ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)). Per-instance
  separation is an operational choice layered on top, not a substitute.
- **Denormalize sender statistics into `messages`.** No case was tabled for it. Rejected: the gate
  and heuristics read sender-level aggregates constantly, and a `senders` table keeps those reads
  cheap and their updates batched.

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
- Schema evolution is by migration, and the DDL's no-body comment binds every future one.
- **Row-level security costs indexes.** PostgreSQL uses a predicate as an index condition ahead of
  a policy only when every function it applies to the row's columns is leakproof, so an index over
  a column compared with an operator that is not leakproof goes unused by every runtime role. That
  is why the sender domains are `text`, served by a leakproof `text` index, and why no index over
  labels or the subject exists: array operators, `ILIKE` and trigram matching have no leakproof
  form, so reads over labels and the subject filter after the account index. Marking a function
  leakproof is refused. The rule a statement keeps is
  [ADR-0066](./0066-data-access-generated-from-sql.md)'s. Whether row-level security stays is
  reconsidered after the third production point ([ROADMAP.md](../../../ROADMAP.md)).
- **A new account-keyed table** leads its key with `account_id text NOT NULL REFERENCES accounts`,
  carries a policy on `app.account`, compares its keys with leakproof operators, checks its closed
  vocabularies, has its grants per role in the migration that creates it, and holds no body column.
