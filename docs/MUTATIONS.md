# Mutations

Each automatable control's mutation demonstration lives here. A demonstration is the record
that removing the control's mechanism made its tests go red. This ledger is the companion of
[VERIFICATIONS.md](./VERIFICATIONS.md), and the two are split by authoring moment. A
verification row is minted at design time, when a control is decided. A mutation row can exist
only at implementation time, when the mechanism and its tests both exist to be broken. Controls
proven only by drill or by manual exercise never appear here because there is no standing test
to demand red. The split is ADR-0046's, resolved through the
[decision-record index](./adr/README.md).

**What a row is.** One control, one demonstration. The row names how the mechanism was removed
or disabled, which tests went red, the date, and an evidence pointer. The table is the artifact
and never a pass rate. A surviving mutant on a control is a defect, not a statistic. Either the
tests are vacuous or the mechanism is redundant, and the row stays open until one of those is
resolved.

**When rows change.** A row is produced when its control lands. It is re-produced only when the
control or its tests change. Between those events the row stands as a claim about its date,
alongside the permanent CI tests that keep the control's green continuously earned.

No row exists yet. This ledger predates the first line of code, so there is no mechanism to
remove and no test to go red. That is the correct starting state, not an empty document.

## Demonstrations

| Control | Mechanism removed | Tests that went red | Date · evidence |
| --- | --- | --- | --- |
