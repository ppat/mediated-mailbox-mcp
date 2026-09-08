# 0032. All validation precedes the first write: batches fail whole, and saved plans are validated at creation, re-validated at apply, and expire

**Status:** Accepted ·
**Serves:** [A2](../../../USE_CASES.md#a2--no-destructive-action-on-sensitive-mail), [A3](../../../USE_CASES.md#a3--bulk-change-is-reversible), [G3](../../../USE_CASES.md#g3--reorganization)

## Context

A defect discovered mid-application leaves a half-applied batch, and half-applied batches are
miserable to debug. A reorg plan sharpens the problem: it can be created long before it is
applied, and the mailbox keeps changing in the gap — so a plan valid at creation is not
necessarily valid at apply.

## Decision

- **All validation for a batch completes before the first write of any part of it.** References
  resolve, operations are well-formed, authorization passes — all of it, before anything writes.
  A defect in operation 18 fails the batch before operation 1 writes anything. This generalizes
  the all-or-nothing rule of [ADR-0019](./0019-asymmetric-mutation.md) from "fails whole on
  authorization" to "fails whole on any validation."
- **A saved reorg plan is validated twice: at creation, and again at apply time.** The apply-time
  re-validation covers the entire plan, runs immediately before the first write, and evaluates
  against the mailbox as it exists then — not as it existed when the plan was drawn up. Creation
  and apply both gate: a plan that fails validation is rejected at whichever point catches it.
- **A plan older — measured from its creation — than the maximum plan age is rejected outright at
  apply time**, even if it would still pass re-validation. The value of the maximum age is not
  fixed by this record; it is an open decision tracked in
  [ROADMAP.md](../../../ROADMAP.md#open-decisions).
- A saved plan is stored state — a plan is never an action
  ([ADR-0020](./0020-reorg-plan-approve-apply-rollback.md)), but saving one is a state change, so
  creating a plan is not a dry-run under
  [ADR-0031](./0031-dry-run-on-mutating-operations.md)'s zero-state-change definition.

## Alternatives considered

- **Validate at creation only.** Rejected: the mailbox changes between creation and apply — delta
  sync moves state, policy edits reclassify senders — so validity at creation does not guarantee
  validity at apply.
- **Apply-time validation only, with creation-time checking as a non-gating preview.** The
  recommendation as originally tabled; the operator ruled instead that validation gates at
  creation as well as at apply.
- **No expiry — let apply-time re-validation alone decide old plans.** The case for it as
  tabled: every plan is already reviewed at approval, and the cutoff is one more knob to pick and
  maintain. Rejected: re-validation catches mechanical staleness only; a plan can remain
  mechanically valid yet organizationally stale, drawn against a corpus that has since changed in
  ways validation cannot see.

## Consequences

- Whole-batch validation is fully compatible with checkpointed apply
  ([ADR-0020](./0020-reorg-plan-approve-apply-rollback.md)): checkpointing governs recovery from
  mid-flight *runtime* failure, while this decision governs never starting on a batch that was
  invalid at submission.
- Assumptions about other components: validation verdicts are computable without side effects
  (the Mutation Authorizer's matrix is a pure check, an assumption
  [ADR-0031](./0031-dry-run-on-mutating-operations.md) already names); the Organize Engine's
  (`mail-organize`) apply job runs re-validation and the age check before its first write — and
  "first write" includes the ensure-labels-exist step, which creates labels at the provider;
  and the age check assumes a plan's creation time is recorded with the plan.
- The rules here are controls; their violation injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
