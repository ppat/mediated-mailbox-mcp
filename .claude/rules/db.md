---
paths:
  - "db/**"
  - "migrator/**"
  - ".sqlfluff"
  - ".sqlfluffignore"
---

# Rules for the data-access library and the migration step

You are touching statements, migrations, the bootstrap, generated data-access code, or the
migrator, the migration step's image. The library's layout is [db/README.md](../../db/README.md),
and the migrator's is [CLAUDE.md](../../CLAUDE.md#code-layout-and-conventions), under Components
and Images. These are the tripwires:

- **Migrations are hand-written, forward-only SQL** (ADR-0048), and no migration creates a trigger,
  a procedure or a function (ADR-0060). Grants name roles literally. The roles come from the
  bootstrap before the chain, and the chain's first migration creates the one extension the schema
  needs, which is trusted, so nothing runs as a superuser inside the application database
  (ADR-0048).
- **SQL lives only in statement files**, grouped by concern into data-access subsections, never as
  strings in a component (ADR-0047, ADR-0066). The one exception is the transaction helper's own
  setting and reading back of the account, and of the base policy's scope (ADR-0112), the
  `SET LOCAL` under the driver ADR-0066 names.
- **Every statement meets ADR-0066's five constraints**, no statement folds a sender domain's case
  in SQL (ADR-0016), which `db/check` refuses in the forms its scope in `db/README.md` names and
  review holds beyond them, and the generator's suppression annotation appears in no statement file.
  The checks in `db/check` run over the real library and over their own test library.
- **Generated code is never edited by hand.** Regenerate it, and the drift checks compare it.
- **Where a generated function may be called** is the txhelper analyser's rule, in
  [CLAUDE.md](../../CLAUDE.md#go) under Static analysis and formatting.
- **A component's import list names each subsection it uses**, and the grant check tests that list
  against the component's role (ADR-0066, ADR-0071). A shared library has no role of its own, so
  its list is tested against the role of each deployable or job kind whose list admits the library,
  and each of those roles is granted what the library's statements need (ADR-0118).
- **Each deployable, and inside the worker each job kind, connects as a runtime role of its own,
  whose grants stay within ADR-0118's three lines.** Nothing automated refuses a grant beyond them,
  so review each migration that grants a privilege against the three lines. A statement reaching the
  operation log reads its plan's account through its join, so its role reads those plan columns too.
- **goose is linked as a library into `db/migrate` alone**, the one package whose own import list
  admits it. It applies the chain `db/migrations` embeds under goose's advisory lock, for the
  migrator and for the integration runs alike, and `db/migrations` holds the SQL files and the one
  Go file that embeds them (ADR-0139).
