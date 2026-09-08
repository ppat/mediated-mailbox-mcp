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
- **Statuses**: pending rows name the roadmap unit that delivers the control — or the later
  unit at which the violation first becomes constructible, whichever comes later; proven rows
  carry the date and a pointer to the evidence; parked rows carry the standing reason and a
  stated re-open condition, and no date. Proof lifetime depends on the row's kind (ADR-0044):
  an automatable injection becomes a permanent CI test and its proof is continuous; a drill or
  a manual exercise is a claim about its date, and re-runs after relevant change belong to the
  affected unit, not this table.
- A control whose tests are automatable also owes a mutation demonstration in
  `docs/MUTATIONS.md` (ADR-0046) — the catalogue does not carry it.
- Rows are **rekeyed, not rewritten**, when the work breakdown changes; a recut ticket inherits
  its rows and a passed row keeps its evidence pointer.
- The how-to detail of running an injection lives with the implementation, never here.
