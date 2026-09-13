# 0045. Crash-injection stateful testing is aimed where silent failure meets hard-to-reverse damage

**Status:** Accepted ·
**Serves:** [A3](../../../USE_CASES.md#a3--bulk-change-is-reversible),
[O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The delivery posture defers freely except where deferral is irreversible or the failure it
permits is silent. A crash-recovery bug in checkpointed apply or rollback is both. It corrupts
label state on the real mailbox, and it fails silently until a process dies mid-sequence.
Sequence-dependent failures are reachable by no other kind of test, because no other kind
places a crash inside a generated sequence, and a physical kill-the-pod drill proves the real
substrate exactly once, on one sequence. "Learn from production" cannot cover this class. The
pattern this record builds on is the one published in Amazon's S3 ShardStore work (SOSP 2021).
That pattern is a crash operation in the generator's alphabet, persistence and forward-progress
invariants checked after recovery, and a deliberately coarse crash model.

## Decision

- **Build a crash-injection stateful test harness on that pattern.** Extend the operation
  alphabet with a synthetic crash operation the generator can place anywhere, drive generated
  operation sequences against the machinery, and after each simulated crash run the real
  recovery path and check the two invariant families. **Persistence** means anything reported
  durable before the crash is intact after it. **Forward progress** means recovery completes
  and nothing is left permanently stuck.
- **Use a coarse, component-level crash model deliberately.** The published evidence is that
  the ShardStore work found fine-grained exhaustive crash enumeration catching substantially
  the same bugs at far higher cost.
- **Aim it by asymmetric payoff, in order.** The reorganization apply/rollback path comes
  first, because checkpointed apply and the op log's exact-restore claim are
  sequence-dependent machinery on the operator's real mailbox. Backfill checkpoint/resume
  comes second. Lease accounting and sync-cursor recovery come only if the first two prove the
  harness pays.
- **Bounded, fixed-seed runs ride the ordinary code-test suite if they prove fast enough, and
  deep sequence exploration runs scheduled, never gating.** If the bounded runs are too slow
  for the gate, they run on the scheduled, non-gating workflow instead.
- **The physical drills stay.** Killing the real pod mid-run proves the real substrate once,
  and the in-process harness explores the sequence space cheaply and continuously. Disjoint
  kinds, not substitutes.

## Alternatives considered

- **Physical drills only.** The case for them is that they exercise the real substrate, and
  the catalogue already demands them. Rejected as the sole kind. A one-shot drill executes one
  sequence, and the failures worth fearing live in sequences nobody enumerates by hand.
- **Exhaustive fine-grained crash enumeration.** The case for it is completeness. Rejected on
  the ShardStore work's own published evidence. Coarse component-level granularity caught
  essentially the same bugs at far lower cost, and the harness is built from scratch either
  way.
- **A blanket harness over every stateful component.** Rejected because the harness is its own
  real project, and spending it where failures are loud or easily reversed inverts the payoff
  argument that justifies it.

## Consequences

- The harness is built from scratch, and each target beyond the first two must justify itself. No
  crash-injection harness exists to import. What does exist is a library that generates sequences of
  named operations and reduces a failing one, and
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) takes it. The crash operation's
  behaviour, the call into the real recovery path, the two invariant families, and the sampler that
  draws a fresh operation mix for each sequence in the scheduled run are what this project writes.
- The catalogue's existing recovery rows (kill the backfill pod mid-run, kill the apply job
  mid-plan) keep the drill kind's date-claim semantics for the real-substrate half. The
  harness's continuous proof of the same controls is dispositioned in the catalogue's §5
  rather than as new rows.
- Assumptions about other components: checkpoint and op-log machinery expose their state as
  inspectable values ([ADR-0040](./0040-pure-core-decisions-as-values.md)'s shape), so the
  harness can assert persistence and forward progress without reaching into internals.
