---
paths:
  - "db/**"
  - "migrate/**"
  - ".sqlfluff"
  - ".sqlfluffignore"
---

# Rules for the data-access library and the migration step

You are touching statements, migrations, the bootstrap, generated data-access code, or the migration
image. The library's layout is [db/README.md](../../db/README.md), and the migration image's is
[CLAUDE.md](../../CLAUDE.md#code-layout-and-conventions), under Components and Images. These are the
tripwires:

- **Migrations are hand-written, forward-only SQL** (ADR-0048), and no migration creates a trigger,
  a procedure or a function (ADR-0060). Grants name roles literally, and roles and extensions come
  from the superuser bootstrap before the chain (ADR-0048).
- **SQL lives only in statement files**, grouped by concern into data-access subsections, never as
  strings in a component (ADR-0047, ADR-0066).
- **Every statement meets ADR-0066's five constraints**, and the generator's suppression annotation
  appears in no statement file. The checks in `db/check` run over the real library and over their
  own test library.
- **Generated code is never edited by hand.** Regenerate it, and the drift checks compare it.
- **A component's import list names each subsection it uses**, and the grant check tests that list
  against the component's role (ADR-0066, ADR-0071).
- **goose runs as a command** and is built with its exclusion tags in `migrate/Dockerfile`, never
  linked into the project's binaries or run through `go tool` (ADR-0067).
