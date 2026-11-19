# Mutations: migrator

The demonstrations of the controls whose patches sit in `migrator/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## The chart sets the migration Job a configuration the migrator starts with

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** the chart leaves the log level out of the Job's environment though the values set one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartSetsAConfigurationTheMigratorStartsWith`
- **Break (2):** the chart takes the migrator's log level from the shared configuration whatever the migrator's own block sets
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartSetsAConfigurationTheMigratorStartsWith`
- **Break (3):** the chart names the database section's variables with one underscore after the section, a name the migrator does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartSetsAConfigurationTheMigratorStartsWith`
- **Break (4):** the schema admits a key in the migrator's own block that the migrator does not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/migrator/app`:** `TestTheChartAdmitsEveryMigratorKeyItDoesNotRender`
