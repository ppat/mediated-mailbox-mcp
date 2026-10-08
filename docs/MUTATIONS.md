# Mutations

The mutation ledger. A row here records one demonstration, that breaking a control's mechanism
made its tests go red. This ledger and [VERIFICATIONS.md](./VERIFICATIONS.md) are companions
split by authoring moment. Deciding a control mints its verification row at design time. A
demonstration can happen only at implementation time, once there is a mechanism to remove and
tests to fail. A control with no standing automated test never appears here, since nothing
exists to demand red from, and proof only by drill or by manual exercise is that case. The
decision is ADR-0046's, resolved through the [decision-record index](./adr/README.md).

The ledger is this file and the files under [mutations/](./mutations/) it lists in
[Where the rows are](#where-the-rows-are). This file holds what a row is and where it goes, and
those files hold the rows.

**A row.** One control, one demonstration. It names how the mechanism was broken, the tests that
went red, the date, and an evidence pointer. The rows are the whole artifact, never a pass rate.
Every mechanism is broken at least both ways, once so it does less and once so it does the wrong
thing, one patch per break, and the row numbers each break and names the tests each turned red. A
surviving mutant holds its row open as a defect until the tests are fixed or the mechanism is
deliberately deleted as redundant, and the row says so beside its date. The runner that applies a
demonstration's patches, `go tool mutproof` ([testsupport/README.md](../testsupport/README.md)),
prints each row in this shape.

**A row's form.** A row is a section whose heading is the control, so each control has an anchor
of its own. Each field sits on a line of its own, so reproducing a row edits only the lines whose
content changed.

| Line | Holds |
| --- | --- |
| `## <control>` | The control |
| `- **Date · evidence:** <date> · <evidence>` | The date of the demonstration and its evidence pointer, with any later reproduction of some or all breaks and its own pointer after it, and `· open, a surviving mutant` while a mutant survives |
| `- **Break (<n>):** <how the mechanism was broken>` | One line per break, numbered from 1 in the order the patches were run. A control broken by one patch alone has `- **Break:**` |
| `- **Went red in <package>:** <tests>` | Nested under its break, one line per package whose tests went red, with the tests by the names the test runner reports. A surviving mutant has the nested line `- **Went red:** none, a surviving mutant` instead |

**Lifecycle.** A row is produced when its control lands and reproduced when the control, its
tests, or a generator its tests draw from changes. Between those events it is a claim about its
date, while the permanent CI tests keep the control's green continuously earned. Each
demonstration is recorded with the implementation work that touches the surface area its control
interacts with. Whether each demonstration's patch still applies is checked on every pull request
by the `mutation-patches` workflow ([CLAUDE.md](../CLAUDE.md#ci-workflows)).

## Where the rows are

A control's row sits in the file of the component whose directory holds its patches. A patch sits
in the package whose control it demonstrates ([CLAUDE.md](../CLAUDE.md#tests)), and its component
is the top-level directory of its path, with `packaging/chart` and `tests/chainsaw` taken whole as
in [CLAUDE.md](../CLAUDE.md#components). A control whose patches sit in more than one component's
directory has its row in [cross-component.md](./mutations/cross-component.md), which keeps every
such control whole in one place. A row whose control gains or loses a patch so that its home
changes moves to its new file in the same change. Inside a file, rows are ordered by control name,
alphabetically, ignoring letter case and code formatting, so rows added by different changes land
at different places in the file. A component's first row creates its file and its line below.

| File | Holds the rows of the controls whose patches sit in |
| --- | --- |
| [mutations/content.md](./mutations/content.md) | `content/` |
| [mutations/core.md](./mutations/core.md) | `core/` |
| [mutations/db.md](./mutations/db.md) | `db/` |
| [mutations/executioncontext.md](./mutations/executioncontext.md) | `executioncontext/` |
| [mutations/process.md](./mutations/process.md) | `process/` |
| [mutations/provider.md](./mutations/provider.md) | `provider/` |
| [mutations/ratelimit.md](./mutations/ratelimit.md) | `ratelimit/` |
| [mutations/testsupport.md](./mutations/testsupport.md) | `testsupport/` |
| [mutations/mediate.md](./mutations/mediate.md) | `mediate/` |
| [mutations/backfill.md](./mutations/backfill.md) | `backfill/` |
| [mutations/sync.md](./mutations/sync.md) | `sync/` |
| [mutations/ui.md](./mutations/ui.md) | `ui/` |
| [mutations/cross-component.md](./mutations/cross-component.md) | More than one component's directory |
