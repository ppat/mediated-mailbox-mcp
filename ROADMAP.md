# Mediated Mailbox MCP — Roadmap

This file holds all the work in one place, meaning the delivery posture, the value path, the work
units, their dependencies, and the open decisions. It is the one top-level document that changes
as work progresses. [USE_CASES.md](./USE_CASES.md) and [DESIGN.md](./DESIGN.md) are the stable
contract it is measured against.

**Reading rules.** Status distinguishes *authored → merged → released → deployed-and-observed*,
four different states, never collapsed (`main` is not deployed state, in either direction). Claims
are **[measured]** (read from the repo, an API, or a record that records its own measurement) or
**[inferred]**. Effort notes on units (≈ days) are sizing guesses, not commitments.

**Acceptance.** Every control's proving injection lives in
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), keyed to the units below. A unit is not done
while its pending verification rows are unproven. An automatable control's acceptance also
includes the mutation demonstration recorded in
[docs/MUTATIONS.md](./docs/MUTATIONS.md) (ADR-0046).

**Identifiers.** Outcomes (C1–C4, G1–G4, P1–P3, A1–A4, O1–O6) are defined in
[USE_CASES.md](./USE_CASES.md). Work units (S·F·D·M·X·H + number) and value increments (V1–V6) are
defined here. Decisions are cited by number and resolved through the
[decision-record index](./docs/adr/README.md). Every reference in prose links to the section
defining it. Table cells and dependency edges within this document may use bare identifiers.

**How this document relates to tickets.** No tickets exist yet. As units are cut into tickets, the
rule binds both directions. Every ticket names the one unit it serves, every unit here names its
tickets, and the **Position** line below is re-dated whenever the checklists are reconciled
against the tickets, so staleness is detectable instead of silent.

**Position: 2026-09-11.**

## Delivery posture

Three commitments govern how every unit is scoped and sequenced. They exist to pre-empt a known
failure mode, an implementation effort that tries to account for every minute thing before
shipping and takes forever to put value in front of its user.

1. **Deliver value faster.** Each increment on the value path is cut at the smallest shape that
   ships real value, not at capability-complete.
2. **Learn from production; iterate on learnings.** First passes are deliberately minimal, and
   what running against the real mailbox teaches drives the next pass. Discoveries that do not
   block the value path become tickets, not scope.
3. **Harden and tighten later, once real behavior exists.** Broader drills, dashboards, and the
   learned detection tier sit in a deliberate late band, built against observed behavior rather
   than guesses, which is also what makes them cheap.

Four things are **not** deferrable under this posture. The membership test is that *deferral is
irreversible, or the failure it permits is silent*.

- **The fail-closed safeguard machinery before any body flows** ([V1](#v1--the-safeguard-exists-before-anything-flows)
  precedes everything). A gate bug fails catastrophically and silently. It cannot be "learned
  from" in production because nothing visibly breaks when it leaks.
- **Credential-rotation writeback, exercised deliberately in [F1](#group-f--foundation)** — its
  failure surfaces only at the restart after a rotation, the classic quiet death of systems like
  this.
- **The rate hard cap before the first corpus-scale workload** ([F3](#group-f--foundation) precedes
  [D1](#group-d--data-flows)) — collectively overrunning the provider's budget risks a
  provider-side account restriction on the operator's personal mailbox, which is not recoverable
  by iteration.
- **Audit and measurement emission riding as acceptance criteria on the units being built anyway**
  (gate decisions, masking events, audit rows, rate gauges) — an uninstrumented window is gone
  forever, and the design refuses unmeasured accepted risks.

The same posture bounds quality scope in the other direction. First passes of detection rules,
gate predicates, and heuristics are expected to be tuned from observed traffic via the review
loops, not perfected up front.

## Where things stand, in one table

| Layer | State |
| --- | --- |
| Documents (design, outcomes, decisions, this roadmap, verifications, mutations) | Authored **[measured]** |
| Code | None exists |
| Infrastructure (database, secrets sync, deployments) | None provisioned for this system |
| Verifications | All pending or parked (see [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md)) |
| Mutations | Nothing demonstrable yet. The ledger stays empty until implementation. See [docs/MUTATIONS.md](./docs/MUTATIONS.md) |
| **The delivery gap** | Everything. No unit has started, and V1 is the front of the line |

## Delivered, mapped to outcomes

Nothing is delivered. This roadmap predates the first line of code, and the register starts
empty, deliberately.

What does **not** exist yet, stated so a cold reader does not assume otherwise. There is no API or
MCP endpoint, no index, no gate, no deployment, and no proven property of any kind. Every claim in
the design is authored, and none is yet demonstrated by code in this repository.

## The value path

```mermaid
flowchart TB
    V1["V1 — the safeguard exists\nbefore anything flows"]
    V2["V2 — the corpus becomes visible"]
    V3["V3 — the agent arrives, read-only"]
    V4["V4 — the agent acts"]
    V5["V5 — a second of everything"]
    V6["V6 — hardening and the learned tier"]
    V1 -->|"the index is built by machinery whose failure modes are already proven"| V2
    V2 -->|"the agent's first queries land on a corpus that already exists"| V3
    V3 -->|"mutation opens only after the read invariant survived a live adversary"| V4
    V4 -->|"contracts are tested by a second implementation only after one vertical works"| V5
    V5 -->|"hardening and training built against observed behavior, not guesses"| V6
```

### V1 — The safeguard exists before anything flows

**Units:** [S1](#group-s--safeguard-machinery) · [S2](#group-s--safeguard-machinery) ·
[F1](#group-f--foundation).
**Value shipped:** the two failure modes that would quietly kill the project (a leaking gate and
a decaying credential) are proven survivable, offline and cheaply, before anything is built on
top of them.
**Why it is first:** the gate is the only component that fails catastrophically *and* silently,
so it is built where correctness is provable against fixtures. And if refresh-token durability
breaks this design, it breaks in a half-day spike instead of mid-backfill.

### V2 — The corpus becomes visible

**Units:** [F2](#group-f--foundation) · [F3](#group-f--foundation) ·
[D1](#group-d--data-flows) · [D2](#group-d--data-flows).
**Value shipped:** the full-history metadata index exists (every sender classified, every subject
masked, sender statistics built, scan verdicts recorded), acquired politely enough to never
antagonize the provider.
**Why here:** the index is the instrument every later acceptance depends on. And backfill is where
the canonical mapping meets the real corpus, where a flaw costs a re-run now versus a redesign
after agent workflows exist.

### V3 — The agent arrives, read-only

**Units:** [D3](#group-d--data-flows) · [S3](#group-s--safeguard-machinery) ·
[D4](#group-d--data-flows).
**Value shipped:** the first real user value, an agent doing whole-mailbox analysis over live,
current data, with the invariant holding against a live adversary (the operator deliberately
trying to talk it into a restricted body).
**Why here:** connecting the agent read-only is the first end-to-end proof of the invariant
against a real adversary. Mutation capability opens only after that proof exists.

### V4 — The agent acts

**Units:** [M1](#group-m--mutation-and-approval) · [M2](#group-m--mutation-and-approval) ·
[M3](#group-m--mutation-and-approval) · [M4](#group-m--mutation-and-approval).
**Value shipped:** the system's headline capability (organize, then propose and enact a
mailbox-wide reorganization) with approval, rollback, and the review loops that keep the policy
list alive.
**One disposition inside this band:** approval is command-line-only until M3
lands. The UI is what makes review humane, and it arrives inside the same band as the engine it
reviews.

### V5 — A second of everything

**Units:** [X3](#group-x--expansion) · [X4](#group-x--expansion) · [X2](#group-x--expansion).
**Value shipped:** the second account and the second mail backend (the deferred proofs of the
account model and the provider contract), plus calendar joining mail behind the same invariant.
**Why here:** each is the real test of a contract authored earlier. If anything above the port
must change to accommodate it, the contract was wrong, and finding that out cheaply is the point.

### V6 — Hardening and the learned tier

**Units:** [H1](#group-h--hardening) · [X1](#group-x--expansion).
**Value shipped:** dashboards and alerts over the metrics that have been accumulating since V2,
admission-enforced pod policy, backup verification, plus the learned detection tier, trained on
the labeled data the earlier bands produced.
**One disposition inside this band:** deferring dashboards is a deliberate trade. The
*emission* is non-deferrable and has been riding every unit. The *presentation* is cheap to pull
forward any time.

## Remaining work — the units

Each unit serves exactly one outcome. Where a unit's machinery also carries another outcome's
acceptance criteria, that rides in *Criteria:* rather than splitting the unit. Checkboxes are the
only state marker. Nuance lives in prose.

### Group S — safeguard machinery

The invariant's enforcement components, built first, offline, against fixtures. No network, no
database, no provider.

- [ ] **S1 — Redaction Gate + Sender Classifier + Mutation Authorizer, isolated** →
  [C2](./USE_CASES.md#c2--sensitive-sender-content-never-released) · V1 · ≈1–2 days
  Property test that no message-body value with non-normal sensitivity or a denying scan state can
  even be constructed. Fail-closed paths tested **first**, because production never exercises
  them. Types carry sensitivity so a leak is a static type error, with a strict type checker in CI
  as the enforcing gate. *Criteria:* every deny writes an audit row. An invalid policy update
  never takes effect. The active policy keeps serving and the reload-failure alarm fires
  (ADR-0041).
- [ ] **S2 — Content Scanner tiers 1–2 + subject masking** →
  [C3](./USE_CASES.md#c3--content-based-secrets-caught) · V1 · ≈1–2 days
  Fixture-driven. Verdict-cannot-carry-content is verified by type and by a test that searches
  scanner output and logs for fixture body text. The real MFA-format corpus gets built from the
  operator's own mail during D1. *Criteria:* masking events recorded from the first run.
- [ ] **S3 — Body sanitization + injection hardening** →
  [A4](./USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything) · V3 · ≈1 day
  HTML-to-Markdown conversion via an existing library, untrusted-content delimiters, the
  serve-time pattern check on unscanned bodies, anomalous body-fetch alerting.
  *Criteria:* the serve-time check denies on a hit from the first serve, with an audit row.

### Group F — foundation

What everything runs on. Credentials, store, adapter, budget.

- [ ] **F1 — Auth spike** → [O3](./USE_CASES.md#o3--survives-its-failure-modes) · V1 · ≈½ day
  No client surface, no redaction, no database. Obtain the Gmail refresh token, store it, mount
  it via the secret-sync path, make one authenticated list call, and **deliberately exercise
  rotation writeback**. *Criteria:* writeback survives a pod restart.
- [ ] **F2 — Postgres + schema + Gmail adapter, metadata path** →
  [G1](./USE_CASES.md#g1--whole-mailbox-visibility) · V2 · ≈2 days
  Metadata-only fetches throughout. Seed a sensitive fixture and verify no snippet survives the
  gate. Confirm account-scoped query enforcement, the append-only audit grants, and the
  operation log's plan-scoped policy before there is data to migrate. This unit carries the data
  layer's tooling (ADR-0066, ADR-0067, ADR-0068) and most of its controls.
- [ ] **F3 — Rate limiter + Gmail cost profile** →
  [O1](./USE_CASES.md#o1--rate-limited-politely) · V2 · ≈1 day
  Cost profile, adaptive controller, priority classes, database lease coordination, all tested
  against a **simulated** provider that throttles on schedule, so convergence and recovery are
  proven before pointing at the real one. Verify the hard cap holds when the controller is fed
  deliberately bad inputs. *Criteria:* rate, lease, and throttle gauges emitting.

### Group D — data flows

The index and the surfaces that read it. This group carries
[O5](./USE_CASES.md#o5--clients-can-tell-failures-apart)'s failure-transparency criteria inside
D3, flagged here rather than split, because the error contract and the surface are built as one
piece.

- [ ] **D1 — Backfill pass 1, full history** →
  [G2](./USE_CASES.md#g2--historical-understanding) · V2 · ≈1–2 days
  Metadata, sender classification, subject masking, sender aggregates. Validates rate limiting
  under real conditions, checkpoint/resume, and the canonical mapping against messy real data.
  The unit brings a second deployable's code into the repository, the point where the
  cross-deployable import rule (ADR-0054) first has a violation to construct.
  **Deliberately kill the pod mid-run and confirm clean resume.** Watch the rate gauge throughout.
  This run is where the real ceiling for this account reveals itself, as opposed to the documented
  one. *Criteria:* the run, its progress events, and its per-item failures are recorded in the
  job tables from the first page (ADR-0022).
- [ ] **D2 — Scan gate + backfill pass 2** →
  [C3](./USE_CASES.md#c3--content-based-secrets-caught) · V2 · ≈1 day
  The composite gate evaluated with pass-1 statistics, gated body scanning, skip decisions
  recorded from the first evaluation, and the delisting transition (ADR-0037). Review skip rates
  before trusting the compromise. *Criteria:* pass 2 records its runs, progress events, and
  per-item failures (ADR-0022).
- [ ] **D3 — Client surface (API + thin MCP adapter), read-only** →
  [G1](./USE_CASES.md#g1--whole-mailbox-visibility) · V3 · ≈1–2 days
  The canonical API contract (OpenAPI) with its one-to-one MCP mirror, exposing `list_threads`,
  `get_message_metadata`, `get_message_body`, `search_messages`, `list_policy_rules`,
  `corpus_stats`, a labels listing and an accounts listing (ADR-0035), and a per-account
  system-status read (ADR-0034). TLS and bearer auth on both roots. Connect the real agent.
  **Manually attempt to
  talk the agent into a restricted body**, the first end-to-end proof of the invariant against a
  live adversary. *Criteria:* every serve and denial audited. API operations and MCP tools match
  one-to-one. Timestamps on both roots are UTC-only, with non-UTC input rejected. No operation
  requires an identifier the surface's read operations cannot supply. Failure responses let a
  client tell apart its own failure, the system's, and the provider's.
- [ ] **D4 — Delta sync** →
  [G4](./USE_CASES.md#g4--the-index-tracks-the-live-mailbox) · V3 · ≈1 day
  Cursor management, gap detection and bounded recovery, idempotency. Run alongside backfill for a
  few days and reconcile counts against the provider to catch drift. *Criteria:* every tick and
  every gap recovery is a recorded run, and the cursor's write time is recorded with it
  (ADR-0022, ADR-0016).

### Group M — mutation and approval

The write path, in escalating blast radius. This group carries
[A3](./USE_CASES.md#a3--bulk-change-is-reversible)'s machinery inside M2 and
[G3](./USE_CASES.md#g3--reorganization)'s plan review view inside M3, flagged here rather than
split, because the engine and its reversibility, and the UI and its views, are each built as one
piece.

- [ ] **M1 — Mutations, non-reorg** →
  [A1](./USE_CASES.md#a1--asymmetric-mutation) · V4 · ≈1 day
  Single and batch label/move/archive under the authorization matrix. Dry-run first. Verify
  provider effects after mutating. *Criteria:* mixed-class batch with a disposal verb fails
  whole. A batch with an invalid operation anywhere in it fails whole before any write. Every
  mutating operation offers a dry-run mode that changes no state of any kind.
- [ ] **M2 — Reorg engine** →
  [G3](./USE_CASES.md#g3--reorganization) · V4 · ≈3 days
  Plan storage and validation (at creation, re-validation at apply, the plan-age check), the
  `describe_reorg_plan` / `sample_reorg_plan` tools, checkpointed apply, op log, rollback.
  **Test rollback on a real ~1000-message plan before trusting it on 40k.** Approval is
  command-line-only until M3. *Criteria:* saving a plan writes its operations as rows with their
  flows, and apply and rollback runs are recorded with their per-operation failures (ADR-0020,
  ADR-0022).
- [ ] **M3 — Reporting + approval UI** →
  [O4](./USE_CASES.md#o4--the-operator-can-see-and-steer) · V4 · ≈2–3 days
  Corpus overview, plan diff and approval, review queue, masking events, gate decisions, audit view,
  the live jobs view, and the failed-run drill-down, built to the design in
  [docs/UI.md](./docs/UI.md). Separate deployment and scoped database role. *Criteria:* the
  accepted-residual and masking loops become operator-reviewable. Message-derived text renders
  inert, the dataset registry refuses what it does not declare, and no lens answers without an
  account (ADR-0056, ADR-0057). The content security policy holds (ADR-0062), and a verb without its
  request token (ADR-0061) or its declared identity (ADR-0021) is refused. No UI path writes a
  decision's status without its companion columns and, on confirm, its rule row (ADR-0060). The
  framework spike of [docs/UI.md](./docs/UI.md#16-framework-requirements) ran on 2026-09-10
  **[measured]**, outside this repository and on the chosen candidate, so ADR-0063 weighs its result
  and nothing from it is code here. The order of work within the unit is the registry and its
  endpoint with the contract pipeline (ADR-0065), the recorded fixtures (ADR-0064), the ladder and
  rows proven on masking events, the five lenses, the home, the review queue's reads, the plan
  reviewer and plans list, jobs and the run, policy and system, and the four decision requests last
  because they are the only writes.
- [ ] **M4 — Heuristics job + embeddings** →
  [C4](./USE_CASES.md#c4--the-sensitive-sender-list-keeps-pace) · V4 · ≈1–2 days
  Candidate generation into the review queue. Needs M3 to be useful, hence after it. *Criteria:*
  each run is recorded (ADR-0022).

### Group X — expansion

Each unit is a deliberate test of a contract authored long before it.

- [ ] **X1 — Scanner tier 3** →
  [C3](./USE_CASES.md#c3--content-based-secrets-caught) · V6 · ≈1–2 days
  Only once a few hundred confirmed examples exist from real traffic. Train, evaluate against
  held-out fixtures, ship behind the scanner-version flag so rollback is trivial.
- [ ] **X2 — Calendar** →
  [C1](./USE_CASES.md#c1--metadata-always-visible) · V5 · ≈2 days
  Calendar port + Google Calendar adapter, attendee-domain classification.
- [ ] **X3 — Second account** →
  [P3](./USE_CASES.md#p3--multi-account) · V5 · ≈½ day
  The real test of the account model. If anything above the port needs changing, the model was
  wrong, and finding out here, cheaply, is the point.
- [ ] **X4 — Fastmail adapter** →
  [P2](./USE_CASES.md#p2--backend-swap) · V5 · ≈2–3 days
  The real test of the provider contract, same principle one layer down, including discovering
  the provider's actual throttling behavior, which is undocumented.

### Group H — hardening

- [ ] **H1 — Operational hardening** →
  [O2](./USE_CASES.md#o2--observable) · V6
  Dashboards and alerts over the long-emitting metrics (deny rate, unclassified-sender volume,
  scan backlog, gate skip rate, body-fetch rate, rate-controller gauges), admission-enforced pod
  policy, backup verification. Carries [O3](./USE_CASES.md#o3--survives-its-failure-modes)'s
  remaining drills (flagged, not split).

### The mapping at a glance

| Outcome | Remaining units | Gaps |
| --- | --- | --- |
| [C1](./USE_CASES.md#c1--metadata-always-visible) metadata visible | X2 | Mail-side visibility is built by F2 and D3 (G1 units). X2 is the calendar half |
| [C2](./USE_CASES.md#c2--sensitive-sender-content-never-released) content never released | S1 | Injection hardening on released bodies rides S3, now serving A4. The calendar content-release side rides X2 |
| [C3](./USE_CASES.md#c3--content-based-secrets-caught) secrets caught | S2 · D2 · X1 | The serve-time check rides S3 (an A4 unit) |
| [C4](./USE_CASES.md#c4--the-sensitive-sender-list-keeps-pace) list keeps pace | M4 | — |
| [G1](./USE_CASES.md#g1--whole-mailbox-visibility) whole-mailbox view | F2 · D3 | — |
| [G2](./USE_CASES.md#g2--historical-understanding) historical understanding | D1 | — |
| [G3](./USE_CASES.md#g3--reorganization) reorganization | M2 | The plan review view rides M3, flagged in Group M's preamble |
| [G4](./USE_CASES.md#g4--the-index-tracks-the-live-mailbox) index tracks live | D4 | — |
| [P1](./USE_CASES.md#p1--one-contract) one contract | — | No dedicated unit, correctly. The contract is authored in the decision records and proven by X4 |
| [P2](./USE_CASES.md#p2--backend-swap) backend swap | X4 | — |
| [P3](./USE_CASES.md#p3--multi-account) multi-account | X3 | The identifier-discoverability criterion (ADR-0035) rides D3 |
| [A1](./USE_CASES.md#a1--asymmetric-mutation) asymmetric mutation | M1 | — |
| [A2](./USE_CASES.md#a2--no-destructive-action-on-sensitive-mail) no destructive action | — | No dedicated unit, correctly. One structural half lands with F1 (token scope), the other with M1 (client surface). Criteria ride those units |
| [A3](./USE_CASES.md#a3--bulk-change-is-reversible) reversible bulk change | — | Carried inside M2, flagged in Group M's preamble |
| [A4](./USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything) harmless released bodies | S3 | — |
| [O1](./USE_CASES.md#o1--rate-limited-politely) rate-limited | F3 | — |
| [O2](./USE_CASES.md#o2--observable) observable | H1 | Emission rides S1 · S2 · F3 · D2 · D3 as criteria. H1 is presentation |
| [O3](./USE_CASES.md#o3--survives-its-failure-modes) survives failure | F1 | Drills ride D1 (kill mid-run) · F3 (rate-controller pathology) · H1 (the rest). The gap-recovery drill keys to G4 and the rollback drill to A3 |
| [O4](./USE_CASES.md#o4--the-operator-can-see-and-steer) operator legibility | M3 | — |
| [O5](./USE_CASES.md#o5--clients-can-tell-failures-apart) failures distinguishable | — | Rides D3 as criteria, flagged in Group D's preamble. The system-status read (ADR-0034) is the transparency half |
| [O6](./USE_CASES.md#o6--deployable) deployable | F1 | The chart and its suites first exist at F1 and accrete with every deploying unit after it. The bare-cluster install proof stands from F1 onward |

## Dependencies

Three kinds, kept apart because conflating them is how phase numbering becomes rigid.

### Structural — the capability cannot exist without it

| Edge | What only the dependency supplies |
| --- | --- |
| F1 → F2 | A working, rotation-durable credential for the adapter to authenticate with |
| F2 → D1 | The schema and adapter backfill writes through |
| F3 → D1 | The budget machinery backfill spends from (backfill is the first workload big enough to trip real limits) |
| D1 → D2 | The sender statistics the gate evaluates, which cannot exist before pass 1 builds them |
| S2 → D2 | The tiers pass 2 scans with |
| S1 → D3 | The gate the read surface serves through |
| S1 → M1 | The authorization matrix every mutation is checked against |
| M1 → M2 | The authorized batched-mutation path apply executes through |
| M2 → M3 | Plans and their operation rows, for the approval screen to approve and group |
| D4 → M3 | The recorded runs, events, and failures the jobs view and the run screen read (D1 and D2 write theirs earlier, D4 completes the set) |
| F3 → M3 | The per-priority-class rate state the jobs view shows |
| S2 → X1 | The tier boundary and version flag tier 3 slots into |

### Conventional — real reasons, but the capability would function

| Edge | Reason | Standing |
| --- | --- | --- |
| D1 → D3 | The read surface is worth connecting when the index has history in it | "Nothing to read", not "cannot function" |
| D3 → S3 | Sanitization hardens a surface that must first exist to be hardened | Could be built earlier against fixtures |
| M3 → M4 | Candidate review needs the queue view to be useful | The job itself runs headless |
| V4 → X2/X3/X4 | Expansion multiplies surface, and one vertical should work first | A sequencing preference, not a hard dependency |

### Operational — bookkeeping, not design

- Provision the Postgres cluster and the secret-store machine account before F1/F2.
- Publish the OAuth consent screen to production before F1 (testing-mode tokens die in 7 days).
- Releases cut per merged unit. Deployment manifests land with the unit they deploy, in the
  Helm chart (ADR-0052), whose first pieces and test suites land with F1, the first unit that
  runs a pod.

## Orderings that would guarantee waste

The inverse of the value path. Each is an anti-constraint the posture must not be read as
licensing:

- Building the client surface before the gate exists, "adding redaction after". Retrofitting the
  invariant is the fail-open path.
- Running backfill before the rate limiter exists. That is debugging two new systems against a
  live provider at once, with the account-restriction failure mode in play on day one.
- Evaluating the scan gate before pass-1 statistics exist. A gate deciding blind is either
  scan-everything (the cost it exists to avoid) or skip-blind (the leak it exists to prevent).
- Building the UI before the reorg engine. A review screen with nothing to review.
- Training tier 3 on synthetic data because real labels haven't accumulated. Confident wrong
  answers on exactly the ambiguous cases.
- Adding a second datastore for coordination before contention exists. Infrastructure for a
  predicted bottleneck.
- Trusting the scan gate's compromise before reviewing its recorded skip rates. The rule is
  review before trust.

## Open decisions

Where a decision is recorded, the row cites its number, resolved through the
[decision-record index](./docs/adr/README.md).

| Decision | Gates | Standing |
| --- | --- | --- |
| Tier-3 model choice and training setup | X1 | Deliberately open. ADR-0006 defers it until real labeled data exists |
| Live-update transport for the UI | M3 | ADR-0058 proposes server-sent events from the UI's Go server with polling as the fallback. The operator asked for the behavior on 2026-09-10 and has not ruled on the transport. Only the UI's stream client depends on it ([docs/UI.md](./docs/UI.md#9-live-surfaces)) |
| Policy import and export to a file | nothing yet | ADR-0004 keeps a file form for import and export. The operator said on 2026-09-10 that such a mechanism may exist. No unit owns it and no record decides its shape |
| Maximum plan age | M2 | ADR-0032 requires rejecting plans older than a maximum age at apply time. The value is unchosen |
| Real-provider contract-suite runs | nothing yet | ADR-0043 defers whether the provider contract suite ever runs against the real provider, and against what mailbox, until the first real adapter is implemented (F2). Complexity and payoff at that point drive it |
| Whether one in-memory model serves both crash-harness targets | M2, D1 | ADR-0069 runs sequence reduction against an in-memory model of the machinery and leaves open whether one model serves both of ADR-0045's targets, the reorganization apply path at M2 and backfill resume at D1, or each gets its own. Only the harness's own internals depend on the answer |
| The gating case count for property-based tests | S1 | ADR-0055 requires a fixed count and fixes none. ADR-0069 reports what was measured at 100 and 200 and what each costs, without limiting the choice to those. At 100 the generator shape ADR-0069 chooses does not reliably reach the inputs that matter, so the count and that shape are decided together |
| Whether the generator coverage holds for the real generators | S1 | ADR-0069's choice of generator shape rests on a measurement against a model of the design's sensitivity axes, not against this project's generators. The record states two options, measuring one real generator written both ways before writing the rest, or writing them as decided and reading the generator report. If the real generators need the other shape, ADR-0069 is re-argued |
| Whether the backfill target's generators use the same shape as the reorganization plan generator | D1 | ADR-0069 measured the collection-generator shape for the reorganization plan generator only, and states two options for backfill resume, using the same shape and reading the generator report, or measuring a conditional generator against it first |
| Whether the operation sampler also runs in the gating run | M2 | ADR-0069 places it in the scheduled run and leaves the gating run open, stating what each option costs |
| Whether an edit to a generator re-triggers a control's mutation demonstration | S1 | ADR-0046 re-runs a demonstration when the control or its tests change. ADR-0069 records a generator edit made to satisfy the generator report leaving a demonstration green when it had to go red. Whether ADR-0046 names a generator edit as a trigger is undecided |
| Whether every control's mutation demonstration breaks the code both ways | M2 | ADR-0046 requires one removal of the mechanism. ADR-0069 records that in the crash harness a break making the code do less was caught only by checks against an absolute value, and a break making it do the wrong thing only by the comparison. Whether ADR-0046 requires both breaks for every control is undecided |
| Where the generated per-role data-access packages live | F2 | ADR-0066 leaves this open deliberately and states both options with their trade. Cheap to reverse. ADR-0071's per-role import rule waits on it, because under one option the rule is another allow list and under the other the compiler refuses the import |
| How many runtime database roles exist | F2 | ADR-0048 gives migrations their own role and ADR-0021 gives the user interface its own. The mapping from the six deployables to runtime roles is stated nowhere, and ADR-0066's per-role package split needs it before the generator is configured |
| Whether the audit log is ever trimmed, and by what | nothing yet | No runtime role may delete from it (ADR-0016), so nothing in the running system trims it. Never trimming is affordable at the stated corpus and is the strongest form of the surviving-evidence claim. If trimming is ever wanted it is a forward migration plus a step under a role that does not exist today |
