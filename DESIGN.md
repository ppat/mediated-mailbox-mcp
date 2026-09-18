# Mediated Mailbox MCP — Design

This file holds the high-level design of the mediated mailbox, meaning the pillars and invariants
that hold the system together and the reasoning behind them. It states what the system *is*. It
deliberately does not re-argue every decision that had alternatives. Those live as decision
records, indexed at [docs/adr/README.md](./docs/adr/README.md). The split is that this document
holds what would still be true if any individual reversible decision had gone the other way, and
a decision record holds one such decision with its context, alternatives, and consequences.

Companions: [USE_CASES.md](./USE_CASES.md) holds the outcomes this design serves, each with a
falsifiable acceptance criterion. [ROADMAP.md](./ROADMAP.md) holds all the work. Build state is a
roadmap fact, not a design fact, and a pillar binds identically whether its mechanisms are live or
unbuilt. [TESTING.md](./TESTING.md) holds the testing strategy the records behind it decide.
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) holds the proving injection for every control.
And [docs/MUTATIONS.md](./docs/MUTATIONS.md) holds each automatable control's mutation
demonstration. Where this document cites a specific decision it does so by number ("ADR-0007"),
resolved through the decision-record index, never deep-linked. Records are the fluid layer and
may move or be superseded, while a record's number is stable. Vocabulary used across the
documents is defined in the [Glossary](#glossary) below, and nowhere else.

## 1. What the system is

An AI agent is given a complete mailbox to organize, triage, and summarize. A self-hosted
mediation layer sits between the mailbox provider and the system's clients, exposing an API
(with MCP as a thin protocol adapter over it) while enforcing sender-based and content-based
redaction underneath. The agent always sees full mailbox structure (every thread, sender,
subject, label, timestamp) and never sees the body content of messages from a defined set of
sensitive senders, nor the codes and login links that grant account access. The mailbox stays
with its managed provider. Only the mediation layer runs on infrastructure the operator already
owns.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│                                                                  │
│  ┌────────────────────────┐      ┌────────────────────────────┐  │
│  │ Clients: agent (MCP),  │      │ Operator, in a browser     │  │
│  │ API callers            │      │                            │  │
│  └───────────┬────────────┘      └─────────────┬──────────────┘  │
│              │ MCP or API, HTTPS               │ HTTPS           │
│              │ bearer token                    │                 │
└──────────────┼─────────────────────────────────┼─────────────────┘
               │                                 │
┌──────────────▼─────────────────────┐  ┌────────▼─────────────────┐
│ mail-mediator                      │  │ UI (Web interface)       │
│                                    │  │  read-mostly reporting   │
│ ┌────────────────────────────────┐ │  │  + approval surface      │
│ │ Client Surface: API + MCP roots│ │  │  separate identity and   │
│ └──────────────┬─────────────────┘ │  │  DB role; NEVER shows    │
│ ┌──────────────▼─────────────────┐ │  │  bodies (none exist)     │
│ │ Redaction Gate     ◄ CHOKEPOINT│ │  └────────┬─────────────────┘
│ └──────────────┬─────────────────┘ │           │
│ ┌──────────────▼─────────────────┐ │           │ writes ONLY (direct
│ │ Mutation Authorizer            │ │           │ to DB, never via the
│ └──────────────┬─────────────────┘ │           │ client surface):
│ ┌──────────────▼─────────────────┐ │           │ plan approve/reject,
│ │ Sender Classifier              │ │           │ candidate confirm/
│ └──────────────┬─────────────────┘ │           │ dismiss — ADR-0021
│ ┌──────────────▼─────────────────┐ │           │
│ │ Provider Port     ◄ ABSTRACTION│ │           │
│ └──┬────────┬────────┬────────┬──┘ │           │
│  ┌─▼───┐ ┌──▼───┐ ┌──▼───┐ ┌──▼───┐│           │
│  │Gmail│ │GCal  │ │JMAP  │ │CalDAV││           │
│  └─┬───┘ └──┬───┘ └──┬───┘ └──┬───┘│           │
└────┼────────┼────────┼────────┼────┘           │
     │        │        │        │                │
  provider APIs (Gmail, Google Calendar,         │
  Fastmail JMAP, Fastmail CalDAV)                │
                                                 │
┌────────────────────────────────────────────────┼─────────────────┐
│ Batch subsystems — separate workloads          │                 │
│                                                │                 │
│ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌────▼──────────────┐  │
│ │ Backfill  │ │ Delta     │ │ Reorg     │ │ Heuristics Job    │  │
│ │ Job       │ │ Sync      │ │ Engine    │ │ (candidate gen)   │  │
│ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └────┬──────────────┘  │
│       └─────────────┼─────────────┘            │                 │
│         ┌───────────▼──────────┐               │                 │
│         │ Scan Gate            │               │                 │
│         └───────────┬──────────┘               │                 │
│         ┌───────────▼──────────┐               │                 │
│         │ Content Scanner      │               │                 │
│         │  emits verdict ONLY  │               │                 │
│         └───────────┬──────────┘               │                 │
│         ┌───────────▼──────────┐               │                 │
│         │ Rate Limiter         │  shared budget per account,     │
│         │  (spent from by each │  every process that calls a     │
│         │   provider caller)   │  provider leases from it        │
│         └──────────────────────┘               │                 │
└────────────────────────────────────────────────┼─────────────────┘
                                                 │
┌────────────────────────────────────────────────▼─────────────────┐
│ Persistence                                                      │
│  • Metadata/State Store — metadata, senders, plans, audit        │
│  • Policy Store — rules as rows, snapshotted on load             │
│  • Secrets — secret mounted as files                             │
└──────────────────────────────────────────────────────────────────┘
```

| Component | One job | Runs as |
| --- | --- | --- |
| Client Surface (API + thin MCP adapter) | Validate, page, and route client requests by account | mail-mediator |
| Redaction Gate | Decide what survives the last hop before any client | mail-mediator |
| Mutation Authorizer | Enforce per-sensitivity mutation rights on every write | mail-mediator |
| Sender Classifier | Classify senders deterministically against the policy list | mail-mediator |
| Provider Port + adapters | Speak each provider's API and expose one canonical model | mail-mediator, and each batch workload that calls a provider |
| Scan Gate | Choose which non-restricted bodies get scanned | batch subsystems |
| Content Scanner | Read bodies it will withhold and emit content-free verdicts | batch subsystems |
| Rate Limiter | Keep all workloads inside one polite per-account budget | mail-mediator, and each batch workload that calls a provider |
| Backfill Job | Build the full-history metadata index once | batch workload |
| Delta Sync | Keep the index current against the provider | batch workload |
| Reorg Engine | Turn approved plans into reversible bulk mutations | batch workload |
| Heuristics Job | Propose sensitive-sender candidates for human review | batch workload |
| UI (Web interface) | Make the system legible to the operator and carry the approval verbs | separate deployment |
| Metadata & State Store | Hold metadata, plans, and audit, never a body | Postgres instance |
| Policy Store | Hold the sender rules as data, snapshotted on load | Postgres tables |

Component names in this document are conceptual. Directories and published names follow the
convention of ADR-0054, via the [decision-record index](./docs/adr/README.md).

### How a body request flows

The one narrative to read first, assembled from the decision records, resolved through the
[decision-record index](./docs/adr/README.md) with each step restated nowhere else:

- ADR-0002 — the fetch-time gate flow that decides allow or deny, including the serve-time
  pattern check that can still deny a never-scanned body.
- ADR-0001 — the field-level matrix behind that decision.
- ADR-0007 — the scan states a message can be in, including the accepted residual.
- ADR-0036 — the sanitization applied to whatever is released.

Metadata requests need none of it. Metadata paths cannot carry a body by construction.

## 2. The pillars

Everything else in this repository is a consequence of these. Each is stated with the reasoning
that holds it up. The decisions that *implement* each one, and their alternatives, are decision
records.

### Metadata always flows; sensitive bodies never do

Every component is judged on whether it holds that line at the only place it can be held
reliably, the last hop before any client, inside a process no client can instruct. Structure
(senders, subjects, threads, labels, dates) is always visible, because organizational capability
is the system's purpose, not a concession. Sensitive content (restricted-sender bodies, MFA
codes, login links) is never released, and no request any client can make unlocks it.

Why: both halves at once are the point. Where any other outcome would trade against holding
them, the other outcome loses. And because the organizational patterns are metadata-derivable, the
design can offer strong organizational capability alongside strong content restriction rather than
trading one for the other.

Known limit, stated rather than hidden: metadata itself leaks. Subjects, sender identities, and
traffic patterns are deliberately exposed, and a bounded residual of unscanned bodies is accepted.
Both dispositions are recorded and measured (see [Known limits](#3-known-limits)).

### Redaction is enforced by code, not by the provider's token

The mediator's own credentials are always full-mailbox. No available mail credential can be scoped
to exclude senders. The design assumes the token is over-privileged and compensates by making the
redaction path unbypassable, rather than by trusting the credential to constrain anything.

Why: this is the constraint the whole architecture is built around. Because it cannot be delegated
to the provider, enforcement must be a property of code the operator controls, which is why the
mediation layer exists at all. If a provider ever offers genuine per-sender token scoping, the
mediation layer becomes optional and this design should be revisited from the ground up.

Known limit, stated rather than hidden: because the credential cannot be narrowed, a compromise of
any process holding it exposes the full mailbox. The trust-anchor pillar carries that consequence.
Where a provider *does* offer narrowing, capability absent from the granted credential (such as
permanent delete) is a real guarantee, and the design takes it as defense-in-depth wherever it
exists.

### One gate, N dumb adapters

Redaction happens in exactly one place, above the provider adapters and below every client-facing
surface. Adapters return full data by design. The Redaction Gate decides what survives. No
adapter, frontend, tool handler, or batch path holds its own copy of the rules.

Why: redaction can only be enforced above the provider adapter and below the client surface. No
available mailbox credential can be scoped to exclude senders, and the client surface is where
callers instruct. Adapters return full data by design and the gate decides what survives, so a
bug in adapter #3 cannot become a silent leak. The adapter never had redaction responsibility
to get wrong.

Known limit, stated rather than hidden: a single chokepoint concentrates correctness. A bug in the
gate is a bug everywhere. That is the accepted trade, and it is why the gate is built first,
tested against its failure modes, and kept small.

### Fail closed, everywhere

Every ambiguous or error state (a policy that has never validly loaded, scanner backlog,
classification failure, unscanned message) resolves to *deny the body*. "Allow" requires an
affirmative safe classification. The mere absence of a positive signal is never enough. Work that
has not happened yet is a deny state, not an open door. A policy update that fails validation is
not an ambiguous state. It never takes effect, and the active valid policy continues to govern.

Why: the failure directions are asymmetric. Over-redaction is an inconvenience the operator can see
and tune. Under-redaction is a leak that cannot be recalled. A system whose degraded modes all
point toward deny degrades in utility, never in safety.

Known limit, stated rather than hidden: fail-closed paths are exercised by tests or not at all.
Production never visits them until the day it matters. Their proving injections are catalogued in
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), and the proof those tests can fail lives in
[docs/MUTATIONS.md](./docs/MUTATIONS.md).

### Sensitivity is two independent axes

Who sent a message and what the message contains are separate questions with separate
consequences. Sender class governs readability *and* disposal rights. Content flags govern
readability only. The axes are independent in that either one can deny a body on its own, and only
sender class constrains disposal. They compose by union. Restrictions accumulate and never cancel.

Why: collapsing the axes would force sensitivity to mean one thing when it means two. A message can
be unreadable yet disposable (an expired MFA code from an ordinary sender is exactly the
clutter the agent should archive), and unreadable yet untouchable-for-disposal (a restricted
financial notice the agent files but can never trash). Each useful combination exists. A
one-dimensional model would forbid some of them by accident.

Known limit, stated rather than hidden: union composition means restrictions only accumulate. A
message over-restricted by either axis stays restricted until the policy or verdict behind it is
corrected. Over-redaction is the accepted failure direction, and the masking and gate review loops
exist to tune it from observed traffic.

### Approval is not in any client's vocabulary

The transition that authorizes a bulk mailbox change is performed by the operator, on a surface
no client can reach. No MCP tool and no API endpoint performs it, so no sequence of client
actions, however persuaded or prompt-injected, can manufacture consent. Clients can only
propose. Only the human can approve.

Why: attacker-controlled text flows into the agent's planning input by design (subjects are always
visible, and non-sensitive bodies flow). Instructing the agent to be careful is not a control. The
control is structural. The approval verb does not exist on the client-facing surface, so talking
the agent into anything changes nothing.

Known limit, stated rather than hidden: the approval surface itself becomes a target. Compromise
of it means malicious plans can be approved. Its write surface is kept minimal and its blast radius
bounded by independent enforcement at apply time. The decisions doing that bounding are recorded as
decision records.

### Unsafe states are unconstructable, not merely untaken

Where a leak is possible, the design removes the type, field, column, or verb that could carry it,
rather than adding a check that declines to use it. Scanner verdicts have no field a body could
hide in. Metadata fetch paths cannot return content. The message store has no body column. The
client surface has no permanent-delete operation, and neither does the granted token.

Why: an unsafe state that is merely untaken can still be taken. One that cannot be represented
cannot be reached at all. There is no field a body could hide in, no verb a permanent delete could
ride on.

Known limit, stated rather than hidden: not every unsafe state can be made unrepresentable. Bodies
must transit mediator memory to be served and scanned at all. Where representation is unavoidable,
the design falls back to fail-closed checks plus audit, and says so explicitly.

### Everything above the port speaks canonical

A single provider port defines the system's model of mail and calendar. Everything above it (gate,
classifier, authorizer, client surface, batch workloads) speaks that canonical model. The adapters
below it are the only code that knows a Gmail label from a JMAP mailbox. Adding a backend means
writing one new adapter, not redesigning the system.

Why: without the boundary, provider-specific concepts (label IDs, query syntax, mailbox trees,
sync-state strings) appear above the adapter layer, and switching backends becomes a redesign
instead of one new adapter. With it, each adapter compiles the differences away, and the redaction
gate, classifier, and client surface never contain a branch on provider identity.

Known limit, stated rather than hidden: the abstraction is a design intention until the second
adapter is built. The contract is only proven when a backend swap forces no change above the port.
That deferred test is deliberate and tracked in [ROADMAP.md](./ROADMAP.md).

### Accounts are isolated by structure, not convention

The architecture is multi-account even while one account is deployed. Every operation names its
account explicitly. There is no implicit current account. Every table, query, client, and
credential is account-scoped by construction, and accounts may span organizations, with no shared
OAuth client, no delegation, and no assumed common administrator.

Why: cross-account bleed is the class of bug that convention cannot hold against concurrency. A
shared client with a mutable auth header fails exactly when two accounts are active at once.
Structural scoping makes the bleed unrepresentable rather than unlikely, and building it in from
the start costs little while retrofitting it later costs a redesign.

Known limit, stated rather than hidden: like the port abstraction, isolation is proven only when
the second account exists. Until then it is enforced structure awaiting its test.

### An accepted risk that is not measured is an unmeasured risk

Every deliberate compromise in this design (each accepted residual, each deliberate exposure) is
recorded with its reason and instrumented so its actual size is observable. Every body served,
every denial, every mutation, every skip is audited. Acceptance without measurement is not
acceptance. It is blindness with a rationale.

Why: the recorded skips, masks, serves, and denials are what let the operator tune each compromise
from evidence rather than intuition. A body served, denied, or mutated without a record would
be unanswerable after the fact.

Known limit, stated rather than hidden: a metric not collected for a window already passed is lost
for good, so instrumentation must exist before the window it will be asked about. That is an
ordering constraint on the work, carried by [ROADMAP.md](./ROADMAP.md).

### Network position never substitutes for the gate

The redaction invariant does not depend on network position of the deployment. Any authenticated
caller reaching the client surface gets exactly what the gate permits and nothing more.

Why: A LAN-only scope is a meaningful simplification (it removes the entire class of "someone on
the internet reaches the mediator" risk), but it determined by the deployment environment and its'
configuration. As that falls outside the purview of this design, the redaction invariant must not
depend on the network position of the deployment.

### The mediation layer is the irreducible trust anchor

The mediation layer holds full mailbox credentials, in every one of its processes that calls a
provider. If any of those processes is compromised, redaction is moot, because the attacker calls
the provider directly. The design does not pretend otherwise. The stance is that this anchor is
hardened, its blast radius is understood, and evidence of its compromise survives outside its own
reach.

Why: some process must hold the over-privileged credential. That follows from redaction being
enforced by code, not the token. The design accepts this as the irreducible trust anchor rather
than pretending otherwise, and directs the hardening there.

Known limit, stated rather than hidden: this is the one failure the design cannot make
unrepresentable, only expensive, detectable, and evidenced, which is why no runtime role can erase
an audit row. What that buys is bounded. Evidence written before a compromise survives it. What an
attacker writes afterwards is theirs.

### Concerns stay un-braided; components know only their contracts

Nothing in the design or the implementation may inhibit future evolution, including evolution
arriving after the entire currently-envisioned product is built and working. Two rules carry
that. Choices are judged on whether they braid together concerns that will need to vary
independently, and the familiar-but-complected option never wins over the simple-but-harder one.
And no component assumes or encodes anything about how other components or subsystems operate
beyond what its contract with them says. Contracts are the only licensed knowledge. Everything
else is somebody else's business.

Why: the design's deliberate absences (absent verbs, absent columns, a minimal write surface)
are explicit, decision-recorded, and extendable any time by minting a new decision through the
front door. They do not inhibit evolution. What inhibits it is implicit coupling, a component
quietly relying on how another behaves today, captured in no record and found by no future reader
until it breaks their extension. The architecture already instantiates the principle (the
provider port, the separation of the gate from the authorizer, the four separate workloads), and
this pillar makes it binding on every decision and every line of code to come, which is why
decision records name their cross-component assumptions in their Consequences.

Known limit, stated rather than hidden: like the backend swap, this is only tested when an
evolution arrives. Until then it is enforced structure awaiting its test.

## 3. Known limits

The pillars state their limits in place. This table exists so the full set (every pillar's limit,
plus the standing assumptions that attach to no single pillar) is checkable in one place. Rows say
where each disposition is recorded, not what it is. The record named is the single home.

| Limit | Disposition lives in |
| --- | --- |
| Metadata (subjects, senders, traffic patterns) is deliberately exposed | ADR-0001, via the [decision-record index](./docs/adr/README.md) |
| A bounded residual of unscanned bodies is released by design | ADR-0007 and ADR-0002, and its measurement rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| A single chokepoint concentrates correctness, so a gate bug is a bug everywhere | Built first and proven offline, via the S1 unit in [ROADMAP.md](./ROADMAP.md) and its rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| Fail-closed paths are exercised by tests or not at all | Their injections in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), and the proof those tests can fail in [docs/MUTATIONS.md](./docs/MUTATIONS.md) (ADR-0046) |
| Union composition means over-restriction stands until its policy or verdict is corrected | The masking and gate review loops (ADR-0003, ADR-0007) |
| The approval surface is itself a target | ADR-0021 (two verbs, scoped role, no credentials) |
| Bodies must transit mediator memory to be served and scanned at all | ADR-0009 |
| A metric not collected for a past window is lost for good | [ROADMAP.md](./ROADMAP.md), where emission is a non-deferrable riding the units that emit |
| Content released to the agent is released, into context, transcripts, and memory | ADR-0036 bounds it. It cannot be recalled |
| Compromise of a process holding provider credentials defeats redaction | ADR-0028 (hardening, blast radius, evidence that survives) |
| Backend-swap and multi-account isolation are unproven until a second adapter/account exists | [ROADMAP.md](./ROADMAP.md), as the units that run those tests |
| Un-braided concerns and contract-only knowledge are only tested when an evolution arrives | The records' assumption-naming convention ([docs/adr/README.md](./docs/adr/README.md)) |
| The corpus is assumed ≤100k messages per account | ADR-0016 records what changes beyond it, and ADR-0066 what stops being affordable |
| Nothing in the running system can trim the audit log, because no runtime role may delete from it | ADR-0016, with the retention question open in [ROADMAP.md](./ROADMAP.md) |

## 4. Failure modes

The system's known failure modes with their qualitative assessments, enumerable in one place. Rows
are pointers. Each fix and its reasoning live in the records named, never here.

| Failure mode | Likelihood / impact | Disposition lives in |
| --- | --- | --- |
| Prompt injection via a released body | near-certain / high | ADR-0036 · ADR-0002 (the gate is the control) |
| Injection aimed at the reorg plan through visible subjects | moderate / high | ADR-0020 |
| Sender spoofing (the unlisted co-brand domain) | moderate / high | ADR-0004 |
| Policy-list staleness | certain over time / medium | ADR-0004 |
| Metadata leakage | certain, accepted / medium | ADR-0001 |
| Scan-gate residual leakage | accepted / medium | ADR-0007 · ADR-0002 |
| Compromise of a process holding provider credentials | low / catastrophic | ADR-0028 |
| Fail-open on classifier or scanner error | low / severe | ADR-0002 · the fail-closed rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| Agent context as an exfiltration surface | moderate / medium | ADR-0036 |
| Bulk mutation error | moderate / severe | ADR-0020 · ADR-0032 |
| Scan backlog as silent utility loss | low / moderate | ADR-0007 |
| UI as a write path | low / moderate | ADR-0021 |
| Rate-controller pathology (collapse or runaway) | moderate / medium | ADR-0024 · ADR-0025 |
| Policy reload failure leaves a published rule unapplied | low / medium | ADR-0041 |

## Glossary

The single home for vocabulary. A cold reader should be able to resolve any term used in the
top-level documents, a decision record, or a ticket from here without guessing.

### The system and its parties

- **The invariant** — the fixed point of [USE_CASES.md](./USE_CASES.md), used as a proper noun
  throughout the documents, meaning full organizational visibility and zero sensitive content,
  held at once.
- **The mediator** (`mail-mediator` in this document's diagram and component table, published as
  `mediated-mailbox-mediate` from the directory `mediate/`) — the process that enforces redaction
  and serves the client surface. It holds provider credentials, as every deployable that calls a
  provider does (ADR-0038, via the [decision-record index](./docs/adr/README.md)), and those
  processes together are the trust anchor.
- **Client** — any caller of the serving surface, whether the agent over MCP or any other caller
  of the API. Every client is untrusted by design. Every control assumes a client can be talked
  into, or built to attempt, anything.
- **The client surface** — the mediator's serving surface, the API plus its MCP adapter (shape
  and rules in ADR-0030, via the [decision-record index](./docs/adr/README.md)).
- **The agent** — the primary client today, the MCP-speaking assistant given the mailbox to
  organize (Claude Code or similar).
- **The operator** — the single human who owns the infrastructure, edits policy, and approves
  plans.
- **The UI** (directory `ui/`) — the read-mostly reporting and approval surface. Separate
  deployment, separate identity, no provider credentials. Carries the approval verbs no client
  has. Its design is [docs/UI.md](./docs/UI.md).
- **The shared pure library** — the pure-core-only library every deployable may
  import. Impure shared needs live in narrow, named exception libraries instead (rule in
  ADR-0050, via the [decision-record index](./docs/adr/README.md)).
- **Provider** — the managed service actually holding the mail or calendar (Gmail, Fastmail).
- **The real mailbox** — the operator's own mail at the provider, as distinct from the synthetic
  fixtures every test runs over.
- **Production** — the deployed system on the operator's infrastructure, as distinct from the
  real mailbox it reads. A build's production mode, the toolchain's sense in
  [docs/UI.md](./docs/UI.md) and ADR-0063, is unrelated.

### Sensitivity and redaction

- **Sender class** — the per-sender axis, `normal` or `restricted`. Decides disposal rights, and
  a restricted class denies the body on its own. Assigned deterministically from the policy list,
  never by heuristic.
- **Restricted sender** — a sender matching the operator's deny list. Bodies never released.
  Disposal verbs never authorized.
- **Content flag** — the per-message axis, a detected MFA code or login link. Decides
  readability. Any flag denies the body.
- **Scan state** — a message's position relative to the content scanner, meaning pending,
  scanned, or one of the deliberate skip states. "Not scanned" has distinct meanings with
  different consequences. Pending always denies.
- **Redaction Gate** — the mediator component that decides, at the last hop, which fields of a
  message a client receives. Distinct from the *Scan Gate* below.
- **Scan Gate** — the batch-side predicate that decides which non-restricted bodies are worth
  scanning. Distinct from the *Redaction Gate* above. The Redaction Gate guards release to
  clients, and the Scan Gate budgets scanning work.
- **Content Scanner** — the component that reads bodies it intends to withhold, detects codes and
  links, and emits verdicts that structurally cannot carry content.
- **Masking** — replacing a detected secret inside an otherwise-visible field (an MFA code in a
  subject) with an opaque placeholder.
- **Delisting transition** — the designed path for a sender removed from the sensitive list. Its
  messages are marked pending scan and re-enter the ordinary scanning machinery (rule in
  ADR-0037, via the [decision-record index](./docs/adr/README.md)).
- **Serve-time pattern check** — the additional inspection a gate-skipped body passes at
  release (rule in ADR-0002, via the [decision-record index](./docs/adr/README.md)).
- **The residual** — the accepted, measured set of bodies released without having been scanned
  (bounds in ADR-0007 and ADR-0002, via the
  [decision-record index](./docs/adr/README.md)).

### Policy and classification

- **Policy list / gazetteer** — the operator-editable, deterministic sender-classification rules,
  held as rows in the database and taken as snapshots, with a file form for import and export
  (rule in ADR-0004, via the [decision-record index](./docs/adr/README.md)). The only authority
  on sender class.
- **Policy overlay** — per-account additions to the base policy. Overlays only add restrictions.
  They can over-restrict, never under-restrict.
- **Policy snapshot** — the immutable, atomically-swapped copy of the policy that one request or
  unit of batch work decides against (rule in ADR-0041, via the
  [decision-record index](./docs/adr/README.md)).
- **Heuristics Job** — the batch workload that proposes sensitive-sender candidates from observed
  traffic. Proposes only. Nothing it emits takes effect without operator confirmation.
- **Review queue** — where heuristic candidates wait, ranked with their evidence, for the operator
  to confirm or dismiss through the UI.
- **Tier** — a stage of content detection, ordered by cost. The tiers are structural patterns,
  then scoring, then (deferred) a small local model. Higher tiers see only what lower tiers could
  not settle.

### Provider abstraction and accounts

- **Provider Port** — the single interface the system's canonical model is defined against.
  Everything above it is provider-blind.
- **Adapter** — a per-provider implementation of the port, and the only code that knows provider
  concepts. Adapters are deliberately dumb. Full data in, no redaction responsibility.
- **Canonical model** — the provider-neutral representation of messages, threads, labels, queries,
  and changes that everything above the port speaks.
- **Account context** — the per-account bundle of provider clients, credentials, policy overlay,
  and rate state. Nothing about an account is ambient. Every operation names one.

### Data paths and mutation

- **Backfill** — the one-time construction of the full-history metadata index, metadata first,
  gated body scanning after.
- **Delta sync** — the recurring job that keeps the index current against provider change feeds.
- **Sync interval** — the delta-sync polling cadence that bounds index freshness (rule and
  value in ADR-0018, via the [decision-record index](./docs/adr/README.md)).
- **Reorg plan** — a proposed bulk reorganization, stored as data (label operations, per-message
  operations, reasoning, and scale). A plan is never an action.
- **Maximum plan age** — the age, measured from creation, past which a saved reorg plan is
  rejected outright at apply time (rule in ADR-0032, via the
  [decision-record index](./docs/adr/README.md)). The value is an open decision in
  [ROADMAP.md](./ROADMAP.md).
- **Flow** — within a reorg plan, the pair of one removed label (or none) and one added label
  (or none) that one operation makes, the unit the plan reviewer groups by (rule in ADR-0020, via
  the [decision-record index](./docs/adr/README.md)).
- **Op log** — the per-operation before/after record written during plan application, from which
  rollback is exact replay, not inference.
- **Statement file** — one hand-written SQL file the data-access generator reads. How the files
  are grouped, what they may contain, and what is generated from them are ADR-0066's (via the
  [decision-record index](./docs/adr/README.md)).
- **Account-keyed table** — a table carrying an `account_id` column. What every statement against
  one must do, how the set of them is established, and which tables are excepted are ADR-0047's
  (via the [decision-record index](./docs/adr/README.md)).
- **Runtime role** — a database role a running deployable connects as, holding only the grants its
  work needs, as distinct from the schema-owning role the migration step uses (roles in ADR-0048
  and ADR-0021, via the [decision-record index](./docs/adr/README.md)).
- **The migration chain** — the ordered set of hand-written migration files that builds the
  schema. How it evolves, when it is applied, and what its first entry carries are ADR-0048's
  (via the [decision-record index](./docs/adr/README.md)).
- **Mutation Authorizer** — the mediator component that checks every write against the message's
  sensitivity, independently of who approved what. Mutation here is the mailbox-write sense,
  not the test-suite sense of **Mutation demonstration**.

### Operability

- **Rate profile** — an adapter's declaration of what operations cost on its provider and what
  budget exists. The limiter is provider-agnostic. Only adapters know costs.
- **Priority class** — the division of the per-account rate budget between interactive, sync, and
  batch work, so background work can never starve interactive work.
- **Lease** — a short-lived allocation of rate budget to one process, the mechanism by which
  separate workloads share one per-account budget without a coordinator process.
- **Audit log** — the record of every body served, every denial, and every mutation. Must survive
  the compromise of any process holding provider credentials, so no runtime role may update or
  delete a row of it (grant and bound in ADR-0016 and ADR-0028, via the [decision-record
  index](./docs/adr/README.md)).
- **Masking events / gate decisions** — the per-event records that make masking behavior and scan
  skips tunable from evidence.

### Doctrine

- **Pure core / impure shell** — the structural split every component follows. Pure decision
  functions over parameters, wrapped by a thin shell that does the I/O and enacts the decisions
  (rule in ADR-0040, via the [decision-record index](./docs/adr/README.md)).
- **Verdict** — a decision returned as a data value by a pure core, enacted and recorded by the
  shell. Verdict types structurally cannot carry the content they withhold (rule in ADR-0040 and
  ADR-0009, via the [decision-record index](./docs/adr/README.md)).
- **Shared comparison options** — the single options value every unit test's comparison passes,
  naming each type whose unexported fields a comparison may read (rule in ADR-0070, via the
  [decision-record index](./docs/adr/README.md)).
- **Contract suite** — the one test suite every provider-port implementation must pass
  (rule in ADR-0043, via the [decision-record index](./docs/adr/README.md)).
- **Control** — a rule the system enforces, together with the mechanism that enforces it. Every
  control is proven by its violation injection, catalogued in
  [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md).
- **Generator report** — the test support this project writes that classifies each case a
  property-based test generates, reports the mix, and fails the run when a stated minimum share of
  a kind of input is not reached (rule in ADR-0069, via the
  [decision-record index](./docs/adr/README.md)).
- **Failing-case store** — the test support this project writes that keeps a failing
  property-based or crash-sequence case and replays it on later runs, in a form that survives an
  edit to its generator (rule in ADR-0069, via the [decision-record index](./docs/adr/README.md)).
- **Operation sampler** — the test support this project writes that draws a fresh mix of
  operations for each sequence the crash harness generates in its scheduled run (rule in ADR-0069,
  via the [decision-record index](./docs/adr/README.md)).
- **Marker text** — the searchable strings designed into synthetic fixture bodies and metadata
  fields so a leak check over any output surface, a rendering surface included, is deterministic
  (rule in ADR-0044, via the
  [decision-record index](./docs/adr/README.md)).
- **Provider fake** — the contract-tested stand-in implementing the provider port in tests
  (rule in ADR-0043, via the [decision-record index](./docs/adr/README.md)).
- **Mutation demonstration** (also *mutation table*) — the per-control record that removing a
  control's mechanism made its tests go red, kept in
  [docs/MUTATIONS.md](./docs/MUTATIONS.md) (rule in ADR-0046, via the
  [decision-record index](./docs/adr/README.md)). Mutation here is the test-suite sense,
  removing a mechanism, and not the mailbox-write sense of **Mutation Authorizer**.
- **Violation injection** (also *proving injection*) — the acceptance standard. A control is
  proven by deliberately creating the violation it exists to stop and watching it fire, never by
  observing that nothing bad happened. Catalogued in
  [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md).
- **Violation file** — a checked-in file that deliberately breaks a check standing in for a control,
  such as a lint ban, an import rule or a statement-file constraint, placed where the rule applies,
  so the check's proof can require it to be reported (rule in ADR-0046 and ADR-0071, via the
  [decision-record index](./docs/adr/README.md)). Where the files sit is
  [CLAUDE.md](./CLAUDE.md#tests)'s.
- **Ban-proof script** — the check that runs every lint ban and import rule against its violation
  files and requires each expected finding and no other, and refuses a suppression that could switch
  a control's rule off (rule in ADR-0046 and ADR-0071, via the [decision-record
  index](./docs/adr/README.md)).
- **Ordinary linter** — a linter the repository runs that stands in for no control, one of a closed
  list whose findings alone may be suppressed (rule in ADR-0071, via the [decision-record
  index](./docs/adr/README.md)). The list is
  [CLAUDE.md](./CLAUDE.md#static-analysis-and-formatting)'s.
- **Data-access subsection** — one package of the data-access library, grouping the statements and
  generated code for one table or closely related tables, which a component's import list names when
  the component may use it (rule in ADR-0066 and ADR-0071, via the [decision-record
  index](./docs/adr/README.md)).
- **Answerable-by-doing** — an open empirical question that needs an experiment or accumulated
  data rather than a build, registered with its trigger point so it cannot evaporate.

### The UI

- **Lens** — one view of the UI, meaning an account-scoped dataset viewed at a zoom level,
  sliced by dimensions, drilled to rows, with a decision attached where one exists (model in
  [docs/UI.md](./docs/UI.md#3-the-lens-model)).
- **Zoom ladder** — the five levels a lens is viewed at, from summary to row detail, and the
  navigation rules between them (levels in [docs/UI.md](./docs/UI.md#4-the-zoom-ladder)).
- **Dataset registry** — the UI's single declaration of what its read API can be asked, per
  dataset (rule in ADR-0057, via the [decision-record index](./docs/adr/README.md)). Distinct
  from the client surface's operation registry of ADR-0053, which it mirrors in mechanism.

### Identifiers

- **Outcome identifiers** — C1–C4 (the invariant), G1–G4 (organizational capability), P1–P3
  (provider abstraction), A1–A4 (safe action), O1–O6 (operability). They are defined in
  [USE_CASES.md](./USE_CASES.md) and used as the coordinate system everywhere else.
- **ADR-NNNN** — decision records, numbered globally in mint order, resolved through
  [docs/adr/README.md](./docs/adr/README.md).
- **Work units, value increments, production points, and finish lines** — defined in
  [ROADMAP.md](./ROADMAP.md).
- **Ticket** — one pull request's worth of work under one work unit, held as a GitHub issue in
  this repository. Its format is [the template](./.github/ISSUE_TEMPLATE/ticket.md)'s and its
  rules are [CLAUDE.md](./CLAUDE.md#repository-process)'s. The word keeps its ordinary meaning for
  a tracker item anywhere else, including the deployment repositories' own tickets that
  [ROADMAP.md](./ROADMAP.md) names.
- **Zoom levels L0–L4** — the five levels of the zoom ladder, defined in
  [docs/UI.md](./docs/UI.md#4-the-zoom-ladder).
