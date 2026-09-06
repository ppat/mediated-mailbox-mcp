# Mediated Mailbox MCP — Design

The high-level design of the mediated mailbox: the pillars and invariants that hold the system
together, and the reasoning behind them. It states what the system *is*; it deliberately does not
re-argue every decision that had alternatives. Those live as decision records, indexed at
[docs/adr/README.md](./docs/adr/README.md) — the split is that this document holds what would still
be true if any individual reversible decision had gone the other way, and a decision record holds
one such decision: its context, alternatives, and consequences.

Companions: [USE_CASES.md](./USE_CASES.md) holds the outcomes this design serves, each with a
falsifiable acceptance criterion; [ROADMAP.md](./ROADMAP.md) holds all the work — build state is a
roadmap fact, not a design fact, and a pillar binds identically whether its mechanisms are live or
unbuilt; [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) holds the proving injection for every
control. Where this document cites a specific decision it does so by number ("ADR-0007"), resolved
through the decision-record index, never deep-linked — records are the fluid layer and may move or
be superseded, while a record's number is stable. Vocabulary used across the documents is defined
in the [Glossary](#glossary) below, and nowhere else.

## 1. What the system is

An AI agent is given a complete mailbox to organize, triage, and summarize. A self-hosted
mediation layer sits between the mailbox provider and the system's clients, exposing an API —
with MCP as a thin protocol adapter over it — while enforcing sender-based and content-based
redaction underneath: the agent always sees full mailbox structure — every thread, sender,
subject, label, timestamp — and never sees the body content of messages from a defined set of
sensitive senders, nor the codes and login links that grant account access. The mailbox stays
with its managed provider; only the mediation layer runs on infrastructure the operator already
owns.

```
┌──────────────────────────────────────────────────────────────────┐
│ LAN (homelab)                                                    │
│                                                                  │
│  ┌────────────────────────┐      ┌────────────────────────────┐  │
│  │ Clients: agent (MCP),  │      │ Operator, in a browser     │  │
│  │ API callers — in-LAN   │      │                            │  │
│  └───────────┬────────────┘      └─────────────┬──────────────┘  │
│              │ MCP or API, HTTPS               │ HTTPS           │
│              │ bearer token                    │                 │
└──────────────┼─────────────────────────────────┼─────────────────┘
               │                                 │
┌──────────────▼─────────────────────┐  ┌────────▼─────────────────┐
│ mail-mediator                      │  │ mail-ui                  │
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
│         │  (spent from by ALL  │  every process leases from it   │
│         │   processes)         │                                 │
│         └──────────────────────┘               │                 │
└────────────────────────────────────────────────┼─────────────────┘
                                                 │
┌────────────────────────────────────────────────▼─────────────────┐
│ Persistence                                                      │
│  • Postgres — metadata, senders, plans, audit. NO BODIES.        │
│  • Policy Store — declarative config, hot-reloaded               │
│  • Secrets — external secret store, mounted as files             │
└──────────────────────────────────────────────────────────────────┘
```

| Component | One job | Runs as |
| --- | --- | --- |
| Client Surface (API + thin MCP adapter) | Validate, page, and route client requests by account | mail-mediator |
| Redaction Gate | Decide what survives the last hop before any client | mail-mediator |
| Mutation Authorizer | Enforce per-sensitivity mutation rights on every write | mail-mediator |
| Sender Classifier | Classify senders deterministically against the policy list | mail-mediator |
| Provider Port + adapters | Speak each provider's API; expose one canonical model | mail-mediator |
| Scan Gate | Choose which non-restricted bodies get scanned | batch subsystems |
| Content Scanner | Read bodies it will withhold; emit content-free verdicts | batch subsystems |
| Rate Limiter | Keep all workloads inside one polite per-account budget | all processes |
| Backfill Job | Build the full-history metadata index once | batch workload |
| Delta Sync | Keep the index current against the provider | batch workload |
| Reorg Engine | Turn approved plans into reversible bulk mutations | batch workload |
| Heuristics Job | Propose sensitive-sender candidates for human review | batch workload |
| mail-ui | Make the system legible to the operator; carry the approval verbs | separate deployment |
| Postgres | Hold metadata, plans, and audit — never a body | managed Postgres cluster |
| Policy Store | Hold the sender rules as data, hot-reloaded | GitOps-managed config |

None of it is built; [ROADMAP.md](./ROADMAP.md) holds delivery state.

### How a body request flows

The one narrative to read first, assembled from the decision records (resolved through the
[decision-record index](./docs/adr/README.md), each step restated nowhere else): ADR-0002 — the
fetch-time gate flow that decides allow or deny; ADR-0001 — the field-level matrix behind that
decision; ADR-0007 — the scan states a message can be in, including the accepted residual;
ADR-0029 — the sanitization applied to whatever is released. Metadata requests need none of it:
metadata paths cannot carry a body by construction.

## 2. The pillars

Everything else in this repository is a consequence of these. Each is stated with the reasoning
that holds it up; the decisions that *implement* each one, and their alternatives, are decision
records.

### Metadata always flows; sensitive bodies never do

Every component is judged on whether it holds that line at the only place it can be held reliably:
the last hop before any client, inside a process no client can instruct. Structure — senders,
subjects, threads, labels, dates — is always visible, because organizational capability is the
system's purpose, not a concession. Sensitive content — restricted-sender bodies, MFA codes, login
links — is never released, and no request any client can make unlocks it.

Why: both halves at once are the point — where any other outcome would trade against holding
them, the other outcome loses. And because the organizational patterns are metadata-derivable, the
design can offer strong organizational capability alongside strong content restriction rather than
trading one for the other.

Known limit, stated rather than hidden: metadata itself leaks. Subjects, sender identities, and
traffic patterns are deliberately exposed, and a bounded residual of unscanned bodies is accepted —
both dispositions are recorded and measured (see [Known limits](#3-known-limits)).

### Redaction is enforced by code, not by the provider's token

The mediator's own credentials are always full-mailbox: no available mail credential can be scoped
to exclude senders. The design assumes the token is over-privileged and compensates by making the
redaction path unbypassable, rather than by trusting the credential to constrain anything.

Why: this is the constraint the whole architecture is built around. Because it cannot be delegated
to the provider, enforcement must be a property of code the operator controls — which is why the
mediation layer exists at all. If a provider ever offers genuine per-sender token scoping, the
mediation layer becomes optional and this design should be revisited from the ground up.

Known limit, stated rather than hidden: because the credential cannot be narrowed, a mediator
compromise exposes the full mailbox — the trust-anchor pillar carries that consequence. Where a
provider *does* offer narrowing, capability absent from the granted credential (such as permanent
delete) is a real guarantee, and the design takes it as defense-in-depth wherever it exists.

### One gate, N dumb adapters

Redaction happens in exactly one place: above the provider adapters and below every client-facing
surface. Adapters return full data by design; the Redaction Gate decides what survives. No
adapter, frontend, tool handler, or batch path holds its own copy of the rules.

Why: redaction can only be enforced above the provider adapter and below the client surface — no
available mailbox credential can be scoped to exclude senders, and the client surface is where
callers instruct. Adapters return full data by design; the gate decides what survives — so a bug
in adapter #3 cannot become a silent leak, because the adapter never had redaction responsibility
to get wrong.

Known limit, stated rather than hidden: a single chokepoint concentrates correctness — a bug in the
gate is a bug everywhere. That is the accepted trade; it is why the gate is built first, tested
against its failure modes explicitly, and kept small.

### Fail closed, everywhere

Every ambiguous or error state — unreadable policy, scanner backlog, classification failure,
unscanned message — resolves to *deny the body*. "Allow" requires an affirmative safe
classification; the mere absence of a positive signal is never enough. Work that has not happened
yet is a deny state, not an open door.

Why: the failure directions are asymmetric. Over-redaction is an inconvenience the operator can see
and tune; under-redaction is a leak that cannot be recalled. A system whose degraded modes all
point toward deny degrades in utility, never in safety.

Known limit, stated rather than hidden: fail-closed paths are exercised by tests or not at all —
production never visits them until the day it matters. Their proving injections are catalogued in
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md).

### Sensitivity is two independent axes

Who sent a message and what the message contains are separate questions with separate
consequences. Sender class governs readability *and* disposal rights; content flags govern
readability only. The axes are independent in that either one can deny a body on its own, and only
sender class constrains disposal. They compose by union: restrictions accumulate and never cancel.

Why: collapsing the axes would force sensitivity to mean one thing when it means two. A message can
be unreadable yet fully disposable (an expired MFA code from an ordinary sender is exactly the
clutter the agent should archive), and unreadable yet untouchable-for-disposal (a restricted
financial notice the agent files but can never trash). Each useful combination exists; a
one-dimensional model would forbid some of them by accident.

Known limit, stated rather than hidden: union composition means restrictions only accumulate — a
message over-restricted by either axis stays restricted until the policy or verdict behind it is
corrected. Over-redaction is the accepted failure direction, and the masking and gate review loops
exist to tune it from observed traffic.

### Approval is not in any client's vocabulary

The transition that authorizes a bulk mailbox change is performed by the operator, on a surface
no client can reach. No MCP tool and no API endpoint performs it, so no sequence of client
actions — however persuaded, however prompt-injected — can manufacture consent. Clients propose;
the human disposes.

Why: attacker-controlled text flows into the agent's planning input by design (subjects are always
visible, and non-sensitive bodies flow). Instructing the agent to be careful is not a control. The
control is structural: the approval verb does not exist on the client-facing surface, so talking
the agent into anything changes nothing.

Known limit, stated rather than hidden: the approval surface itself becomes a target — compromise
of it means malicious plans can be approved. Its write surface is kept minimal and its blast radius
bounded by independent enforcement at apply time; the decisions doing that bounding are recorded as
decision records.

### Unsafe states are unconstructable, not merely untaken

Where a leak is possible, the design removes the type, field, column, or verb that could carry it,
rather than adding a check that declines to use it. Scanner verdicts have no field a body could
hide in. Metadata fetch paths cannot return content. The message store has no body column. The
client surface has no permanent-delete operation, and neither does the granted token.

Why: an unsafe state that is merely untaken can still be taken; one that cannot be represented
cannot be reached at all — there is no field a body could hide in, no verb a permanent delete could
ride on.

Known limit, stated rather than hidden: not every unsafe state can be made unrepresentable — bodies
must transit mediator memory to be served and scanned at all. Where representation is unavoidable,
the design falls back to fail-closed checks plus audit, and says so explicitly.

### Everything above the port speaks canonical

A single provider port defines the system's model of mail and calendar. Everything above it — gate,
classifier, authorizer, client surface, batch workloads — speaks that canonical model; the adapters
below it are the only code that knows a Gmail label from a JMAP mailbox. Adding a backend means
writing one new adapter, not redesigning the system.

Why: without the boundary, provider-specific concepts — label IDs, query syntax, mailbox trees,
sync-state strings — appear above the adapter layer, and switching backends becomes a redesign
instead of one new adapter. With it, each adapter compiles the differences away, and the redaction
gate, classifier, and client surface never contain a branch on provider identity.

Known limit, stated rather than hidden: the abstraction is a design intention until the second
adapter is built; the contract is only proven when a backend swap forces no change above the port.
That deferred test is deliberate and tracked in [ROADMAP.md](./ROADMAP.md).

### Accounts are isolated by structure, not convention

The architecture is multi-account even while one account is deployed. Every operation names its
account explicitly — there is no implicit current account. Every table, query, client, and
credential is account-scoped by construction, and accounts may span organizations: no shared OAuth
client, no delegation, no assumed common administrator.

Why: cross-account bleed is the class of bug that convention cannot hold against concurrency — a
shared client with a mutable auth header fails exactly when two accounts are active at once.
Structural scoping makes the bleed unrepresentable rather than unlikely, and building it in from
the start costs little while retrofitting it later costs a redesign.

Known limit, stated rather than hidden: like the port abstraction, isolation is proven only when
the second account exists; until then it is enforced structure awaiting its test.

### An accepted risk that is not measured is an unmeasured risk

Every deliberate compromise in this design — each accepted residual, each deliberate exposure — is
recorded with its reason and instrumented so its actual size is observable. Every body served,
every denial, every mutation, every skip is audited. Acceptance without measurement is not
acceptance; it is blindness with a rationale.

Why: the recorded skips, masks, serves, and denials are what let the operator tune each compromise
from evidence rather than intuition — and a body served, denied, or mutated without a record would
be unanswerable after the fact.

Known limit, stated rather than hidden: a metric not collected for a window already passed is lost
unrecoverably, so instrumentation must exist before the window it will be asked about — an ordering
constraint on the work, carried by [ROADMAP.md](./ROADMAP.md).

### Network position never substitutes for the gate

The deployment keeps the endpoint off the internet and the agent inside the LAN — an exposure
choice, recorded as a decision — but the redaction invariant does not depend on network position.
A caller on the LAN, or malware on a laptop on the LAN, reaching the client surface gets exactly
what the gate permits and nothing more.

Why: LAN-only scope is a meaningful simplification — it removes the entire class of
"someone on the internet reaches the mediator" risk — but it is not a substitute for the gate, and
the invariant does not depend on it. Egress restriction is a different matter: the
anti-exfiltration control, and it matters *more* than ingress restriction, because the realistic
attack is a prompt-injected agent trying to send data *out* — and a suborned agent with nowhere to
send data is contained even when persuaded.

Known limit, stated rather than hidden: the transport model does assume the agent is inside the
LAN; running an agent from outside reopens the ingress question as a decision to make, not a hole
to patch.

### The mediator is the irreducible trust anchor

The mediation layer holds full mailbox credentials; if its process is compromised, redaction is
moot — the attacker calls the provider directly. The design does not pretend otherwise. The stance
is: this anchor is hardened, its blast radius is understood, and evidence of its compromise
survives outside its own reach.

Why: some process must hold the over-privileged credential — that follows from redaction being
enforced by code, not the token. The design accepts this as the irreducible trust anchor rather
than pretending otherwise, and directs the hardening there.

Known limit, stated rather than hidden: this is the one failure the design cannot make
unrepresentable, only expensive, detectable, and evidenced — which is why the audit trail must ship
somewhere a compromised mediator cannot erase.

### Concerns stay un-braided; components know only their contracts

Nothing in the design or the implementation may inhibit future evolution — including evolution
arriving after the entire currently-envisioned product is built and working. Two rules carry
that. Choices are judged on whether they braid together concerns that will need to vary
independently, and the familiar-but-complected option never wins over the simple-but-harder one.
And no component assumes or encodes anything about how other components or subsystems operate
beyond what its contract with them says — contracts are the only licensed knowledge; everything
else is somebody else's business.

Why: the design's deliberate absences — absent verbs, absent columns, a minimal write surface —
are explicit, decision-recorded, and extendable any time by minting a new decision through the
front door; they do not inhibit evolution. What inhibits it is implicit coupling: a component
quietly relying on how another behaves today, captured in no record and found by no future reader
until it breaks their extension. The architecture already instantiates the principle — the
provider port, the separation of the gate from the authorizer, the four separate workloads — and
this pillar makes it binding on every decision and every line of code to come, which is why
decision records name their cross-component assumptions in their Consequences.

Known limit, stated rather than hidden: like the backend swap, this is only truly tested when an
evolution actually arrives; until then it is enforced structure awaiting its test.

## 3. Known limits

The pillars state their limits in place; this table exists so the full set — every pillar's limit,
plus the standing assumptions that attach to no single pillar — is checkable in one place. Rows say
where each disposition is recorded, not what it is — the record named is the single home.

| Limit | Disposition lives in |
| --- | --- |
| Metadata (subjects, senders, traffic patterns) is deliberately exposed | ADR-0001, via the [decision-record index](./docs/adr/README.md) |
| A bounded residual of unscanned bodies is released by design | ADR-0007, and its measurement rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| A single chokepoint concentrates correctness — a gate bug is a bug everywhere | Built first, proven offline: the S1 unit in [ROADMAP.md](./ROADMAP.md) and its rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| Fail-closed paths are exercised by tests or not at all | Their injections in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| Union composition means over-restriction stands until its policy or verdict is corrected | The masking and gate review loops — ADR-0003, ADR-0007 |
| The approval surface is itself a target | ADR-0021 — two verbs, scoped role, no credentials |
| Bodies must transit mediator memory to be served and scanned at all | ADR-0009 |
| A metric not collected for a past window is lost unrecoverably | [ROADMAP.md](./ROADMAP.md) — emission is a non-deferrable riding every unit |
| Content released to the agent is released — context, transcripts, memory | ADR-0029 bounds it; it cannot be recalled |
| Mediator compromise defeats redaction | ADR-0028 — hardening, blast radius, off-cluster evidence |
| Backend-swap and multi-account isolation are unproven until a second adapter/account exists | [ROADMAP.md](./ROADMAP.md), as the units that run those tests |
| Un-braided concerns and contract-only knowledge are only truly tested when an evolution arrives | The records' assumption-naming convention ([docs/adr/README.md](./docs/adr/README.md)) |
| The corpus is assumed ≤100k messages per account | ADR-0016 records what changes beyond it |
| The agent is assumed to run inside the LAN | ADR-0014 records what reopens if it does not |

## 4. Failure modes

The system's known failure modes with their qualitative assessments, enumerable in one place. Rows
are pointers: each fix and its reasoning live in the records named, never here.

| Failure mode | Likelihood / impact | Disposition lives in |
| --- | --- | --- |
| Prompt injection via a released body | near-certain / high | ADR-0029; ADR-0002 (the gate is the control); ADR-0014 (egress) |
| Injection aimed at the reorg plan through visible subjects | moderate / high | ADR-0020 |
| Sender spoofing — the unlisted co-brand domain | moderate / high | ADR-0004 |
| Policy-list staleness | certain over time / medium | ADR-0004 |
| Metadata leakage | certain, accepted / medium | ADR-0001 |
| Scan-gate residual leakage | accepted / medium | ADR-0007 |
| Mediator compromise | low / catastrophic | ADR-0028 |
| Fail-open on classifier or scanner error | low / severe | ADR-0002; the fail-closed rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) |
| Agent context as an exfiltration surface | moderate / medium | ADR-0029 |
| Bulk mutation error | moderate / severe | ADR-0020 |
| Scan backlog as silent utility loss | low / moderate | ADR-0007 |
| UI as a write path | low / moderate | ADR-0021 |
| Rate-controller pathology — collapse or runaway | moderate / medium | ADR-0024; ADR-0025 |

## Glossary

The single home for vocabulary. A cold reader should be able to resolve any term used in the three
top-level documents, a decision record, or a ticket from here without guessing.

### The system and its parties

- **The invariant** — the fixed point of [USE_CASES.md](./USE_CASES.md), used as a proper noun
  throughout the documents: full organizational visibility and zero sensitive content, held at
  once.
- **The mediator** (`mail-mediator`) — the process that holds provider credentials, enforces
  redaction, and serves the client surface. The trust anchor.
- **Client** — any caller of the serving surface: the agent over MCP, or any caller of the API.
  Every client is untrusted by design: every control assumes a client can be talked into, or
  built to attempt, anything.
- **The client surface** — the mediator's serving surface: the API plus its MCP adapter (shape
  and rules: ADR-0030, via the [decision-record index](./docs/adr/README.md)).
- **The agent** — the primary client today: the MCP-speaking assistant given the mailbox to
  organize (Claude Code or similar).
- **The operator** — the single human who owns the infrastructure, edits policy, and approves
  plans.
- **The UI** (`mail-ui`) — the read-mostly reporting and approval surface. Separate deployment,
  separate identity, no provider credentials; carries the approval verbs no client has.
- **Provider** — the managed service actually holding the mail or calendar (Gmail, Fastmail).

### Sensitivity and redaction

- **Sender class** — the per-sender axis: `normal` or `restricted`. Decides disposal rights, and
  a restricted class denies the body on its own. Assigned deterministically from the policy list,
  never by heuristic.
- **Restricted sender** — a sender matching the operator's deny list. Bodies never released;
  disposal verbs never authorized.
- **Content flag** — the per-message axis: a detected MFA code or login link. Decides readability;
  any flag denies the body.
- **Scan state** — a message's position relative to the content scanner: pending, scanned, or one
  of the deliberate skip states. "Not scanned" has distinct meanings with different consequences;
  pending always denies.
- **Redaction Gate** — the mediator component that decides, at the last hop, which fields of a
  message a client receives. Distinct from the *Scan Gate* below.
- **Scan Gate** — the batch-side predicate that decides which non-restricted bodies are worth
  scanning. Distinct from the *Redaction Gate* above: the Redaction Gate guards release to
  clients; the Scan Gate budgets scanning work.
- **Content Scanner** — the component that reads bodies it intends to withhold, detects codes and
  links, and emits verdicts that structurally cannot carry content.
- **Masking** — replacing a detected secret inside an otherwise-visible field (an MFA code in a
  subject) with an opaque placeholder.
- **The residual** — the accepted, measured set of bodies released without having been scanned:
  non-restricted sender, secret present, zero metadata signal.

### Policy and classification

- **Policy list / gazetteer** — the operator-editable, deterministic sender-classification rules,
  managed as data under GitOps and hot-reloaded. The only authority on sender class.
- **Policy overlay** — per-account additions to the base policy. Overlays only add restrictions;
  they can over-restrict, never under-restrict.
- **Heuristics Job** — the batch workload that proposes sensitive-sender candidates from observed
  traffic. Proposes only; nothing it emits takes effect without operator confirmation.
- **Review queue** — where heuristic candidates wait, ranked with their evidence, for the operator
  to confirm or dismiss through the UI.
- **Tier** — a stage of content detection, ordered by cost: structural patterns, then scoring, then
  (deferred) a small local model. Higher tiers see only what lower tiers could not settle.

### Provider abstraction and accounts

- **Provider Port** — the single interface the system's canonical model is defined against.
  Everything above it is provider-blind.
- **Adapter** — a per-provider implementation of the port; the only code that knows provider
  concepts. Adapters are deliberately dumb: full data in, no redaction responsibility.
- **Canonical model** — the provider-neutral representation of messages, threads, labels, queries,
  and changes that everything above the port speaks.
- **Account context** — the per-account bundle of provider clients, credentials, policy overlay,
  and rate state. Nothing about an account is ambient; every operation names one.

### Data paths and mutation

- **Backfill** — the one-time construction of the full-history metadata index, metadata first,
  gated body scanning after.
- **Delta sync** — the recurring job that keeps the index current against provider change feeds.
- **Reorg plan** — a proposed bulk reorganization, stored as data: label operations, per-message
  operations, reasoning, and scale. A plan is never an action.
- **Op log** — the per-operation before/after record written during plan application, from which
  rollback is exact replay, not inference.
- **Mutation Authorizer** — the mediator component that checks every write against the message's
  sensitivity, independently of who approved what.

### Operability

- **Rate profile** — an adapter's declaration of what operations cost on its provider and what
  budget exists. The limiter is provider-agnostic; only adapters know costs.
- **Priority class** — the division of the per-account rate budget between interactive, sync, and
  batch work, so background work can never starve interactive work.
- **Lease** — a short-lived allocation of rate budget to one process, the mechanism by which
  separate workloads share one per-account budget without a coordinator process.
- **Audit log** — the record of every body served, every denial, and every mutation. Must survive
  mediator compromise, so it ships off-cluster.
- **Masking events / gate decisions** — the per-event records that make masking behavior and scan
  skips tunable from evidence.

### Doctrine

- **Violation injection** (also *proving injection*) — the acceptance standard: a control is
  proven by deliberately creating the violation it exists to stop and watching it fire, never by
  observing that nothing bad happened. Catalogued in
  [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md).
- **Answerable-by-doing** — an open empirical question that needs an experiment or accumulated
  data rather than a build, registered with its trigger point so it cannot evaporate.

### Identifiers

- **Outcome identifiers** — C1–C3 (the invariant), G1–G3 (organizational capability), P1–P3
  (provider abstraction), A1–A3 (safe action), O1–O3 (operability): defined in
  [USE_CASES.md](./USE_CASES.md) and used as the coordinate system everywhere else.
- **ADR-NNNN** — decision records, numbered globally in mint order, resolved through
  [docs/adr/README.md](./docs/adr/README.md).
- **Work units and value increments** — defined in [ROADMAP.md](./ROADMAP.md).
