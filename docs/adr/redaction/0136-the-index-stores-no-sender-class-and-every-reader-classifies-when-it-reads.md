# 0136. The index stores no sender class, and every reader classifies under the policy in force when it reads

**Status:** Accepted (supersedes
[ADR-0113](./0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)) ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C4](../../../USE_CASES.md#c4--the-sensitive-sender-list-keeps-pace),
[O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

Ingest classified each message's sender under the policy of that moment and stored the class with
the rule that set it on the message, and the statistics rebuild stored a class on each sender,
restricted when any of the domain's messages was
([ADR-0016](../data/0016-schema.md)). Two comparisons kept the stored class current. The delisting
transition returned the messages of a domain the policy no longer restricts to a normal class and
to pending scan ([ADR-0037](./0037-delisting-transition.md)), and its counterpart for an added rule
set the messages of a domain stored as normal that the policy now restricts to the restricted class
with the rule (ADR-0113). Backfill's second pass made both at each run's start and when its policy
changed by value, and delta sync on every tick once that pass had ended. Each read every message of
the account, measured at about 30 ms each at 100,000 messages, on every tick.

Only the UI read the stored class. The Redaction Gate classifies at fetch time and the mediator's
role held no grant on the column ([ADR-0002](./0002-fetch-time-re-evaluation.md)). The scan gate
classifies under the policy it is given, and the mediator's index reads classify domains themselves
([ADR-0108](../operability/0108-index-reads-select-by-an-index-query-of-the-surfaces-own.md)). The
UI read it in four places. Home's corpus figures counted restricted messages, the failures dataset
showed each failed message's class and counted restricted failures, its row detail named the rule
that set the class, and the policy screens counted the senders restricted, showed each rule's
"index updated, {k} of {n}" and badged each sender. So the delisting transition's one safety job,
returning messages skipped as restricted to pending once their sender is normal, needs only the
scan state, and the comparison for an added rule existed only to keep the UI's displays current.

That comparison also opened a path to a state the Redaction Gate reads as invalid. A message scanned
with a content flag, whose domain a rule then restricted, was marked restricted and stayed scanned
with its flag. When the rule was lifted, the delisting transition set it to pending without clearing
the flag, and the gate denies a body whose message carries content flags while not scanned as an
invalid stored state rather than as pending its scan
([ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)). No body was
released wrongly. The path was read from the statements and not reproduced by a test.

## Decision

- **The index stores no sender class.** A message holds no class and no rule behind one, and the
  sender statistics hold no class ([ADR-0016](../data/0016-schema.md)). Ingest still classifies
  each sender, because the series of unclassified senders counts what the classifier cannot read.
- **Every reader of a sender's class classifies it under the policy in force when it reads.** The
  UI classifies under the account's composed policy, the base rules and the account's own, read in
  the same transaction as the rows the class is shown beside, with the classifier's lookups its
  composition root builds. Each of its reads that classifies runs in a repeatable-read, read-only
  transaction, so every statement of one response sees one state of the policy and the index. A sender the classifier cannot read is restricted, as the classifier
  decides it ([ADR-0004](../classification/0004-sender-list-decides.md)).
  - Home's restricted messages are the sum of the stored message counts of the sender domains the
    policy restricts. The sender statistics hold one row per stored domain with its exact count
    ([ADR-0109](../operability/0109-the-index-is-read-through-search-count-and-the-sender-listing.md)),
    so the sum is exact.
  - The policy screens and the sender picker classify each sender domain they read.
  - The failures dataset classifies each row's sender, and counts restricted failures by passing the
    domains the policy classifies normal into its statements, every other stored domain counting as
    restricted, as the mediator's sender class term does, so a domain the read did not classify fails
    closed
    ([ADR-0108](../operability/0108-index-reads-select-by-an-index-query-of-the-surfaces-own.md)).
    Its row detail names the rule that restricts the sender under the policy in force.
- **A rule's count on the policy screens is what it restricts now.** No count measures a stored
  class catching up with a rule, since nothing lags. The policy screens show no "index updated".
- **Nothing compares the policy with a stored class for an added rule.** The delisting transition is
  the one comparison left, and it reads only the messages skipped as restricted
  ([ADR-0037](./0037-delisting-transition.md)). A scanned message whose sender a rule restricts
  keeps its verdict, which a body denied for its sender never reads.

## Alternatives considered

- **Keep the stored class current through the comparison for an added rule (ADR-0113).** For it, a
  display reads one stored column with no classification, and "index updated" measures how far the
  stored classes have caught up with a rule. Against it, every tick reads every message of the
  account twice to keep current a value only the UI's displays read, a rule's effect on release is
  already immediate from each process's next policy reload, so the count measures nothing a reader
  acts on, and the comparison opened the path to an invalid stored state above.
- **Count restricted messages by classifying each message's domain over every message.** For it,
  the count reads the messages themselves. Against it, it counts every message of the account,
  measured at about 130 ms where the sum over the sender statistics took 1 to 2 ms, for the same
  number.

## Consequences

- A rule's effect on every figure, row and badge the UI shows follows the policy read with it, as
  its effect on release does. One response reads one snapshot, so its figures and rows never
  disagree about a sender's class. Separate requests read separate snapshots, so a screen whose
  figures and rows arrive in different requests can show them under two policies when a rule changes
  between the requests.
- A confirmed candidate's rule needs no path of its own to the index. It takes effect on release
  from each process's next policy reload and on the UI's displays from its next read.
- The audit row's `class_rule_id` stays. It is written at each body decision and records the rule
  behind that decision at that moment ([ADR-0016](../data/0016-schema.md)).
- Assumptions about other components. Every workload that adds, changes or removes messages
  rebuilds or removes the statistics of each domain it touched in the same transaction, so the
  statistics hold exactly the domains the index stores with their exact counts
  ([ADR-0109](../operability/0109-the-index-is-read-through-search-count-and-the-sender-listing.md)).
  A later display that splits or counts by sender class classifies when it reads, as these do.
- The UI's classification under the policy in force is a control. Its violation injection is
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
