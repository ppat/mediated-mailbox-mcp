# Decision records

Every reversible decision in the design, one record each: its context, the decision, the
alternatives it displaced, and its consequences. The split with [DESIGN.md](../../DESIGN.md) is
stated there and holds from both sides: the design document holds what would still be true if any
individual reversible decision had gone the other way; a record here holds one such decision.
Records state decisions; [ROADMAP.md](../../ROADMAP.md) tracks what is built versus pending —
build state never lives here.

The stable documents cite records by number ("ADR-0007"), resolved through this index, never
deep-linked — records are the fluid layer and may move folders, split, or be superseded, while a
record's number is stable. Records themselves link freely: deep into
[USE_CASES.md](../../USE_CASES.md), [DESIGN.md](../../DESIGN.md),
[ROADMAP.md](../../ROADMAP.md), and each other.

## Record format

- **Filename `NNNN-slug.md`.** The number is global across all groups, allocated in mint order,
  stable, and never reused; the folder is theme navigation only, and a record may move folders
  without renumbering. The number is the durable handle; the path is not.
- **An H1 stating the decision as a claim** (`# NNNN. <decision>`), then a header line carrying
  the record's metadata as visible, linked text: `**Status:** … · **Pillar:** … · **Serves:** …`,
  with outcome identifiers linked to their defining sections. `Pillar` appears only when the
  record implements one — some decisions are pure consequence-management and serve no single
  pillar. This line is the *single* home for a record's metadata; the tables below mirror status
  for scanning, and the record wins on disagreement.
- **Fixed sections: Context, Decision, Alternatives considered, Consequences.** Alternatives is
  mandatory, because a decision whose alternatives are unstated cannot be re-argued honestly —
  and each alternative carries the case that was made for it, not only why it lost, for the same
  reason. An alternative for which no case was tabled says so plainly rather than inventing one.
  An explicit "none seriously considered, because …" is a valid body; a missing section is
  indistinguishable from an oversight. Where a Decision's own structure already carries the
  rejected options (a verdict table, rejection reasons inline), the section says so and points at
  them rather than restating.
- **No YAML frontmatter, deliberately.** A machine-readable metadata block would be a second home
  for facts the header line already states, and frontmatter cannot carry the links the line
  requires. The readers here — humans and LLMs — read text; this index is the queryable view.
- **Statuses:** **Proposed** (adopted by the documents, awaiting operator ratification) →
  **Accepted** → **Superseded** (the header gains `**Superseded by:** ADR-NNNN`; the replacement
  is a new number).
- **In-place change versus supersession.** An accepted record may change in place when the change
  stays true to the original decision in spirit and is backwards compatible with the previous
  interpretation — everything true or permitted under the old reading remains so (broadening a
  referent, clarifying, adding a consequence the decision always implied). A change that
  reverses, narrows, or re-argues what was decided supersedes instead.
- **Consequences name what the decision assumes about other components.** A dependency on how
  another component or subsystem behaves, left implicit, is the coupling that breaks future
  evolution; naming it in the record makes it reviewable at the moment it is created.
- **Records state what must effectively happen, never the platform stack.** No product names and
  no platform vocabulary in a record unless the decision is genuinely about that product; the
  product-free rendering ("values", "admission machinery", "the disposable cluster") carries the
  same meaning without binding the decision to a stack it does not depend on.
- **One decision per record, cut by the re-argue test.** Decisions merge into one record when they
  share one review context and would be re-argued together — reversing one forces re-arguing the
  others. They stay separate when independently reversible. A record found to be carrying two
  separable decisions is split at the next substantive touch; two records that always travel
  together merge the same way.

## Redaction — `redaction/`

| # | Record | Status |
| --- | --- | --- |
| 0001 | [The redaction matrix: metadata survives restriction, content never does](./redaction/0001-redaction-matrix.md) | Accepted |
| 0002 | [Body release re-evaluates at fetch time; a gate denial never contacts the provider](./redaction/0002-fetch-time-re-evaluation.md) | Accepted |
| 0003 | [Subject masking is aggressive, runs on every message, uses only the pattern tiers](./redaction/0003-subject-masking.md) | Accepted |
| 0007 | [A composite scan gate with a measured, accepted residual](./redaction/0007-composite-scan-gate.md) | Accepted |
| 0008 | [Restricted-sender bodies are never scanned](./redaction/0008-restricted-senders-are-never-scanned.md) | Accepted |
| 0009 | [The scanner runs out-of-band, in-memory, emitting verdicts that cannot carry content](./redaction/0009-scanner-verdicts-carry-no-content.md) | Accepted |
| 0029 | [Released bodies are sanitized and delimited as untrusted data](./redaction/0029-released-bodies-are-sanitized.md) | **Superseded** |
| 0036 | [Released bodies are clean Markdown — content and links, nothing else](./redaction/0036-released-bodies-are-clean-markdown.md) | Accepted |
| 0037 | [Delisting is a designed transition: a removed sender's messages are marked pending scan](./redaction/0037-delisting-transition.md) | Accepted |

## Classification — `classification/`

| # | Record | Status |
| --- | --- | --- |
| 0004 | [The sender list decides; heuristics only propose](./classification/0004-sender-list-decides.md) | Accepted |
| 0005 | [Tiered structural secret detection; no LLM in the scanning path](./classification/0005-tiered-detection.md) | Accepted |
| 0006 | [Tier 3 is a small local model, deferred until real labeled data exists](./classification/0006-tier-3-local-model-deferred.md) | Accepted |

## Provider — `provider/`

| # | Record | Status |
| --- | --- | --- |
| 0010 | [One provider port: a canonical contract every adapter compiles to](./provider/0010-one-provider-port.md) | Accepted |
| 0011 | [Gmail auth: installed-app OAuth with `gmail.modify`; delegation rejected](./provider/0011-gmail-auth-installed-app-oauth.md) | Accepted |
| 0012 | [Fastmail auth: scoped API tokens per protocol category](./provider/0012-fastmail-scoped-jmap-tokens.md) | Accepted |
| 0026 | [Multi-account: one process, N account contexts, isolation by construction](./provider/0026-multi-account-contexts.md) | Accepted |
| 0027 | [Calendar sensitivity keys on any participant, not organizer-only; Fastmail speaks CalDAV](./provider/0027-calendar-classification.md) | Accepted |

## Data — `data/`

| # | Record | Status |
| --- | --- | --- |
| 0015 | [The metadata index lives in Postgres, not a key-value store](./data/0015-postgres-not-a-kv-store.md) | Accepted |
| 0016 | [The schema: no body columns anywhere, everything scoped by account](./data/0016-schema.md) | Accepted |
| 0017 | [Backfill is full-history and two-pass: metadata first, gated scanning second](./data/0017-two-pass-backfill.md) | Accepted |
| 0018 | [Delta sync polls on a short cadence; push delivery rejected](./data/0018-delta-sync-polls.md) | Accepted |
| 0047 | [The schema is the single authority, read through per-query result types — no entity model](./data/0047-schema-first-data-access.md) | Accepted |
| 0048 | [Migrations are hand-written SQL, forward-only, run under their own role](./data/0048-forward-only-migrations.md) | Accepted |

## Mutation — `mutation/`

| # | Record | Status |
| --- | --- | --- |
| 0019 | [Asymmetric mutation: organize everything, dispose by sensitivity, delete nothing](./mutation/0019-asymmetric-mutation.md) | Accepted |
| 0020 | [Reorganization is plan → approve → apply → rollback, with an exact-restore op log](./mutation/0020-reorg-plan-approve-apply-rollback.md) | Accepted |
| 0021 | [The approval surface writes the database directly: two verbs, no credentials](./mutation/0021-approval-surface.md) | Accepted |
| 0031 | [Every mutating operation is dry-runnable — a preflight that writes nothing](./mutation/0031-dry-run-on-mutating-operations.md) | Accepted |
| 0032 | [All validation precedes the first write; saved plans validate at creation, re-validate at apply, and expire](./mutation/0032-whole-batch-validation.md) | Accepted |

## Operability — `operability/`

| # | Record | Status |
| --- | --- | --- |
| 0013 | [Credentials as mounted files via the external secret store; rotation writes back](./operability/0013-credentials-and-rotation-writeback.md) | **Superseded** |
| 0014 | [LAN-only transport; egress restriction is the control that matters](./operability/0014-lan-only-transport.md) | Accepted |
| 0022 | [The batch work is four workloads, not one background process](./operability/0022-four-workloads.md) | Accepted |
| 0023 | [The adapter declares what operations cost; the limiter is provider-agnostic](./operability/0023-adapter-declares-cost.md) | Accepted |
| 0024 | [Target half the ceiling, hard-cap at 80%, adapt below with AIMD](./operability/0024-conservative-target-aimd.md) | Accepted |
| 0025 | [One budget per account, split by priority class, shared via database leases](./operability/0025-priority-classes-and-leases.md) | Accepted |
| 0028 | [Hardening the trust anchor: minimal pod, detectable drift, surviving evidence](./operability/0028-trust-anchor-hardening.md) | Accepted |
| 0030 | [The serving layer is an API; MCP is a thin protocol adapter over it](./operability/0030-api-core-mcp-thin-adapter.md) | Accepted |
| 0033 | [Timestamps on the client surface are UTC-only; non-UTC input is rejected, never converted](./operability/0033-utc-only-timestamps.md) | Accepted |
| 0034 | [One read-only system-status operation exposes recorded per-account operational state](./operability/0034-system-status-operation.md) | Accepted |
| 0035 | [Every required identifier is discoverable on the same surface; accounts gain a listing](./operability/0035-required-identifiers-are-discoverable.md) | Accepted |
| 0038 | [Credentials arrive as mounted files](./operability/0038-credentials-as-mounted-files.md) | Accepted |
| 0039 | [Rotation write-back is delegated; the mediator holds no secret-store credential](./operability/0039-rotation-writeback.md) | Accepted |

## Engineering — `engineering/`

| # | Record | Status |
| --- | --- | --- |
| 0040 | [Cores are pure and decisions are values; thin impure shells enact them](./engineering/0040-pure-core-decisions-as-values.md) | Accepted |
| 0041 | [Policy arrives as an immutable snapshot, taken once per unit of work](./engineering/0041-policy-as-immutable-snapshots.md) | Accepted |
| 0042 | [The stack is Go end to end on the server, TypeScript only in the browser](./engineering/0042-implementation-stack.md) | Accepted |
| 0043 | [No mocking: tests run against the real dependency or a contract-tested fake](./engineering/0043-no-mocking.md) | Accepted |
| 0044 | [Tests are layered by disjoint bug class, and every green must be able to go red](./engineering/0044-layered-testing-strategy.md) | Accepted |
| 0045 | [Crash-injection stateful testing is aimed where silent failure meets hard-to-reverse damage](./engineering/0045-crash-injection-testing.md) | Accepted |
| 0046 | [Every automatable control's tests are shown to go red when the control is removed](./engineering/0046-mutation-obligation.md) | Accepted |
| 0049 | [One image per deployable, all moving in lockstep](./engineering/0049-image-per-component-lockstep.md) | Accepted |
| 0050 | [Shared code is pure, or it is a narrow, named exception](./engineering/0050-shared-code-pure-or-narrow.md) | Accepted |
| 0051 | [The app knows its environment contract, never its platform](./engineering/0051-environment-contract.md) | Accepted |
| 0052 | [The repository ships a self-contained deployment artifact that assumes nothing](./engineering/0052-deployment-product.md) | Accepted |
| 0053 | [Both client roots are generated from one operation registry — a one-sided operation is unrepresentable](./engineering/0053-parity-by-construction.md) | Accepted |
| 0054 | [One repository, flat at the top, one module; published names follow `mail-<role>`](./engineering/0054-repository-structure-and-naming.md) | Accepted |
