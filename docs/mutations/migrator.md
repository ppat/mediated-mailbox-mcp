# Mutations: migrator

The demonstrations of the controls whose patches sit in `migrator/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## The chart sets the migration Job a configuration the migrator starts with

- **Date · evidence:** 2026-10-15 · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), at RAPID_SEED=1, and every break again on 2026-10-15 at RAPID_SEED=1, after the host's start, shared with a process that runs once, changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397), and every break again, with breaks (3) and (4) added, on 2026-10-15 at RAPID_SEED=1, after the migrator's own configuration block changed the code, tests or patches the row rests on · [pull request #397](https://github.com/ppat/mediated-mailbox-mcp/pull/397)
- **Break (1):** the chart leaves the log level out of the Job's environment though the values set one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartSetsAConfigurationTheMigratorStartsWith`
- **Break (2):** the chart names the database section's variables with one underscore after the section, a name the migrator does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartSetsAConfigurationTheMigratorStartsWith`
- **Break (3):** the chart takes the migrator's log level from the shared configuration whatever the migrator's own block sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartSetsAConfigurationTheMigratorStartsWith`
- **Break (4):** the schema admits a key in the migrator's own block that the migrator does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartAdmitsEveryMigratorKeyItDoesNotRender`
