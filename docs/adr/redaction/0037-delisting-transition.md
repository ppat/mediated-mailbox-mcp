# 0037. Removing a sender from the sensitive list marks its messages pending scan — delisting is a designed transition

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

Restricted-sender bodies are never scanned while the sender is restricted
([ADR-0008](./0008-restricted-senders-are-never-scanned.md)), which is correct, since their
denial follows from sender class alone. On removal from the list, all that sender's messages sit
with no scan verdict, and fail-closed means unscanned denies. Without a designed transition, their
bodies stay denied forever even though the sender is now normal, and no path finds those messages
or queues them for scanning. That is safe (over-denial, never a leak) but silent and permanent, so
it reads as a bug, and the agent's denial reason stops making sense.

## Decision

**Removing a sender from the sensitive list marks that sender's messages as pending scan.** The
normal machinery, scan-gate evaluation and then scanning for the gated-in, picks them up like any
other unscanned mail. Bodies remain denied until scanned, which fail-closed already guarantees.
One state transition, plus machinery that already exists.

**A removal reaches the transition by its effect, not by the edit that made it.** Every workload
that scans compares the sender domains of the messages the index stores as restricted, or holds
skipped as restricted, with the policy snapshot it loaded
([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)), before it evaluates the gate.
For every domain the snapshot now classifies normal, it sets that domain's messages stored as
restricted or skipped as restricted to a normal sender class with no rule naming it, and to pending
scan, and rebuilds the domain's sender statistics, in one transaction. Clearing the rule keeps a
message from naming a rule for a class that rule no longer sets
([ADR-0016](../data/0016-schema.md)). The skip state is read as well as the stored class,
because a rule added after a message was stored can leave its stored class normal while the scan
gate skips it as restricted, until the same comparison restricts the stored class
([ADR-0113](./0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)).
Backfill's second pass runs the comparison at the start of each of its runs, which it makes until
the pass has ended for the account, and when it marks any message the run starts over from the first
message waiting for a scan, because the marked messages may sit before the point it resumed from
([ADR-0017](../data/0017-two-pass-backfill.md)). A running second pass also makes it again at the
first unit boundary after the policy its run holds changes, where a change means the rules differ
by value, not that a reload built a new snapshot
([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). When that
comparison marks any message, the run starts over from the first message waiting for a scan inside
the same run, as a comparison at a run's start does. A rule removed by any path therefore reaches
the transition, whether through the UI, an import that drops it, a narrowed domain list, or a change
made directly in the database while nothing was running.

One interaction is bounded deliberately. The serve-time pattern check
([ADR-0002](./0002-fetch-time-re-evaluation.md)) would incidentally check an ex-restricted body at
release, but it is deliberately narrow, covering login-link and code patterns with deny-on-hit, and
does not count as scan clearance. This transition is the real path.

## Alternatives considered

- **No designed transition.** Its case: the resulting state is safe, since it over-denies and never
  leaks. Rejected: silent and permanent over-denial reads as a bug, and the agent's denial reason
  stops making sense.
- **The writer of the removal marks the messages**, the UI's policy edit marking them in the
  transaction that removes the rule. Its case is that the marking happens at the moment of the
  removal. Rejected, because a rule removed by any other path never reaches the transition.
- **A trigger on the policy table.** Its case is that it catches every path. Rejected, because
  [ADR-0060](../engineering/0060-no-code-in-the-database.md) keeps code out of the database.
- **A table of removed rules that each writer appends to.** Its case is an explicit record of what
  was removed. Rejected on the same gap as the writer marking the messages, with a table added.
- **Comparing each newly loaded policy snapshot with the one before it.** Its case is that no
  stored state is read. Rejected, because a process that starts after the removal holds no earlier
  snapshot, so a removal made while nothing ran is never seen.

## Consequences

- Fail-closed holds through the transition. Pending denies until scanned, as the `PENDING` state
  of [ADR-0093](./0093-composite-scan-gate.md), denied at fetch per
  [ADR-0002](./0002-fetch-time-re-evaluation.md), so the transition can never release a body
  early.
- Assumptions about other components: the index can enumerate a sender's messages. The scan gate
  and scanner treat the marked messages as ordinary unscanned mail, with no special path. Every
  workload that scans compares the messages stored as restricted or skipped as restricted with the
  policy it loaded, since that comparison is how a removal is observed. Between a removal and the
  next such comparison, the messages keep their skip state, which denies their bodies.
- Once backfill's second pass has ended, delta sync is the workload that scans, so each of its
  ticks makes the comparison before it decides what waits for a scan
  ([ADR-0104](./0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)).
- The comparison reads the index and the policy as they stand, so it is idempotent. A process
  stopped between the marking and the scan loses nothing, and the next comparison finds nothing
  more to mark.
- The transition is a control, and its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
