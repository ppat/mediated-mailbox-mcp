---
paths:
  - "docs/VERIFICATIONS.md"
---

# Rules for docs/VERIFICATIONS.md

You are touching the verification catalogue — the test-plan layer, more durable than any ticket,
more fluid than the design.

- **A new control lands with its injection row** — in the same change that introduces the
  control, never later. An uninstrumented control is an unproven claim.
- **Rows are violation injections**: written as `<deliberate violation> → <expected refusal>`,
  never "test that X works." The *Proves* column names the claim, cites the decision record by
  number, and where it earns its keep, names the wrong reading the injection rules out.
- **Statuses**: pending rows name the roadmap unit that delivers the control; proven rows carry
  the date and a pointer to the evidence; parked rows carry the standing reason. A row's kind
  decides whether its proof stays current or holds only for its date (ADR-0046). An
  automatable injection becomes a permanent CI test with
  continuous proof. A drill or a manual exercise is a claim about its date, and re-runs after
  relevant change belong to the affected unit, not this table.
- An automatable control also owes its mutation demonstration to `docs/MUTATIONS.md`
  (ADR-0046). This document never carries it.
- Rows are **rekeyed, not rewritten**, when the work breakdown changes; a recut ticket inherits
  its rows and a passed row keeps its evidence pointer.
- The how-to detail of running an injection lives with the implementation, never here.
