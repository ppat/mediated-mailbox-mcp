# 0113. An added rule reaches the stored sender classes by its effect, through the comparison every scanning workload already makes

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C4](../../../USE_CASES.md#c4--the-sensitive-sender-list-keeps-pace), [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

A message's sender class and the rule that set it are stored when the message is indexed
([ADR-0016](../data/0016-schema.md)). The body is decided at fetch time against the policy in force,
so a newly added rule denies its senders' bodies from each process's next policy reload whatever the
index stores ([ADR-0002](./0002-fetch-time-re-evaluation.md),
[ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)). What the index stores is still
read. The UI counts, for each rule, the senders it matches whose stored class reads restricted, as
"index updated, {k} of {n}" ([docs/UI.md section 8.7](../../UI.md#87-policy)), and the scan gate
skips a sender the policy restricts without changing its stored class.

A removed rule reaches the stored classes through the delisting transition, by its effect: every
workload that scans compares the domains the index stores as restricted with the policy it loaded
and returns those the policy no longer restricts to a normal class and to pending scan
([ADR-0037](./0037-delisting-transition.md)). Nothing did the same for an added rule. The UI's
policy management is the first work that adds rules to a filled index, so it decides.

## Decision

- **An added rule reaches the stored classes by its effect, as a removed one does.** In the same
  comparison, every workload that scans also reads the sender domains of the messages the index
  stores as normal, and for every domain the policy snapshot it loaded now restricts under a rule, it
  sets that domain's messages stored as normal to the restricted class with the rule that now sets
  it, and rebuilds the domain's sender statistics, in one transaction. Backfill's second pass makes
  it at the start of each of its runs, and again at the first unit boundary after the policy its run
  holds changes by value, and delta sync on each tick once the pass has ended, as each makes the
  delisting comparison ([ADR-0037](./0037-delisting-transition.md),
  [ADR-0104](./0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)).
- **A domain the policy restricts with no rule is left as stored.** A policy that never loaded, and
  an address the classifier cannot read, classify restricted with no rule naming why. Neither is a
  rule taking effect, and writing either would restrict every stored sender while no policy has
  loaded.
- **The scan state is left as it is.** A message waiting for its scan is skipped as restricted when
  the gate reaches it, and a scanned one keeps its verdict, which a body denied for its sender never
  reads. Restricting needs no rescan, so the pass does not start over.
- **A stored rule is left as written while its domain stays restricted.** The comparison writes only
  domains stored as normal, so a domain restricted by one rule and then by another keeps the first
  rule's identifier, as ADR-0016 states.

## Alternatives considered

- **The UI's write restricts the stored messages in its own transaction.** For it, the index
  catches up at the moment of the write. Against it, a rule added by any other path, an import, a
  confirmed candidate, or a write made directly in the database, never reaches the index, the reason
  ADR-0037 refused the writer marking its own removal. The UI's grant would also widen onto
  `messages`.
- **The stored class stays the class at ingest, and every reader classifies at read time.** For it,
  no write at all. Against it, "index updated" would have nothing to measure, since it counts stored
  classes catching up with a rule.
- **Each comparison sets every stored domain's class and rule to what classifying it now would
  store.** For it, a rule identifier naming a lifted rule while another rule still restricts the
  domain is refreshed too. Against it, it rewrites restricted domains whose class has not changed on
  every comparison, and the stored rule then follows which of two matching rules the classifier meets
  first, which ADR-0016 leaves as written.

## Consequences

- From the first comparison after a rule is added, "index updated" reaches the rule's count of
  senders, and the sender statistics' class reads restricted.
- The comparison reads the index and the policy as they stand, so it is idempotent. A process
  stopped between the marking and anything after it loses nothing, and the next comparison finds
  nothing more to mark. A domain restricted here and lifted later reaches the delisting transition as
  any restricted domain does, and returns to pending scan.
- Between a rule's addition and the next comparison the index stores the domain as normal, while
  fetch-time re-evaluation already denies its bodies. The window is a stale count on a screen, never
  a release.
- Assumptions about other components. The comparison runs where the delisting comparison runs, so a
  workload that stops making one stops making both. A confirmed candidate's rule reaches the index
  the same way as any other rule.
- The transition is a control. Its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
