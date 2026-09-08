# Mutations

Each automatable control's mutation demonstration: the record that removing the control's
mechanism made its tests go red. The companion of [VERIFICATIONS.md](./VERIFICATIONS.md), split
by authoring moment — a verification row is minted at design time, when a control is decided; a
mutation row can exist only at implementation time, when the mechanism and its tests both exist
to be broken. Controls proven only by drill or by manual exercise never appear here — there is
no standing test to demand red; the split is ADR-0046's, resolved through the
[decision-record index](./adr/README.md).

**What a row is.** One control, one demonstration: how the mechanism was removed or disabled,
which tests went red, the date, and an evidence pointer. The table is the artifact — never a
pass rate. A surviving mutant on a control is a defect, not a statistic: either the tests are
vacuous or the mechanism is redundant, and the row stays open until one of those is resolved.

**When rows change.** A row is produced when its control lands and re-produced only when the
control or its tests change. Between those events the row stands as a claim about its date,
alongside the permanent CI tests that keep the control's green continuously earned.

No row exists yet: this ledger predates the first line of code, so there is no mechanism to
remove and no test to go red. That is the correct starting state, not an empty document.

## Demonstrations

| Control | Mechanism removed | Tests that went red | Date · evidence |
| --- | --- | --- | --- |
