# Mutations

The mutation ledger. A row here records the latest demonstration of each of a control's breaks, that
breaking its mechanism made its tests go red. This ledger and [VERIFICATIONS.md](./VERIFICATIONS.md)
are companions split by authoring moment. Deciding a control mints its verification row at design
time. A demonstration can happen only at implementation time, once there is a mechanism to remove
and tests to fail. A control with no standing automated test never appears here, since nothing
exists to demand red from, and proof only by drill or by manual exercise is that case. The decision
is [ADR-0046](./adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)'s.

The ledger is this file and the files under [mutations/](./mutations/) it lists in
[Where the rows are](#where-the-rows-are). This file holds what a row is and where it goes, and
those files hold the rows.

**A row.** One control, the latest demonstration of each of its current breaks. It names how the
mechanism was broken, the tests that went red, and the date and evidence pointer of the run that
last demonstrated each break. The rows are the whole artifact, never a pass rate. Every mechanism is
broken at least both ways, once so it does less and once so it does the wrong thing, one patch per
break, and the row numbers each break and names the tests each turned red. A surviving mutant holds
its row open as a defect until the tests are fixed or the mechanism is deliberately deleted as
redundant, and the row says so beside the date of the run it survived. The runner that applies a
demonstration's patches, `go tool mutproof` ([testsupport/README.md](../testsupport/README.md)),
prints one run's row in this shape, and the author places that run's breaks into the row, joining
the date lines of breaks last demonstrated in different runs as the table below states.

**A row's form.** A row is a section whose heading is the control, so each control has an anchor
of its own. Each field sits on a line of its own, so reproducing a row edits only the lines whose
content changed. The table below is the form a row takes when it is written or rewritten, as
Lifecycle below states.

| Line | Holds |
| --- | --- |
| `## <control>` | The control |
| `- **Date · evidence:** <date> · <evidence>` | For the row's current patches, the date of their latest demonstration and its evidence pointer, followed by the seed when the run fixed one, and `· open, a surviving mutant` after the date and evidence of the run a mutant survived, while it survives. When the breaks were last demonstrated in different runs, it holds one `<date> · <evidence> for <breaks>` per run, in the order of the breaks, joined by `, and`. It says nothing about earlier runs or why a row was demonstrated again |
| `- **Break (<n>):** <how the mechanism was broken>` | One line per break, numbered from 1. A break keeps its number when a later run demonstrates it again, and a new break takes the next number. `go tool mutproof` numbers one run's breaks from 1 in the order its patches were run, and the author places them into the row under the row's numbers. A control broken by one patch alone has `- **Break:**` |
| `- **Went red in <package>:** <tests>` | Nested under its break, one line per package whose tests went red, with the tests by the names the test runner reports. A surviving mutant has the nested line `- **Went red:** none, a surviving mutant` instead |

**Lifecycle.** A row is produced when its control lands and reproduced when the control, its tests,
or a generator its tests draw from changes, which is when its mutations change or need to change,
and only then is it rewritten into the form this file states. A row in that form holds no history.
Every statement in it is true of the code as it stands, so reproducing a row rewrites its date line
for the breaks run again, and a break whose patch is retired leaves the row with it. A row not yet
rewritten keeps the date line it has until its mutations next change or need to change. Between
those events the date line is a claim about the dates it names, while the permanent CI tests keep
the control's green continuously earned. Each demonstration is recorded with the implementation work
that touches the surface area its control interacts with. Apart from those events, the
`mutation-demonstrations` workflow runs every demonstration weekly and when started by hand, and
fails when one no longer holds, which writes no row
([ADR-0124](./adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)). Whether
each demonstration's patch still applies, and applies at one place only, is checked on every pull
request by the `mutation-patches` workflow ([CLAUDE.md](../CLAUDE.md#ci-workflows)).

## Where the rows are

A control's row sits in the file of the component whose directory holds its patches. A patch sits in
the package whose control it demonstrates ([CLAUDE.md](../CLAUDE.md#tests)), and its component is
the top-level directory of its path, with `packaging/chart` taken whole as in
[CLAUDE.md](../CLAUDE.md#components), and a patch under `packaging/tests/chainsaw` belonging to the
same component, whose file is [mutations/packaging-chart.md](./mutations/packaging-chart.md). A
control whose patches sit in more than one component's directory has its row in
[cross-component.md](./mutations/cross-component.md), which keeps every such control whole in one
place. A row whose control gains or loses a patch so that its home changes moves to its new file in
the same change. Inside a file, rows are ordered by control name, alphabetically, ignoring letter
case and code formatting, so rows added by different changes land at different places in the file. A
component's first row creates its file and its line below.

| File | Holds the rows of the controls whose patches sit in |
| --- | --- |
| [mutations/content.md](./mutations/content.md) | `content/` |
| [mutations/core.md](./mutations/core.md) | `core/` |
| [mutations/credential.md](./mutations/credential.md) | `credential/` |
| [mutations/db.md](./mutations/db.md) | `db/` |
| [mutations/executioncontext.md](./mutations/executioncontext.md) | `executioncontext/` |
| [mutations/process.md](./mutations/process.md) | `process/` |
| [mutations/provider.md](./mutations/provider.md) | `provider/` |
| [mutations/testsupport.md](./mutations/testsupport.md) | `testsupport/` |
| [mutations/api.md](./mutations/api.md) | `api/` |
| [mutations/worker.md](./mutations/worker.md) | `worker/` |
| [mutations/ui.md](./mutations/ui.md) | `ui/` |
| [mutations/cli.md](./mutations/cli.md) | `cli/` |
| [mutations/migrator.md](./mutations/migrator.md) | `migrator/` |
| [mutations/packaging-chart.md](./mutations/packaging-chart.md) | `packaging/chart/` and `packaging/tests/chainsaw/` |
| [mutations/cross-component.md](./mutations/cross-component.md) | More than one component's directory |
