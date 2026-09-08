# 0041. Policy arrives as an immutable snapshot, taken once per unit of work

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The policy list is operator-editable data that changes underneath a running system and must take
effect without a restart. Classification is a pure function that takes policy as a parameter
([ADR-0040](./0040-pure-core-decisions-as-values.md)). Those two commitments need a bridge: some
concrete rule for how changing policy data reaches unchanging pure functions — without a
decision ever being made against one policy and audited against another.

## Decision

- **The shell maintains one policy snapshot: an immutable value, swapped atomically when the
  policy source changes.** Nothing mutates a snapshot in place, ever.
- **Each request — and each item of batch work — takes the current snapshot once at entry and
  passes it down.** No decision path re-reads policy mid-flight, so a decision and its audit
  record always refer to the same policy, and a policy edit lands between units of work, never
  inside one.
- **No valid snapshot means no policy, and absence denies.** If a valid snapshot has never been
  loaded, the fail-closed constraint applies as it always does: deny the body.
- **An invalid update, arriving while a valid snapshot is held, keeps the last-good snapshot
  serving — and alarms loudly.** The last-good snapshot is an affirmatively loaded policy, not
  an absence of signal. The accepted cost, stated plainly: until the operator fixes the bad
  edit, a domain added by that edit is not yet protecting — which is exactly why the failure
  must be loud and immediate rather than a log line.

## Alternatives considered

- **Live-mutating shared policy state, read wherever a decision needs it.** No case was tabled
  for it; it is the shape a hot-reload implementation takes by default. Rejected: a policy edit
  landing mid-request tears the decision — classified under one policy, audited under another —
  and classification stops being reproducible, so the audit's rule references stop meaning
  anything.
- **Denying everything whenever a reload fails.** The maximally cautious reading of fail-closed.
  Rejected: fail-closed resolves *ambiguity* toward deny, but a held last-good snapshot is not
  ambiguity — it is a valid policy the operator affirmatively published. Denying the whole
  mailbox because an edit had a typo is over-redaction with no safety gained.

## Consequences

- What the snapshot guarantees for
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)'s fetch-time
  re-evaluation is the anti-tearing half: every fetch is decided against the snapshot current
  at that fetch, so a policy edit lands between calls, never inside one. A newly denied domain
  therefore takes effect on the next call once its update has loaded — the snapshot's staleness
  window is the reload delivery latency named in the assumptions below, and C2's next-call
  property holds to that bound.
- Assumptions about other components: something outside the mediator delivers policy-source
  changes to where the shell can see them (the policy store is managed as data); and something
  makes the reload-failure condition loud and immediate — the surfacing mechanism is not this
  record's to fix.
