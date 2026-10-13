# 0132. A body refused for its size or by its conversion is recorded as refused, a scan state that denies it, and returns to pending only when the size bound changes

**Status:** Accepted (supersedes [ADR-0017](../data/0017-two-pass-backfill.md), with [ADR-0133](../data/0133-backfill-is-full-history-and-two-pass.md) and [ADR-0131](../provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)) ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [O1](../../../USE_CASES.md#o1--rate-limited-politely), [O2](../../../USE_CASES.md#o2--observable)

## Context

Backfill's second pass and delta sync's ticks scan the messages waiting for a scan with the same
gate, scanner and conversion ([ADR-0133](../data/0133-backfill-is-full-history-and-two-pass.md),
[ADR-0104](./0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)). A body the
conversion refused, for its size or because the parser refuses it, stayed pending under
[ADR-0017](../data/0017-two-pass-backfill.md)'s rule, so its message
waited for a scan for good. A tick reads what waits from where the last tick stopped and starts
again from the first waiting message when it reaches the end, so once the backlog was small, every
tick fetched every refused body again. A fetch costs 20 quota units, and at up to 200 decisions a
tick every five minutes the refetches alone could take about a quarter of the account's target.

A body the port refuses is a third case. [ADR-0131](../provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)
bounds the part the port returns at one size constant and refuses a larger part, or one the adapter
cannot decode, with its own error. That refusal repeats on every fetch, since the message does not
change at the provider.

A message's relation to the scanner is a state of its own because "not scanned" has distinct
meanings with different consequences ([ADR-0093](./0093-composite-scan-gate.md)). Pending denies
the body as backlog, and the denial's note tells the agent so
([ADR-0101](../operability/0101-every-failure-names-its-origin-and-a-body-denial-is-a-result.md)).
A refused body is not backlog. Nothing that runs later will release it while the system is as it
is. Left pending, its denial misreports why, and the backlog depth that
[ADR-0093](./0093-composite-scan-gate.md) watches as a failure counts it for good.

A stored decision that a later change can make wrong is reached by comparing what the row records
with what the run runs with, at backfill's run-start step, the step a scanner change reaches
([ADR-0120](./0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).

## Decision

- **A message whose body is refused leaves the waiting state as `refused`, a scan state of its
  own.** A body is refused when the port refuses it (ADR-0131), when the conversion refuses its HTML
  part, and when the part ADR-0131 bounds is text above the bound. Backfill's second pass and delta
  sync's ticks record it in the commit that records the page's other outcomes, with the gate's
  decision to scan it, and record a failed item of class `refused` beside it, so the run shows what
  it refused and why. A run never fetches a refused body again.
- **The Redaction Gate denies a refused message's body**, as `body_refused`, after a restricted
  sender and before content flags, so a restricted sender's message reads as the policy's denial.
  Content flags on a refused message are an invalid stored state, as on any state but scanned. The
  mediator's own refusal at serve time, when the port refuses a body the gate let through, carries
  the same reason (ADR-0131), so the agent reads one meaning however the refusal was found.
- **The refusal records the size bound it was made under.** The row stores the bound in bytes. At
  backfill's run-start step, each refused message whose recorded bound differs from the bound the
  process runs with returns to pending, and the step records the second pass as not ended and
  marks it to start over, as it does for a verdict made under another scanner (ADR-0120). A raised
  bound therefore reaches every body refused under the old one at the worker's next start, and a
  lowered one reaches them too, since a body refused under it is refused again.
- **A change of scanner does not return a refused message to pending**, since neither a body's
  size nor whether it converts depends on the scanner. Nor does a change of its sender's class. The
  index stores no class, so a rule added or lifted leaves the message's state as it is
  ([ADR-0136](./0136-the-index-stores-no-sender-class-and-every-reader-classifies-when-it-reads.md)),
  and the delisting transition returns only the messages skipped as restricted, which a refused
  message never is ([ADR-0037](./0037-delisting-transition.md)). The gate classifies its sender
  under the policy in force when it decides, so while a rule restricts the sender the body is denied
  as the policy's denial, and otherwise as `body_refused`.
- **A refusal records nothing it was made under beyond the bound.** A conversion refused for
  something other than its size, the parser's limit on nested elements, could become convertible
  after a change of the conversion library or its configuration. The system has no identity for
  the conversion to record, and a scan verdict does not record one either, although the Markdown a
  verdict was made from changes with the conversion just as much. A refusal left standing after
  such a change is over-redaction, the direction the design accepts, and it is visible, as a failed
  item of class `refused` on the run that made it and as `body_refused` denials. A conversion
  change that should reach stored refusals ships with a change of how refusals are compared, made
  then by the change that needs it.
- **A failure stays a failure.** A fetch the provider fails, throttles or no longer answers is
  handled as before, left pending and asked for again
  ([ADR-0104](./0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)), since a
  transient failure is the provider's and the next attempt can succeed.

## Alternatives considered

- **A body the converter refuses stays pending**, which [ADR-0017](../data/0017-two-pass-backfill.md)
  decided before this record took the rule from it. The case made for it was that no state is
  added, the body stays denied, and the run records the refusal as a failed item. Not chosen,
  because the message waits for a scan that never comes, delta sync fetches it again on every pass
  over the backlog, and its denial reads as backlog when it is not.
- **Recording a refused body as scanned with a flag.** The case for it, which ADR-0017 weighed, is
  that the body stays denied and leaves the backlog with no new state. Not chosen, for ADR-0017's
  reason, because it claims a scan that never ran, and a flag would read as a code or a link the
  body does not hold.
- **Scanning a body the conversion refuses as its raw HTML**, which ADR-0017 also weighed. Its case
  is that the message leaves the backlog with a real verdict. Not chosen, for ADR-0017's reason,
  because the scanner reads Markdown, and a refused body is withheld at release whatever it holds.
- **Keeping the message pending and leaving it out of the waiting reads**, by a marker beside the
  state. The case for it is that the closed vocabulary of scan states does not grow. Not chosen,
  because the gate would still deny it as pending, a meaning it does not have, which
  [ADR-0093](./0093-composite-scan-gate.md) refuses by giving each meaning of "not scanned" its own
  state.
- **Returning every refused message to pending at each worker start**, recording nothing. The case
  for it is that no column is added and any change, the bound, the conversion or the adapter,
  reaches the refusals. Not chosen, because every start would then reopen backfill's second pass for
  an account holding any refusal, and delta sync stops scanning while that pass is due again, for a
  refetch that almost always refuses again.
- **Recording the release the refusal was made under.** The case for it is that every change that
  ships reaches the refusals. Not chosen for the same reason as the last, since a release is cut on
  every merge, documentation included.
- **Resetting a refusal on a change of scanner.** The case for it is that a scanner change already
  reopens the second pass. Not chosen, because the scanner decides neither whether a body is within
  the bound nor whether it converts, so the refetch would only refuse again.

## Consequences

- A refused body is fetched once, by the run that refused it, and once more after each change of the
  bound. Delta sync's ticks spend nothing on it otherwise.
- The scan backlog depth counts only what waits, so a mailbox holding refused bodies can reach an
  empty backlog.
- The closed vocabularies grow by one each. The schema's scan state and the audit row's scan state
  and reason, the client surface's scan state and denial reason, and the UI's wording of each, all
  gain the new value in the same change ([ADR-0016](../data/0016-schema.md),
  [ADR-0101](../operability/0101-every-failure-names-its-origin-and-a-body-denial-is-a-result.md)).
- Assumptions about other components. The port reports a body it refuses with ADR-0131's error and
  never as the provider's failure, so a refusal is told apart from a failure by the error alone. The
  conversion refuses a body by an error the caller can recognize as a refusal, and the bound it
  refuses above is ADR-0131's constant. Backfill's run-start step runs at every worker start, before
  the second pass, as ADR-0120 states.
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
