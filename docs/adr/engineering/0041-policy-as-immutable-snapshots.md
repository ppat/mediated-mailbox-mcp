# 0041. Policy arrives as an immutable snapshot, taken once per unit of work

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[A2](../../../USE_CASES.md#a2--no-destructive-action-on-sensitive-mail)

## Context

The policy list is operator-editable data that changes underneath a running system and must take
effect without a restart. Classification is a pure function that takes policy as a parameter
([ADR-0040](./0040-pure-core-decisions-as-values.md)). Those two commitments need a bridge: some
concrete rule for how changing policy data reaches unchanging pure functions — without a
decision ever being made against one policy and audited against another.

## Decision

- **The shell maintains one active policy snapshot: an immutable value, swapped atomically,
  read from the policy tables of [ADR-0016](../data/0016-schema.md).**
  Nothing mutates a snapshot in place, ever — and validation is part of taking effect, so a
  published edit becomes the active policy at the moment it validly loads, and not before. From
  the first valid load onward there is, at every moment, exactly one active valid policy, and
  it is the one that counts; before any valid load there is no policy at all (the
  absence-denies rule below).
- **Each request — and each unit of batch work — takes the active snapshot once at entry and
  passes it down.** No decision path re-reads policy mid-flight: a request keeps its snapshot
  through completion even if the active policy changes underneath it, so a decision and its
  audit record always refer to the same policy, and a policy edit lands between units of work,
  never inside one.
- **A reorganization plan is one unit, exactly — twice over.** The whole plan takes the active
  policy once at creation and again, whatever is active then, at apply — consistent with
  whole-batch validation re-running at apply
  ([ADR-0032](../mutation/0032-whole-batch-validation.md)). This cut is fixed; it is not open
  to interpretation.
- **The long-running jobs — backfill, delta sync — cut deliberately small units:** an item, a
  page, or whatever the job's practical characteristics make its natural equivalent, so that a
  policy fix takes effect promptly for the work not yet processed. Here, and only here, the
  exact cut is each job's to make; the spirit — small units, prompt uptake — is the rule.
- **No valid snapshot means no policy, and absence denies.** If a valid snapshot has never been
  loaded, the fail-closed constraint applies as it always does: deny the body.
- **An invalid update never takes effect.** It displaces nothing: the active valid policy
  remains active — not a fallback, not a retained historical state, but the same current,
  correct policy that has governed all along — and the failure alarms loudly. Beyond that alarm
  and its trace in the audit or the logs, the system keeps no memory of the failed update. The
  accepted cost, stated plainly: until the operator fixes the bad edit, whatever protection
  that edit was meant to add is not yet active — which is exactly why the failure must be loud
  and immediate rather than a log line.

## Alternatives considered

- **Live-mutating shared policy state, read wherever a decision needs it.** No case was tabled
  for it. It is the shape a hot-reload implementation takes by default. Rejected because a
  policy edit landing mid-request tears the decision — classified under one policy, audited
  under another — and classification stops being reproducible, so the audit's rule references
  stop meaning anything.
- **Denying everything whenever a reload fails.** The maximally cautious reading of fail-closed.
  Rejected: fail-closed resolves *ambiguity* toward deny, but the active policy is not
  ambiguity — it is a valid policy the operator affirmatively published, and an update that
  fails validation never displaced it. Denying the whole mailbox because an edit had a typo is
  over-redaction with no safety gained.

## Consequences

- What the snapshot guarantees for
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)'s fetch-time
  re-evaluation is the anti-tearing half: every fetch is decided against the snapshot active at
  that fetch, so a policy edit lands between calls, never inside one. A newly denied domain
  takes effect on the next call once its update has validly loaded — before that moment the
  edit is not yet policy. No latency bound is promised. An edit is a row in the policy tables,
  and the gap between writing it and it becoming active is the next snapshot load, whose cadence
  is each process's own.
- A reorganization plan created under one policy and applied under a later one is decided at
  apply time by the policy active then — the plan carries its operations, never a frozen
  policy.
- Assumptions about other components: the policy tables of
  [ADR-0016](../data/0016-schema.md) hold the current policy and every process can read them,
  and something makes the reload-failure condition loud and immediate, since the surfacing
  mechanism is not this record's to fix.
