# 0078. Configuration layers defaults, one optional YAML file, environment variables and flags, strictly, through a small project-owned library

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

[ADR-0051](./0051-environment-contract.md) lets configuration arrive as files, environment variables
and flags, and keeps secret material in files
([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)). It leaves the form open, and
[ADR-0042](./0042-implementation-stack.md) leaves each major library to its own decision. The
deployables' needs are structured, backfill's among the first. Accounts and their credentials are
not configuration. They live in the database, created through the UI
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)).

- **The scanner's vocabulary and tuning.** [ADR-0005](../classification/0005-tiered-detection.md)
  makes the trigger words by language, the link words and Tier 2's weights configuration, so a
  deployment adds languages and retunes without a release.

Configuration may arrive as command-line flags, environment variables and a file, more than one of
them in one deployment, with some overriding others, and nothing may assume the environment the
application is deployed into. Only secret material is barred from environment variables and flags.
Policy is not configuration. It lives in the database and is imported from a file or exported to one
([ADR-0004](../classification/0004-sender-list-decides.md)).

The choice looks like picking a popular configuration library and is not one. What separates the
candidates is whether a value inside a map the code never declared, one language's trigger words,
can come from an environment variable or a flag, and whether every mistake is refused rather than
defaulted.

| Requirement | What it demands | From |
| --- | --- | --- |
| Strict everywhere | A misspelled, duplicated, case-changed or unknown key, flag or prefixed environment variable, a second or null YAML document, a type error and an out-of-range value each stop the process at start | [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) |
| Every value from every source, per key | A flag or an environment variable can set one entry of a keyed map, or name a key the file never did, as a new language for the scanner | The operator, who barred only secret material from environment variables and flags |
| Three sources, one precedence | Defaults, file, environment, flags, each overriding the one before | The operator |
| No platform assumption | No default path, no directory layout, the file optional | The operator |
| Secrets never configuration | No configuration value can be secret material | [ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md) |
| Marginal footprint | Modules added to the processes that hold full-mailbox credentials | [ADR-0042](./0042-implementation-stack.md)'s rejection of a deep package tree inside the trust anchor, read as ordering the field |
| One definition | A value's file key, environment name, flag and help text come from one declaration | [O6](../../../USE_CASES.md#o6--deployable), which requires the inputs a deployment needs to be declared |
| Errors name source and key | A refusal names the file and line, the environment variable or the flag | This record |
| Effective configuration observable | Each value's source is known, and each concern's configuration has a revision | [ADR-0005](../classification/0005-tiered-detection.md), which stamps the scanner configuration's revision on every verdict |
| Hand-edited file | Comments, and a format the operator already edits | The operator |

Strictness, per-key reach and the three sources are gates, because each comes from
the operator's words or a pillar. Footprint then ordered the field, one definition, errors and
observability separated what remained, and maintenance, typing, package-level state and room for
subcommands broke ties. Ordering by footprint is this record's reading of ADR-0042, and it differs
from [ADR-0076](./0076-metrics-emitted-through-client-golang.md), which let footprint break ties for
the same processes. The reading under Alternatives tests the choice under that weighting too. Remote
key-value stores and policy were left out of the grading, the first because nothing in the design
calls for one and the second because policy lives in the database.

## Decision

- **Four layers, per value, in this order:** built-in defaults, then one optional YAML file, then
  environment variables, then command-line flags, each overriding the one before. AWS CLI, the
  command-line guidance at clig.dev, Spring Boot and ASP.NET Core use this order, and Kubernetes'
  kubelet lets flags override its file.
- **The file is named by a flag and an environment variable, and has no default path**, because a
  default path assumes the environment. Without one there is no file, and the other layers carry
  everything.
- **YAML**, decoded by `go.yaml.in/yaml/v3`, because the file is edited by hand and YAML carries
  comments, because the chart's values an operator edits are YAML already, and because this decoder
  refuses duplicate keys, which the standard library's JSON decoder silently accepts.
- **One name per value**, so the file, the environment, the flags and the help cannot drift apart
  ([O6](../../../USE_CASES.md#o6--deployable)). The YAML key path is the name. The environment name
  is the prefix `MEDIATED_MAILBOX_` plus the path upper-cased with segments joined by `__`, so
  `scanner.triggers.de` is `MEDIATED_MAILBOX_SCANNER__TRIGGERS__DE` and the flag
  `--scanner.triggers.de`. `--help` is generated from the same declaration and lists each
  value's path, environment name, flag and default. Every deployable uses the one prefix.
- **Keyed values.** Per-language word lists are a map keyed by language, so a flag or an
  environment variable can override one language's list or add a language. Map keys match
  `[a-z0-9_]+` without `__`, because a key becomes a segment of an environment name, so a language
  tag such as `pt-BR` is written `pt_br`.
- **A value from an environment variable or a flag.** A string field takes the text verbatim. Any
  other field decodes the text as a YAML value into the field's type, so a list is
  `[verify, "log in"]`. An explicit null is refused.
- **Merging.** Maps merge per key. Scalars and lists replace whole, as Kubernetes and Helm merge
  values, so a list given in one layer is never spliced with another layer's.
- **Strictness.** An unknown, duplicated or case-changed key, a second or null document, an alias or
  merge key, an unknown flag, an unknown or mixed-case environment variable under the prefix, a type
  error, a missing required value and a value outside its designed range are each refused at
  start, by an error naming the file and line, the environment variable or the flag.
- **Secret material is never a value** ([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)).
  A value that concerns a secret holds the path of a mounted file, read only by the shell that uses
  it, so no configuration type holds secret material. Each concern's configuration type is pinned
  field by field, so a new value, one meant to hold a secret included, is a visible change.
- **The database connection.** pgx reads its `PG*` environment variables on every parse, among them
  `PGPASSWORD`, with no option to stop it, and a stray one wins over the mounted password file. So
  every connection setting a deployable uses is a configuration value rendered into the connection
  string on every start, the password comes from a password file, and a deployable refuses to start
  while `PGPASSWORD` or `PGSSLPASSWORD` is set.
- **Read once, at start, by the library.** The composition root passes the library its arguments,
  its environment and its root configuration type. The library reads the file those name, layers the
  values, and returns them with each value's source and a revision per top-level concern, or an
  error. A pure validation per concern then checks the merged value before anything else starts
  ([ADR-0040](./0040-pure-core-decisions-as-values.md)). A change takes a restart. The effective
  configuration is logged at start with each value's source. The scanner's revision stamped on every
  verdict is the library's revision of the scanner's section, so a change in any layer changes it.
- **The environment is read in one place.** In the deployables and the shared libraries, outside
  test files and the test tooling, the environment is read only by a deployable's composition root,
  and handed to the library from there.
- **The library is the project's own**, `settings/`, about five hundred lines on
  `go.yaml.in/yaml/v3` and the standard library. The grid below does not separate it from the
  flag-layer koanf column on any gate, so this is a preference. It adds no module beyond the YAML
  decoder, already in `go.mod`, to the processes that hold full-mailbox credentials, and every
  refusal is its own code rather than a library default that fails open when an override is lost,
  at the cost of about two hundred more owned lines. A reader who weighs owned code above those
  modules lands on the koanf column. `Load` holds no package-level state, and it holds no concern's
  schema and no validation, which stay with each concern.
- **No command-line framework**, because no deployable has a mode.

Left open, each settled where it is first needed:

- **More than one file.** One file is read today. A repeatable file flag can be added, costing the
  library a merge across files and a rule for which file wins a conflict, as Helm takes the last
  file given and kubeconfig the first.
- **A command-line mode.** The first deployable that gains one chooses between cobra, whose command
  receives only its parsed arguments and which adds a module, and one standard-library flag set per
  mode, which adds nothing but hand-written dispatch. Either sits in front of `Load` and moves
  nothing else here.

What the ordinary path of the libraries measured does that this record forbids:

| Construction | Harm | What stops it |
| --- | --- | --- |
| An unknown file key accepted and zero-valued | A misspelled setting silently takes its default | The strict decode, with a test and a mutation patch |
| A case-insensitive key match | `Triggers` and `triggers` race | Exact matching, with a test |
| A stray `PGPASSWORD` | It overrides the mounted password file | The refusal at start, with a test and a mutation patch |
| `os.Getenv`, `os.LookupEnv` or `os.Environ` in a deployable or a shared library outside a composition root | A setting bypasses the layers and their refusals | A `go vet` analyser under [ADR-0071](./0071-static-enforcement-toolchain.md), scoped by path to the deployables and the shared libraries outside composition roots, test files and the test tooling, proven by a violation file. A `forbidigo` rule cannot carry that scope, because exempting files from it takes an exclusion naming a linter that stands in for a control, which ADR-0071 refuses |

| Requirement | Met by |
| --- | --- |
| Strict everywhere | `KnownFields`, the document, null and alias checks, paths resolved against the declaration, exact-case environment names, typed decode with null refused, required fields |
| Every value from every source, per key | Map keys as path segments, flags resolved as key paths |
| Three sources, one precedence | One `Load` applying the four layers in order |
| No platform assumption | No default path, no directory, the file optional |
| Secrets never configuration | Values that name mounted files, types pinned field by field, the password file and the `PG*` refusal |
| Marginal footprint | No module beyond the YAML decoder, measured at two packages and under half a percent of a provider-calling deployable's binary |
| One definition | The YAML tag as the only name, and generated help |
| Errors name source and key | Every refusal wrapped with its file, environment variable or flag |
| Effective configuration observable | A source per value and a revision per concern |
| Hand-edited file | YAML comments, the chart's own format |

What an implementer would otherwise pay to discover:

- `go.yaml.in/yaml/v3` is the maintained continuation of the archived `gopkg.in/yaml.v3`. It
  refuses duplicate keys always, refuses unknown keys under `KnownFields(true)`, and its boolean
  table holds only `true` and `false`, so a language code `no` is never a boolean. It does not
  refuse a second document, a null document or an alias on its own, so the library checks those.
- Kubernetes injects `<SERVICE>_SERVICE_HOST`-style variables for every Service in a namespace, and
  a Service named `mediated-mailbox-ui` yields `MEDIATED_MAILBOX_UI_SERVICE_HOST`, which the
  unknown-variable refusal would refuse. A chart sets `enableServiceLinks: false`.
- With one prefix shared by every deployable and each refusing variables it does not read, an
  environment shared by several deployables fails to start. Each deployable is given only its own
  variables.
- A chart that re-serializes values drops comments and can turn `no` into a boolean before the
  process reads the file, so it passes the file through as the text the operator wrote.

## Alternatives considered

Nine libraries were built in scratch programs against the same configuration, a file with two
accounts in a map, one with a lowered rate target, a per-language word list and a flat listen
address, overridden from the environment and flags and attacked with misspellings, duplicates and
malformed values. Each failure to reach an account's field is a failure to reach any entry of a
keyed map, as viper's was also measured on the word list, so the grades hold for the language map
that remains. Grades run 4 (carries it natively), 3 (with bounded discipline or one small
package), 2 (only by convention or with a named gotcha) and 1 (cannot).

| Candidate | Strict | Per key, env | Per key, flags | Precedence | Footprint | One definition | Errors | Observable | Maintained |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **Owned library** | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 3 |
| koanf, corrected, with an owned flag layer | 4 | 4 | 4 | 4 | 2 | 3 | 3 | 3 | 2 |
| koanf, corrected | 4 | 4 | 1 | 4 | 2 | 2 | 3 | 2 | 2 |
| kong with its YAML resolver | 3 | 1 | 1 | 4 | 3 | 4 | 4 | 2 | 3 |
| urfave/cli v3 with its file sources | 3 | 1 | 1 | 4 | 3 | 3 | 4 | 3 | 4 |
| ff v4 | 3 | 1 | 1 | 4 | 3 | 3 | 4 | 2 | 1 |
| aconfig | 3 | 1 | 1 | 4 | 3 | 3 | 3 | 2 | 2 |
| ardanlabs/conf | 1 | 2 | 3 | 3 | 3 | 4 | 2 | 2 | 3 |
| viper | 1 | 1 | 2 | 3 | 1 | 2 | 1 | 2 | 3 |
| Environment-only libraries | 2 | 2 | 1 | 1 | 4 | 3 | 2 | 2 | 3 |

The grid shows the following, derived per column.

- Only the owned library has no cell below 3, its lowest being Maintained, because the project
  maintains it. The koanf column with a flag layer has no 1 and carries two 2s, Footprint and
  Maintained. Each of the other eight rows carries at least one 1, and each carries one on a gate,
  strictness, per-key reach from the environment or from flags, or precedence.
- Precedence separated almost no one. Every library that reads all three sources carries it, and
  only the environment-only libraries fail it, because they read one source.
- Per-key reach from the environment is where most candidates fall. kong, urfave/cli and ff
  resolve the file through the flags the code declares, and aconfig panics, so a key the code never
  named cannot be configured.
- No platform assumption, secrets never configuration and every tie-breaker but maintenance
  separated no one, and have no column. Any of the libraries can be used without a default path,
  secret material is kept out by the schema whatever parses it, and the other tie-breakers moved no
  pair of rows. The hand-edited file is graded in the format table below.
- koanf corrected is dominated by the koanf column with a flag layer, which is the same program plus
  a small owned layer. viper is dominated on every gate by the same column.
- The environment-only row, the Maintained column, and the Observable column for every row but the
  two leaders were read from documentation, release feeds and source. Every other cell rests on a
  run. Footprint was measured against a program linking what a provider-calling deployable links for
  the two leaders, and against a standard-library program for the others.

The file format was graded apart, on the same cases, since it is independent of the library.

| Format and decoder | Comments | Duplicate key refused | Unknown key refused | `no` stays a string | Decoder added | The chart's format |
| --- | --- | --- | --- | --- | --- | --- |
| **YAML, `go.yaml.in/yaml/v3`** | 4 | 4 | 4 | 4 | 4, already in `go.mod` | 4 |
| TOML, `BurntSushi/toml` | 4 | 4 | 3, checked after decoding | 4 | 3 | 2 |
| TOML, `pelletier/go-toml` v2 | 4 | 4 | 4 | 4 | 3 | 2 |
| JSON, `encoding/json` | 1 | 1, the last value wins | 4 | 4 | 4 | 2 |

Only the YAML row has no cell below 3. TOML is a close second on everything but the chart's
format.

| | Owned library | koanf with an owned flag layer |
| --- | --- | --- |
| Owned lines, non-blank and non-comment | about 530, plus each concern's schema | about 340, plus a revision still to write |
| Modules added to a deployable | none beyond the YAML decoder | nine, two of them archived (`mitchellh/copystructure`, `mitchellh/reflectwalk`) |
| Packages and binary size added to a provider-calling deployable | 2, 0.46% | 10, 2.02% |
| Library defaults that fail open if the project's override is lost | none | three, weak typing, unknown keys ignored and keys matched ignoring case |

**The reading.** The owned library's worst cells are 3s, both for being the project's own code,
where the flag-layer koanf column's worst are 2s, footprint in the credential-holding processes and
two archived modules beneath it. Nothing else in the design guards configuration strictness, so a
lenient default reaching production is guarded by nothing, which makes the koanf column's three
defaults that fail open a single point of failure rather than a redundant weakness. Regretting
either costs the same, a swap behind `Load`, since the file format, the names, the precedence and
every refusal survive, but the failures surface differently. A refusal bug fails the start loudly
in both, while a merge or precedence bug yields a wrong value silently in both, found through the
source logged for each value, and only the koanf column can also regress through a library release
or a lost override. The owned library's once-decisive win, flags for keys the code never
declared, turned out to be detachable, since the koanf column gets it in a small owned layer, so the
choice comes down to about two hundred more owned lines against nine modules and three defaults
that fail open. Read with footprint only breaking ties, as ADR-0076 weighed it, the two leaders tie
on every gate and the owned library still leads on one definition, errors and observability, so the
preference holds by a smaller margin.

- **The owned library.** For it, one mechanism meets every requirement, the one name cannot drift
  between file, environment, flag and help, and every failure is the project's own code, pinned by
  its tests. Against it, several hundred lines of reflection the project writes and owns
  permanently, which grew under review as leniencies were found and will grow again, and the choice
  forgoes the existing libraries it was measured against. One resolution function carries both the
  unknown-variable and unknown-flag refusals, so one bug there breaks both.
- **koanf, corrected, with an owned flag layer.** For it, about two hundred fewer owned lines and a
  maintained library doing the merge and the decode. Against it, the project still owns the strict
  checks, the flag layer, the help and the provenance, so koanf carries only the merge and decode,
  and for that it brings nine modules into the processes holding full-mailbox credentials and three
  defaults that fail open silently if a future edit drops the one configuration literal that
  overrides two of them.
- **kong, urfave/cli and ff.** For them, tags that define a value once, clear typed errors, and
  subcommands. Against them, each resolves the file through the declared flag tree, so a key the
  code does not declare cannot come from the file at all. ff v4 has been a beta release since
  August 2025.
- **aconfig.** For it, the standard precedence and unknown variables refused by default. Against
  it, its map-of-struct decode is wrong, and an environment value aimed at a keyed map panics the
  process.
- **ardanlabs/conf.** For it, the clearest help. Against it, a misspelled file key is accepted and
  zero-valued.
- **viper.** For it, the most familiar name. Against it, an environment override never reaches an
  entry inside a keyed map, an environment type error is silently ignored, keys are lower-cased,
  and it is the heaviest candidate.
- **Environment-only libraries.** Small and typed, but they read one source.
- **JSON files.** For them, the standard library's decoder. Against them, no comments in a
  hand-edited file, and `encoding/json` silently keeps the last of two duplicate keys.
- **TOML files.** For them, comments and an unambiguous grammar. Against them, a new module, an
  empty file decoding silently, a keyed map of tables that reads less plainly, and a format unlike
  the chart's. A close second.
- **A separate prefix per deployable.** For it, a shared environment serving several deployables
  cannot trip one on another's variables. Against it, a value several deployables read, the
  database's host or the scanner's section, is spelled once per deployable, so a configuration
  shared across deployables is written several times and drifts.
- **Accepting unknown variables under the prefix.** For it, one environment can serve every
  deployable. Against it, a misspelled variable is silently ignored and its value's default applies,
  the failure the strictness gate exists to refuse.
- **Reloading configuration while running.** For it, a change without a restart. Against it, every
  consumer of a value would have to take a new one safely mid-run, which nothing in the design
  needs, where policy, which does change while the system runs, already has its own reload
  ([ADR-0041](./0041-policy-as-immutable-snapshots.md)). A file watch re-running `Load` can be added
  if a need appears.

## Consequences

- `settings/` is a narrow shared library under
  [ADR-0050](./0050-shared-code-pure-or-narrow.md), arguing its case in its README. Every deployable
  that reads configuration imports it.
- A configuration change takes a restart. A pod of an older version refuses a value it does not
  know, so a value a release adds is set only once no pod of an older version will start again.
- Every refusal the library carries lands with a test and a mutation patch, and each concern's
  configuration type is pinned field by field, so a new value is a visible change.
- pgx still reads `PG*` variables for any connection setting the project does not render.
- The scanner's configuration type stops carrying a revision an operator sets. The revision it
  stamps is the library's revision of its section, handed in by the composition root, and it
  becomes a string, so the scanner's refusal of a revision below 1 becomes a refusal of an empty
  one. The first deployable that builds the scanner lands the change.
- Leaving the library costs its internals only. The file format, the names, the precedence and
  every refusal are the contract, and any replacement sits behind `Load`.
- What would re-argue it is a maintained Go library that derives every name from one tag, discovers
  map keys from the environment and flags, decodes strictly and records sources, or koanf dropping
  its archived dependencies.
- It adds duties to the chart ([ADR-0052](./0052-kubernetes-deployment-helm-chart.md)): restarting
  pods when their configuration changes, passing the file through as written, giving each deployable
  only its own environment variables, and turning Kubernetes' injected service variables off.
- Accepting more than one file adds a rule for which file wins. Giving a deployable a mode adds a
  parser in front of `Load` and changes nothing here.
- Assumptions about other components. A deployment mounts secrets as files, and the chart carries
  the duties above.
