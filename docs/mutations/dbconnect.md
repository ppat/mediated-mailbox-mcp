# Mutations: dbconnect

The demonstrations of the controls whose patches sit in `dbconnect/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A deployable refuses to start while PGPASSWORD or PGSSLPASSWORD is set, an empty value included

- **Date · evidence:** 2026-09-28 · [pull request #181](https://github.com/ppat/mediated-mailbox-mcp/pull/181)
- **Break (1):** a password variable set to the empty value does not refuse the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAPasswordVariableIsRefused`, `TestAPasswordVariableIsRefused/PGPASSWORD=`, `TestAPasswordVariableIsRefused/PGSSLPASSWORD=`
- **Break (2):** every PG* variable refuses the start, not only the two password variables
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAPasswordVariableIsRefused`, `TestAPasswordVariableIsRefused/PGPASSWORD=`, `TestAPasswordVariableIsRefused/PGPASSWORD=secret`, `TestAPasswordVariableIsRefused/PGSSLPASSWORD=`, `TestAPasswordVariableIsRefused/PGSSLPASSWORD=secret`, `TestOtherVariablesAreNotRefused`
- **Break (3):** PGPASSWORD does not refuse the start, so a stray one wins over the password file
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAPasswordVariableIsRefused`, `TestAPasswordVariableIsRefused/PGPASSWORD=`, `TestAPasswordVariableIsRefused/PGPASSWORD=secret`
- **Break (4):** PGSSLPASSWORD does not refuse the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAPasswordVariableIsRefused`, `TestAPasswordVariableIsRefused/PGSSLPASSWORD=`, `TestAPasswordVariableIsRefused/PGSSLPASSWORD=secret`

## The database password is the mounted password file's text less one trailing newline, and an empty one refuses the start

- **Date · evidence:** 2026-09-28 · [pull request #181](https://github.com/ppat/mediated-mailbox-mcp/pull/181)
- **Break (1):** a trailing \r\n loses only its \n, so the password keeps a carriage return
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAnEmptyPasswordIsRefused`, `TestThePasswordIsTheFilesText`
- **Break (2):** a password file holding no password gives an empty password rather than refusing the start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAnEmptyPasswordIsRefused`
- **Break (3):** every trailing newline is trimmed, not only one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestThePasswordIsTheFilesText`
- **Break (4):** the password file is read but its password is not set on the connection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestTheConnectionIsTheConfigurations`, `TestThePasswordIsTheFilesText`
- **Break (5):** no trailing newline is trimmed from the password
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/dbconnect`:** `TestAnEmptyPasswordIsRefused`, `TestThePasswordIsTheFilesText`
