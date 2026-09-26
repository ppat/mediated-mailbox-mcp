# settings

A narrow, named shared library, published as `mediated-mailbox-settings`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every deployable takes its configuration the same way
([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).
Defaults, one optional YAML file, environment variables under `MEDIATED_MAILBOX_` and command-line
flags layer per value in that order, each value named once by its YAML key path, and every mistake
in any layer is refused at start. This library is that mechanism, and nothing else. Each concern's
configuration type, its defaults and its validation stay with the concern, and a deployable's
composition root hands the library its arguments, its environment and the root type.

The case for one library over per-deployable glue is the refusals. Configuration that is misspelled,
duplicated, case-changed, of the wrong type or aimed at a key that does not exist has to stop the
process, because a setting that silently falls back to its default is the failure the fail-closed
pillar forbids. The refusals are the rules glue gets wrong. Six copies would drift, and one copy
that forgot to refuse unknown keys or to match environment names exactly would fail open. Written
once, with a test per refusal and a mutation patch for each refusal the library's own code
carries, they hold in every deployable.

The composition root passes the library its arguments and its environment, and the library reads
the one file they name. It returns the decoded configuration, each value's source and a revision
per concern, or an error naming the file and line, the environment variable or the flag at fault.
It holds no package-level state, and it holds no secret, since a value concerning a secret names a
mounted file.
