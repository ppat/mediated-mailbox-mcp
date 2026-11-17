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
  a procedure or a function (ADR-0060). The project names no database user. The two users, the
  application user and the migrator's user, are created by the deployment platform before the chain,
  under names the deployment chooses (ADR-0143). The chain makes every grant to the application
  user's name, which the migrator is given as a required setting, by writing every grantee as the
  placeholder `application_user`, which `db/migrate` replaces with that name and `db/check` holds to
  the grantee position, refusing every other role a migration names and every statement that
  creates, renames or drops a role or sets or resets the session's role or authorization (ADR-0048).
  The chain's first migration creates the one extension the schema needs, which is trusted, so
  nothing runs as a superuser inside the application database (ADR-0048).
- **Every migration after the baseline declares which binaries run on the schema it leaves**, the
  line `-- compatibility: every chain the floor admits runs on this migration`, which says no
  binary whose chain is at or above the current floor depends on what it removes or changes, or a
  raise of the
  compatibility floor to E in the one form `db/check` accepts, where E is the oldest chain whose
  binaries do not depend on what the migration removes or changes. Expand and contract is the
  default, and a raised floor makes every older binary refuse to start (ADR-0140).
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
- **A component's import list names each subsection it uses**, and the grant check plans every
  statement of every subsection a list admits, a shared library's list included, under the
  application user (ADR-0066, ADR-0071, ADR-0143). One user's grants cannot tell programs apart, so
  the list is what holds one program's code to less than another's, and the raw SQL analyser
  refuses any other statement in the code of the API server, the UI and the worker.
- **The API server, the UI and the worker connect as the one application user, whose grants stay
  within ADR-0143's line**, its writes exactly what the data-access library's statements write, no
  DDL right, and no update or delete on the audit log or the policy history.
  Nothing automated refuses a grant beyond it, so review each migration that grants a privilege
  against it.
- **The subsections the UI's import list admits hold no write outside ADR-0084's list of the UI's
  writes and no read of an account's stored credential** (ADR-0143). Nothing in the database holds
  it, so review each statement added to or changed in such a subsection against it.
- **No subsection another program's import list admits writes `policy_rules` or `policy_changes`,
  or reads a stored credential beyond what that program's opener needs**, so only the UI's code
  writes the policy (ADR-0143). Nothing in the database holds it either, so review each statement
  added to or changed in such a subsection against it.
- **goose is linked as a library into `db/migrate` alone**, the one package whose own import list
  admits it. It applies the chain `db/migrations` embeds under goose's advisory lock, for the
  migrator and for the integration runs alike, and `db/migrations` holds the SQL files, the Go
  file that embeds them (ADR-0139), and the chain's version with the start check's comparison
  (ADR-0140).
