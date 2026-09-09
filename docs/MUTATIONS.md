# Mutations

The ledger of mutation demonstrations. One row records that removing a control's mechanism made
its tests go red. The verification catalogue and this ledger split on authoring moment.
Deciding a control mints its row in [VERIFICATIONS.md](./VERIFICATIONS.md) at design time. A
demonstration can only happen at implementation time, once a mechanism exists to remove and
tests exist to fail. Controls proven by drill or by manual exercise never appear here, having
no standing test to demand red. ADR-0046 holds the decision, resolved through the
[decision-record index](./adr/README.md).

**A row.** One control, one demonstration. It names the way the mechanism was removed or
disabled, the tests that went red, the date, and where the evidence lives. A pass rate is never
the artifact. A surviving mutant keeps its row open as a defect until the tests are fixed or
the mechanism is deliberately deleted as redundant.

**Row lifecycle.** Produced when the control lands. Produced again when the control or its
tests change. Otherwise the row is a claim about its date, while the permanent CI tests keep
earning the green continuously.

This ledger predates the first line of code, so no mechanism exists to remove and no row can
exist yet. An empty table is the correct present state.

## Demonstrations

| Control | Mechanism removed | Tests that went red | Date · evidence |
| --- | --- | --- | --- |
