# dbconnect

A narrow, named shared library, published as `mediated-mailbox-dbconnect`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every deployable that connects to the database takes its connection settings as one `database`
section of its configuration, and connects the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). This
library is that section and that connection. The section's type and its validation are pure and sit
in `core/`. The shell renders every setting into the connection string, reads the password from the
mounted password file ([ADR-0079](../docs/adr/operability/0079-secrets-arrive-as-mounted-files.md))
and refuses a start while `PGPASSWORD` or `PGSSLPASSWORD` is set. Each deployable supplies its own
defaults, the user being its own runtime role
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md)).

The case for one library over per-deployable glue is the database driver's fallbacks. pgx reads its
`PG*` environment variables on every parse, with no option to stop it. A copy that left a setting
out of the connection string, an empty `sslrootcert` included, would take that setting from the
environment. A copy that did not refuse `PGPASSWORD` would let a stray one win over the mounted
password file, and one that trimmed the file's text differently would connect with a different
password. A copy that validated less would let the driver substitute a default the operator never
chose, such as its socket directory for an empty host. Each of these fails silently. Written once,
with a test and a mutation patch for each rule, they hold in every deployable that connects.

The library connects as no role of its own and runs no statement. It returns the connection's
configuration, and the deployable opens the pool.
