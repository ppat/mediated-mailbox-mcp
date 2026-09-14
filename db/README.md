# db

The data-access library, published as `mediated-mailbox-db`
([ADR-0047](../docs/adr/data/0047-schema-first-data-access.md)). The conventions it shares with
every component are [CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

| Path | Holds |
| --- | --- |
| `db/migrations/` | The migration chain ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). Its first entry records the extensions |
| `db/bootstrap/` | The superuser bootstrap that runs before the chain, `roles.sql` once per cluster and `extensions.sql` once per database ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)). It is also the contract a deployment platform follows |
| `db/<subsection>/` | One subsection per table or closely related group of tables of [ADR-0016](../docs/adr/data/0016-schema.md), each holding its statement files and the package generated from them ([ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)) |
| `db/tx/` | The shared transaction helper that sets and verifies the account ([ADR-0047](../docs/adr/data/0047-schema-first-data-access.md)) |
| `db/check/` | The checks over the library's own files, run as Go tests. They are the statement constraints and the suppression-annotation ban, the migration lint, the derived account-keyed table list and its exceptions, the no-table-types assertion, the check that every generated file has its statement file, the check that every SQL file is read by some check, the check of the generator configuration's layout, the check that every component list naming data-access subsections has a database role and every role names such a list, and the grant check, which plans each statement under the role of every component whose list admits its subsection. The map from component to database role those checks read is held in `db/check`. The checks' test inputs, including a small library of their own and the SQL violation files, sit under `db/check/testdata/`. Each check runs over the real library, which must be clean, and over that test library, which must produce exactly its listed problems, so no check passes over an empty input |

The generator's configuration sits at `db/sqlc.yaml`, which joins the library with its first
statement file because the generator refuses a configuration with no statements. CI always diffs the
test library's generated code, whose configuration also reads the real migration chain, and diffs
the library's own generated code whenever `db/sqlc.yaml` exists. Statement and migration files are
linted by sqlfluff, configured to read the generator's parameters as placeholders.
