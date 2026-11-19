# Mutations

The mutation ledger. A row here records one demonstration, that breaking a control's mechanism
made its tests go red. This ledger and [VERIFICATIONS.md](./VERIFICATIONS.md) are companions
split by authoring moment. Deciding a control mints its verification row at design time. A
demonstration can happen only at implementation time, once there is a mechanism to remove and
tests to fail. A control with no standing automated test never appears here, since nothing
exists to demand red from, and proof only by drill or by manual exercise is that case. The
decision is [ADR-0046](./adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)'s.

The ledger is this file and the files under [mutations/](./mutations/) it lists in
[Where the rows are](#where-the-rows-are). This file holds what a row is and where it goes, and
those files hold the rows.

**A row.** One control, one demonstration. It names how the mechanism was broken, the tests that
went red, the date, and an evidence pointer. The rows are the whole artifact, never a pass rate.
Every statement in a row must be true of the current code, and a row holds no history. The ledger
holds every mutation of the current code base. Each patch under a `testdata/mutations` directory,
apart from the runner's own fixtures under `testsupport/cmd/mutproof/testdata/fixture/`, is one
break of its control's row, and no break is without its patch. Every mechanism is broken at least
both ways, once so it does less and once so it does the wrong thing, one patch per break, and the
row numbers each break and names the tests each turned red. A surviving mutant holds its row open
as a defect until the tests are fixed or the mechanism is deliberately deleted as redundant, and
the row says so beside its date. The runner that applies a demonstration's patches,
`go tool mutproof` ([testsupport/README.md](../testsupport/README.md)), prints each row in this
shape. It numbers a control's breaks in the order its patches are given, so the author gives each
control's patches in the byte order of their paths, for example by running `export LC_ALL=C` in
the shell before it expands the patches' glob.

**A row's form.** A row is a section whose heading is the control, so each control has an anchor
of its own. Each field sits on a line of its own, so rewriting a row edits only the lines whose
content changed.

| Line | Holds |
| --- | --- |
| `## <control>` | The control |
| `- **Date · evidence:** <date> · <evidence>` | One date and one evidence pointer, those of the latest demonstration the row was written from, which ran every one of the row's current patches, and `· open, a surviving mutant` while a mutant survives. No earlier demonstration and no reason for running one again is kept |
| `- **Break (<n>):** <how the mechanism was broken>` | One line per break, numbered from 1 in the order of the patches' paths compared byte by byte. A control broken by one patch alone has `- **Break:**` |
| `- **Went red in <package>:** <tests>` | Nested under its break, one line per package whose tests went red, with the tests by the names the test runner reports. A surviving mutant has the nested line `- **Went red:** none, a surviving mutant` instead |

**Lifecycle.** A row is written when its control lands. Its demonstration runs again when the
control, its tests, or a generator its tests draw from changes, with the implementation work that
touches the surface area its control interacts with, while the permanent CI tests keep the
control's green continuously earned. A row is rewritten only when its patches change or need to
change, or when a statement in it would otherwise be false of the current code, such as a test
that went red being renamed or moved to another package. A row being rewritten has every patch of
its control run again in one demonstration, so its one date covers every break. Apart from those
events, the `mutation-demonstrations` workflow runs every demonstration weekly and when started by
hand, and fails when one no longer holds. Such a run writes no row by itself. A row is written from
one only by a change that rewrites the row, and the change's evidence pointer names the run
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
