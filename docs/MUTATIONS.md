# Mutations

The mutation ledger. A row here records one demonstration, that removing a control's mechanism
made its tests go red. This ledger and [VERIFICATIONS.md](./VERIFICATIONS.md) are companions
split by authoring moment. Deciding a control mints its verification row at design time. A
demonstration can happen only at implementation time, once there is a mechanism to remove and
tests to fail. A control with no standing automated test never appears here, since nothing
exists to demand red from, and proof only by drill or by manual exercise is that case. The
decision is ADR-0046's, resolved through the [decision-record index](./adr/README.md).

**A row.** One control, one demonstration. It names how the mechanism was removed or disabled,
the tests that went red, the date, and an evidence pointer. The table is the whole artifact,
never a pass rate. A surviving mutant holds its row open as a defect until the tests are fixed
or the mechanism is deliberately deleted as redundant.

**Lifecycle.** A row is produced when its control lands, as ADR-0046 defines landing, and
reproduced when the control or its tests change. Between those events it is a claim about its
date, while the permanent CI tests keep the control's green continuously earned.

## Demonstrations

| Control | Mechanism removed | Tests that went red | Date · evidence |
| --- | --- | --- | --- |
