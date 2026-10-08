# 0071. Static enforcement runs under golangci-lint, with import boundaries as a closed allow list

**Status:** Accepted · **Pillar:** [Unsafe states are unconstructable, not merely
untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) · **Serves:**
[C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

[ADR-0042](./0042-implementation-stack.md) commits the project to Go and says plainly that Go
carries the unconstructability pillar only with help. It names the conventions that do the carrying,
and then names three kinds of linter that run in continuous integration as, in its words, deputized
enforcement for what the compiler does not check natively. It names no tool for any of them.
[ADR-0040](./0040-pure-core-decisions-as-values.md) requires that a pure core's import boundary is
checked rather than trusted, and [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)
requires that one deployable cannot import another's code. Neither names a tool either. Separately,
[ADR-0070](./0070-unit-comparison-through-one-options-value.md) and
[ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) each ban calls to named functions,
which is not a kind [ADR-0042](./0042-implementation-stack.md) lists and still needs an analyser to
refuse, and ADR-0069 confines a test library to named packages, so this record hosts those too.

These are not style checks. Several of them are controls with rows in
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md), so they are judged by the standard
[ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) sets for a ban, which is that it is
proven by a checked-in file violating it, because a ban can read correctly and match nothing.

### What this choice is not

It reads as a question of which linters to enable, which suggests comparing rule catalogues. Two
things make it a different question.

The first is that a linter's shipped default can be less safe than this project's own coding
conventions, and when it is, the check goes quiet on exactly the code the conventions produce. That
is not a matter of catalogue size and it is invisible from a feature list.

The second is that expressing a rule as a list of what is forbidden and expressing it as a list of
what is permitted are not two spellings of one rule. A core that must reach nothing performing
input or output is a claim about everything, so a forbidden list is incomplete the day the language
adds something, and nothing announces that.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| A rule is expressed as what is permitted | The configuration states the allowed set, so something nobody enumerated is refused rather than admitted | This record's requirement, from [ADR-0040](./0040-pure-core-decisions-as-values.md)'s core rule being a claim about everything a package reaches |
| A standard-library package can be refused | The rule reaches `os`, `net/http`, `os/exec` and their kind, because those are where reading a file, opening a socket and running a command live | This record's requirement, because [ADR-0040](./0040-pure-core-decisions-as-values.md)'s core rule is almost entirely about them |
| A violation fails the build and says where | A non-zero exit and a message naming file, line and rule, stable enough for a script to assert on | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| A rule cannot be switched off quietly | Either no way to suppress a finding at the place it fires, or suppression that a reviewer can see | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| Configuration is checked in and readable | One file a reviewer reads, rather than flags spread through a pipeline | [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md) |
| Works on the Go version the project builds with | The tool builds and runs against the current toolchain rather than a version behind it | [ADR-0042](./0042-implementation-stack.md) |
| Footprint | What the choice adds to the repository's tool surface | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture applied outside the runtime |

**How these were weighted.** Two requirements ordered the import-boundary question. Refusing a
standard-library package came first, because the pure-core rule is almost entirely about such
packages and a candidate that cannot reach them cannot carry the rule at all. Expressing a rule as
what is permitted came second, because the rule makes a claim about everything and a forbidden list
is a guess about the future. Working on the current Go version ordered the separate question of
whether to run the analysers individually or under one command, and did so unexpectedly. Everything
else broke ties.

## Decision

- **`golangci-lint` runs the analysers, configured by one checked-in file.** Not for convenience.
  The analysers this project needs are small, single-purpose projects that are released rarely, and
  the aggregator rebuilds them against a current toolchain. The exhaustiveness analyser's only
  tagged release pins a dependency that no longer builds on current Go releases, and the same
  analyser works correctly through the aggregator, which pins a newer one.
- **Exhaustiveness over enumerated types is checked by `exhaustive`**, configured with
  `default-signifies-exhaustive: false`. That is its own default, and it is written out anyway,
  because the opposite value silences the check on every switch this project is required to write.
- **Unchecked errors are checked by `errcheck`**, configured with `check-blank: true` and
  `check-type-assertions: true`. Both default to `false`, so an error discarded as `_ = f()` is not
  reported by an otherwise ordinary configuration.
- **Import boundaries are checked by `depguard` with `list-mode: strict`, as `allow` lists with no
  `deny` list anywhere.** The ban-proof script refuses a list whose `list-mode` is anything but
  `strict`, including one with no `list-mode` written, which `depguard` reads in its `original` mode
  and which then admits every import when the list has no `allow` entries. It refuses a `deny` key
  on any list, with entries or without. `depguard` makes a file that two lists match satisfy both,
  and does not check a file that no list matches. The lists are these, and
  [CLAUDE.md](../../../CLAUDE.md#components) names the directories they match.
  - **Non-test files of every pure-core package, in any component.** A pure core may import other
    pure-core packages and a named set of outside packages, and nothing else. An outside package
    joins the set only when its code, and the code of every package it imports, meets all four of
    these conditions. Outside the test are the Go runtime, `unsafe`, the standard library's
    `internal/` packages, and `sync` and `io`, whose package state no caller can see. A choice
    between implementations made from the processor's features is the runtime's too, so `math`
    passes.
    1. No code reads or writes a file, the network, the environment or the operating system.
    2. No code makes a system call.
    3. No package-level state holds what an earlier call computed from its input. A cache fails
       this, because it lets a later call skip the computation. A table or data set whose content
       is fixed passes, whether it is built at initialization or on first use, and so does a pool
       that recycles working memory and holds no result. `regexp` passes on both counts, and a
       pattern compiled once into a package-level value is such a table.
    4. No code has an effect beyond its return values and the values its caller passes in.

    `golang.org/x/net/idna` fails condition 1, because it imports `fmt`, which imports `os`.
    `golang.org/x/net/publicsuffix` fails it too, because it imports `net/http/cookiejar`.

    The list covers the pure-core packages inside deployables and libraries as well as the shared
    pure library, matched by path. A pure-core package is one whose path, from the repository root,
    holds an element named `core` at any depth, so a pure core is matched wherever a component's
    layout puts it, a library holding several packages of one concept included
    ([ADR-0050](./0050-shared-code-pure-or-narrow.md)). Admitting a package is review discipline,
    recorded as the closure row in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), because no tool
    checks it.
  - **Each deployable.** A deployable may import its own code, the shared pure library, the
    data-access subsections its list names, and the other named libraries of
    [ADR-0050](./0050-shared-code-pure-or-narrow.md) it uses, beside the standard library and the
    outside modules its own code needs, and nothing belonging to another deployable
    ([ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)). In the worker that runs
    every background job kind
    ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)), each job kind
    has a list of its own, inside the deployable's, so the isolation of one job kind's code from
    another's is checked rather than conventional. A job kind that needs no credential, no provider
    and no account session imports none of them, and each job kind's list names the data-access
    subsections its own role is granted
    ([ADR-0118](../data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)).
  - **Each shared library.** The data-access library, the provider library, the rate limiter and the
    shared test support each have a list. A component's list names each data-access subsection it
    may use, so the grant check of [ADR-0066](../data/0066-data-access-generated-from-sql.md) can
    test the list against the role's grants, while the lists over code that never ships, and over
    every component's files at once, admit the whole data-access library.
  - **The mediator's two protocol roots**, each admitting the service layer and nothing below it
    ([ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md)).
  - **The UI's non-test code outside its one opening part.** It admits the standard library and
    names exactly the seven packages under `crypto/` that code uses, `crypto/aes`, `crypto/cipher`,
    `crypto/hkdf`, `crypto/hmac`, `crypto/rand`, `crypto/sha256` and `crypto/tls`. Since each
    import is compared with the entry sorted just before it, it refuses every other package under
    `crypto/`, the public-key code an opening is built on, `crypto/hpke`, `crypto/mlkem` and
    `crypto/ecdh`, among them
    ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)). The root `crypto`
    package, which holds interfaces and the hash registry and no algorithm, stays admitted. Test files are
    outside it, because a test opens what the UI sealed to check it and is never served.
  - **Non-test code everywhere.** It refuses the shared test support, the provider fake, the
    contract suite, the property-testing library and the comparison library, so none of them reaches
    code that ships.
  - **Test files.** They are matched by lists of their own, because a test imports what its
    package's non-test files may not. A test file may import what its package's non-test files may,
    the standard library's `testing` package, the comparison support of
    [ADR-0070](./0070-unit-comparison-through-one-options-value.md), the provider fake and the
    contract suite of [ADR-0043](./0043-no-mocking.md), the test support an integration test needs
    to reach its database ([ADR-0068](./0068-test-substrate-containers-directly.md)), and the
    outside modules its tests need. The property-testing library
    [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) chooses, ADR-0069's test support
    and the crash harness are admitted only in property-test files, crash-sequence files, and the
    test support that needs them, and `golang.org/x/tools` only in the shared test support, which
    holds the helper that loads a package which must not compile and the `go vet` analysers.
  - **A list over every Go file outside all components**, admitting only the standard library, so a
    directory added outside every other list is still checked.

  No list other than those named above names the property-testing library, so it is refused
  everywhere else with no `deny` key written. That holds only while every Go file in the repository
  is matched by some list, which the list over files outside every component keeps true for such
  files and review keeps true inside the components.
- **A pure core holds no package-level state, and the project's own `go vet` analysers check it.**
  The import lists cannot see this. A variable a composition root sets is a dependency no parameter
  of [ADR-0040](./0040-pure-core-decisions-as-values.md)'s pure core shows, and an exported error
  value is such a variable too. Four rules carry it.
  - A non-test file of a pure-core package declares no package-level variable other than the blank
    identifier and an error value. An error value has no initial value or is made by `errors.New`
    from a constant message, so it holds nothing a call could change.
  - No file anywhere writes a pure-core package's package-level variable except by its declaration.
    A write counts whether it assigns, increments, assigns in a range loop, takes the variable's
    address, or goes through a field, an index or a pointer. A variable a pure core's test file
    declares is not part of the core, so its own package and that package's external tests may
    write it, and whether its file is a test file is read with line directives ignored.
  - No file links to a pure-core package's symbol with a `go:linkname` directive, which would reach
    a variable around its declaration.
  - A non-test file of a pure-core package carries no line directive. Export data gives a variable
    the file name a directive names, so a directive would make a variable of the core pass for a
    test file's in the package's external tests.

  The rules sit in their own analyser beside the one carrying
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)'s placement rules, run by the same
  program. It honours no suppression comment and matches a pure-core package by its path, as the
  import list does.
- **The UI registers a route only through its recording mux, and the project's own `go vet`
  analysers refuse the registrations they can see.**
  [ADR-0057](../operability/0057-one-dataset-endpoint-behind-a-registry.md)'s route check compares
  every route registered through the recording mux's `Handle` with the contract document, so a
  route registered on a mux any other way is one the check never sees. The routes analyser reports,
  in a non-test file of a package under `ui/`, every use of `(*http.ServeMux).Handle`,
  `(*http.ServeMux).HandleFunc`, `http.Handle` and `http.HandleFunc`, and every use of a method
  named `Handle` or `HandleFunc` with the mux's parameters on an interface or a type parameter,
  which a mux can be held in. A parameter spelled through an alias counts as the type it stands
  for, as it does when Go decides that a mux satisfies the interface. A call and a method value are
  reported alike. The one exemption is the body of the recording mux's own `Handle` method, which
  the analyser names by its full name, so renaming the method or its type turns the exemption into
  a finding. So those functions and methods used on the mux beneath the recorder anywhere else in
  package `api`, or on a second mux built anywhere under `ui/`, are refused. Three registrations
  stay with review. No analyser of these calls sees the first two. One is a registration through
  reflection. The other is a route an imported package registers on the default mux when it is
  initialised, as `net/http/pprof` does, which the UI's import list admits with the rest of the
  standard library, and which is served only if a server of the UI is given no handler. The third
  is a registration through an interface method one of whose parameter types is a type parameter,
  which only a mounting helper generic over the mux and over its handler or its pattern writes.
  Matching a type parameter there would add a rule that nothing in the UI needs. Test files are
  exempt, since a route a test registers is never served. The rule sits in its own analyser, run by
  the same program, and honours no suppression comment. The mediator is outside its scope, for the
  reason the alternatives give.
- **The UI runs a statement only through the data-access library, and the project's own `go vet`
  analysers refuse the statements they can see.** The UI's role may read an OAuth client's sealed
  secret for the UI's client-secret part alone
  ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)). The import lists confine
  that read to the part by the subsection it sits in, while they admit the database driver to every
  package of the UI, so a statement written by hand would read around them. The raw SQL analyser
  reports, in a non-test file of a package under `ui/`, every use of a method that runs a
  statement, matched by its name and by how its parameters open, the driver's parameters for it.
  They are `Query`, `QueryRow`, `Exec` and `ExecParams` given a `context.Context` and a
  statement's text, `Prepare` given a context, a name and a statement's text, `SendBatch` and
  `ExecBatch` given the driver's batches, `CopyFrom` given the driver's table identifier or a reader
  and a statement, `CopyTo` given a writer and a statement, and `StartPipeline` returning the
  driver's pipeline. That covers the driver's connection, pool and transaction and its lower-level
  connection, `*pgconn.PgConn`. It reports the driver's own types, an interface such as a generated
  subsection's `DBTX`, and any other type alike, so a wrapper or an interface declared in the UI is
  no way around it, and a parameter spelled through an alias counts as the type it stands for. A call and a method
  value are reported alike. The generated statements sit under `db/`, outside its scope, so the UI's
  code reaches the database only through them. Three paths stay with review. One is a statement run
  through reflection. Another is one written to the wire through the raw network connection or the
  protocol frontend the lower-level connection hands out. The third is a method one of whose
  parameter types is a type parameter. Test files are exempt, since a statement a test runs is never
  served. The rule sits in its own analyser, run by the same program, and honours no suppression
  comment.
- **Every package the build of `./...` reaches is one `./...` lists, and the ban-proof script checks
  it.** The aggregator, the `go vet` analysers and the grant check of
  [ADR-0066](../data/0066-data-access-generated-from-sql.md) read only the packages `./...` lists,
  while a file importing a package by path builds it into a deployable whether or not `./...` lists
  it. The import lists cannot refuse such an import, because a deployable's list admits the
  deployable's own directory by prefix and no list has a `deny` key. So the non-test build graph of
  `./...`, which `go list -deps` reports, holds only three kinds of package. They are the standard
  library, the packages `./...` of this module lists, and packages of other modules the module cache
  holds. Any other package is refused at each import of it from a package `./...` lists, and at
  `go.mod` when only packages from the module cache import it. That covers a package under
  `testdata`, one in a directory whose name starts with `_` or a dot or that the `ignore` directive
  of `go.mod` names, one reached through a symbolic link, a nested module however the build reaches
  it, a module a `go.work` adds, and a module a `replace` points at a local directory, with no rule
  per kind. A test file may import such a package, because tests are outside the graph.
  - **It runs in each configuration code ships in, with no build tag.** The graph depends on the
    configuration, as a file built only without cgo, only on another operating system, or only
    without a tag shows. The images build with `CGO_ENABLED=0` for Linux and no tag, and the release
    builds the key-generation command with `CGO_ENABLED=0` for Linux and macOS on amd64 and arm64
    and no tag. The script holds that list of configurations as a copy of what the Dockerfiles and
    the release workflow build, and nothing checks the copy against them. A configuration added
    there is added to the script by hand. When the script proves the violation files it runs the
    check once more with their tag, in the first configuration, and reports the findings of every
    run.
  - **A module a `replace` points at a directory is refused wherever the directory sits.** `go list`
    names each package's module and its replacement, so a directory under the module cache does not
    pass for a download.
  - **Every non-test Go file a run compiles is one the gating lint and `go vet` runs read.** A
    package `./...` lists can still hold a file no lint reads. The aggregator and the analysers read
    the files `go list` selects with the build tags `.golangci.yaml` sets, for the operating system
    and architecture of the machine running them, with cgo as that machine has it. A file whose
    build constraint excludes one of those tags, such as `//go:build !integration`, one built only
    for another operating system or architecture, and one built only without cgo are each compiled
    into an image or a release binary and read by nothing. So every non-test Go file a run above
    compiles, in a package `./...` lists, must be one `go list` selects with the configuration's
    tags in the environment the script runs in, which in continuous integration is the lint job's
    own. Any other is refused at its package clause. The runs with no tag are compared with the
    configuration's tags, and the run with the violation files' tag with the configuration's tags
    and that tag, as the script's own lint run reads them. Assembly and other files that are not Go
    files are outside the rule.
  - **The gating `go vet` step sets exactly the tags the configuration sets, and the script checks
    it.** The comparison above is with the configuration's tags, so the vet run reads the same files
    only while its `-tags` value in the lint workflow holds the same tags. The script reads the
    workflow with the YAML parser it already reads the configuration with, finds the one step
    running `go vet` with `-vettool`, and refuses the workflow when that step's tags differ from the
    configuration's as a set, naming both. Its own `go vet` run takes the configuration's tags and
    the violation files' tag, rather than a list of its own. Deriving the step's tags from the
    configuration inside the workflow was rejected, because it needs a YAML tool the repository does
    not pin.
  - **A file built only for some of the configurations, or only without cgo, is refused rather than
    linted where it builds.** The lint runs in one configuration, and no file in the tree needs
    another. Linting in every configuration code ships in would run the aggregator and the analysers
    once per configuration, for files nobody has written. When one is needed, the lint also runs in
    the configuration that builds it, and the script compares each run with what the lint read
    there.
  - **It reads the build graph afresh on every run.** `go list` rereads `go.mod` and `go.work`, so no
    cached result outlives a change to them.
  - **It refuses every dependency under vendoring.** A vendored package's directory sits in the
    repository, and `./...` does not list it, so no lint reads it either.
  - **It runs `go list` in the environment of whoever runs it.** `GOWORK` and `GOFLAGS` shape the
    build graph, `GOMODCACHE` decides where the module cache sits, and `GOEXPERIMENT`, `GOAMD64` and
    `GOARM64` shape the implicit build tags. Continuous integration and the Dockerfiles set none of
    them.
- **A finding of a linter standing in for a control cannot be switched off anywhere.** Those linters
  are `depguard`, `errcheck`, `exhaustive` and `forbidigo`. A violation file proves a ban fires in
  that file, and says nothing about another line where the finding was switched off, so every proof
  stays green while the control stops being one. Under the aggregator a finding can be switched off
  at its line by a directive, by the linter's own ignore comment, or for whole paths by a
  configuration setting. So the ban-proof script refuses every directive the aggregator honours that
  can reach one of those linters, including a bare `//nolint`, one naming every linter, and one
  naming such a linter in any letter case. It refuses `exhaustive`'s own ignore comment and
  `forbidigo`'s permit comment, and every configuration setting that can exclude their findings,
  among them an exclusion rule naming no linter or naming such a linter, path exclusions, presets, a
  generated-file mode other than `disable`, not linting tests, and limits or new-issue modes that
  hide findings. Every other linter the repository runs is an ordinary linter, and the ordinary
  linters form one closed list, which [CLAUDE.md](../../../CLAUDE.md#static-analysis-and-formatting)
  names. A false positive of an ordinary linter may be suppressed by a directive naming only
  ordinary linters and giving its reason, or by an exclusion rule naming only ordinary linters. The
  script checks the list against the configuration and the violation files, so a linter newly
  enabled cannot be suppressed until it is listed. **No analyser carries this rule.** `nolintlint`
  is the tool an implementer would reach for and it does not do this. Its own description is that it
  reports ill-formed or insufficient directives, so a well-formed directive naming a linter and
  carrying an explanation silences a ban while `nolintlint` at its strictest settings reports
  nothing. The aggregator offers no option to stop honouring the directives.
- **Six checks are written here, because no tool offers them.** The rules against package-level
  state in a pure core above. The rule above against a route the UI registers other than through its
  recording mux. The check above that the build of `./...` reaches only packages
  `./...` lists and compiles only files the lint reads. The suppression check above, written to
  follow the aggregator's own reading of a directive so it refuses exactly what the aggregator would
  honour, and refusing the spellings the aggregator ignores today as well, so a later release
  honouring them cannot admit one silently. A package that must not compile, loaded with
  `golang.org/x/tools/go/packages` through one helper in the shared test support, which requires the
  exact type error rather than the presence of one. And the ban-proof script, which runs the
  analysers, the suppression check and the build-graph check against the checked-in files violating
  each ban and each list, requiring each expected finding and no other. The script also runs the
  project's `go vet` analysers, for
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)'s placement rules, the rules against
  package-level state and against a route registered around the UI's recording mux above, the rule
  [ADR-0078](./0078-configuration-layers-through-an-owned-library.md) sets against reading the
  environment outside a deployable's `main.go` and the rule
  [ADR-0047](../data/0047-schema-first-data-access.md) sets that every generated data-access
  function runs inside the transaction helper, against their violation files.

### What each tool's ordinary path does that other records forbid

| The ordinary path | What goes wrong | What catches it |
| --- | --- | --- |
| `default-signifies-exhaustive: true`, which treats a default branch as proof the switch is complete | [ADR-0042](./0042-implementation-stack.md) requires a deny-defaulting default branch on every verdict switch, so under that setting the check goes silent on precisely the code this project is required to write, while continuing to report on code that does not matter | The setting is written out as `false`. The fixture behind the catalogue's exhaustiveness row carries the mandated default branch, so the row cannot pass with the setting wrong |
| `check-blank` and `check-type-assertions`, which both default to `false` | Discarding an error explicitly is the ordinary way an unchecked error enters code, and it is the case the check skips unless asked | Both settings written out as on, with a checked-in file discarding an error that the ban-proof script requires to be reported |
| A `deny` list of packages a pure core may not import | The list is a guess about the future. Something added to the standard library later is admitted, and nothing says so | `list-mode: strict`, `allow` lists only, and no `deny` key written anywhere in the configuration. The ban-proof script refuses a list in any other mode and any `deny` key, each proven by a case in its own tests |
| `//nolint`, a linter's own ignore comment, or a configuration exclusion silencing a finding | A rule standing in for a control stops being a control wherever somebody found it inconvenient | The suppression check, written here because no analyser carries it. Each refused directive and ignore comment is proven by a checked-in violation file the ban-proof script requires the check to report, and each refused configuration setting by a case in the ban-proof script's own tests |

### Anything deliberately left open

Nothing in the import boundary. The boundary between database roles is a list per component naming
its data-access subsections, which [ADR-0066](../data/0066-data-access-generated-from-sql.md)'s
grant check tests against the grants.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| A standard-library package can be refused | `depguard` treats standard-library packages as addressable, so they can be named in `allow` and refused when absent from it |
| A rule is expressed as what is permitted | `list-mode: strict`, which refuses anything not named in `allow`. Verified by adding `os/exec`, which appears nowhere in the configuration, to a package governed by the core rule and watching it refused |
| A violation fails the build and says where | A non-zero exit, and a message naming the file, the line, the import and the rule it broke |
| A rule cannot be switched off quietly | The suppression check refuses every directive, ignore comment and configuration setting that can reach a linter standing in for a control, anywhere in the repository, and allows an ordinary linter's suppression only in a form naming it. Each refused directive is proven by a violation file, and each refused configuration setting by a case in the ban-proof script's own tests |
| Configuration is checked in and readable | One file holding every rule and every setting |
| Works on the Go version the project builds with | The aggregator is built with the current toolchain and rebuilds the analysers against it |
| Footprint | One command. The analysers are inside it rather than beside it, apart from the project's `go vet` analysers, for [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)'s placement rules, this record's rules against package-level state in a pure core and against a route registered around the UI's recording mux, [ADR-0078](./0078-configuration-layers-through-an-owned-library.md)'s rule against reading the environment outside a deployable's `main.go` and [ADR-0047](../data/0047-schema-first-data-access.md)'s rule that every generated data-access function runs inside the transaction helper, which run beside it |

One analyser here serves no kind [ADR-0042](./0042-implementation-stack.md) names. `forbidigo`
refuses a call to an identifier named in its configuration, and
[ADR-0070](./0070-unit-comparison-through-one-options-value.md),
[ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) and
[ADR-0076](./0076-metrics-emitted-through-client-golang.md) place such bans. A ban proven by a
checked-in violation file needs a message naming the file and the line, which it gives. Configured
with a pattern for `cmpopts.IgnoreUnexported` it reported the call's file and line, stayed silent on
the `cmp.AllowUnexported` call beside it, and with type analysis enabled reported an aliased import
of the same function as well.

### What the implementer would otherwise pay to discover

- **`check-blank` and `check-type-assertions` are both `false` by default.** A configuration that
  enables `errcheck` and stops there does not report `_ = f()`, which is the ordinary way an
  unchecked error enters code.
- **A violation file proving a ban is an ordinary Go file in a package the analysers load, where the
  ban applies.** It carries a `banproof` build tag that only the ban-proof script's run enables, so
  the gating lint stays clean while the script sees each ban fire, and it is named for what it
  proves. A test file proves a list over test files and a non-test file proves a list over non-test
  files, because a file one list matches says nothing about another. A file under `testdata` would
  not do, because the toolchain excludes that directory and the analysers then load nothing, which
  the ban-proof script reports as a ban that did not fire.
- **`depguard`'s allow entries are string prefixes, compared with one sorted neighbour.** An entry
  `os` also admits `os/signal`, so standard-library entries in the pure-core list end in `$`, which
  makes them exact. Each import is compared only with the entry sorted just before it, so an exact
  entry under a shorter entry for the same path stops that shorter entry admitting its other
  subpackages. `$gostd` admits the standard library through one entry per top-level name, and an
  allowed module whose path starts with `go.` sorts between `go` and `go/ast` and hides every `go/`
  package, so a list holding one also lists `go/`. The violation file for each list imports the
  package its sorted neighbour would wrongly admit.
- **`depguard`'s file patterns match absolute paths.** A pattern such as `**/core/**` also matches
  every file once the checkout sits under a parent directory with that name, so every project
  pattern starts with `${base-path}`. The aggregator's cache does not key on the resolved base path,
  so comparing a configuration across two checkouts needs the cache cleared first. `dir/**.go`
  matches files directly in `dir`, and `dir/**/*.go` does not.
- **The aggregator's defaults hide findings.** Files carrying a generated-code header are excluded
  unless `generated` is `disable`, and a second finding on the same line is dropped unless
  `uniq-by-line` is `false`. `golangci-lint config verify` refuses a misspelled key, which the
  aggregator would otherwise ignore.
- **Linting only what a change touched misses findings a change causes elsewhere.** An enum value
  added in one package leaves a switch incomplete in another, and a run limited to new issues, to
  changed files, or to the changed directory reported nothing where a run over the whole module
  reported the switch. So a path filter decides whether the lint job runs, and the job lints the
  whole module.
- **`nolintlint` does not ban a suppression and no setting makes it.** Configured to require an
  explanation, require a specific linter, and report unused directives, it reported nothing against
  a directive that silenced a ban, because that directive was well formed. An implementer reaching
  for it would ship a control that controls nothing with the build staying green.
- **`forbidigo` runs with `analyze-types: true` and `exclude-godoc-examples: false`.** The second
  setting's default skips the bans inside `Example` functions in test files. With it off the ban
  matches nothing at all, not even the direct call. With it on the ban also survives the two obvious
  evasions, an aliased import and the function held in a variable, both of which were reported. The
  checked-in violation file therefore uses the aliased form, because a ban proven against the harder
  case is proven against the easier one.
- **`default-signifies-exhaustive` is the difference between a live check and a silent one** on
  this project's code specifically, because of the deny-defaulting `default` branch
  [ADR-0042](./0042-implementation-stack.md) requires on every verdict switch.
- **A package path under `core` includes the test binary's generated main package.** `go vet` does
  not analyse that package, but the analysis test harness does, and its generated file carries no
  `.go` name. So the rule on declarations checks only files named `.go` that are not test files.
- **`allow`-list checking is transitively sound here only because the list is closed.** `depguard`
  inspects the imports written in each file and does not follow them. That is enough for the
  pure-core rule as written, because a core package may import only outside packages that meet the
  four conditions above together with everything they import, which reach nothing of this
  project's, and other core packages that the same rule governs. Every path therefore runs through
  something checked. It stops being enough the moment the allow list admits a package that fails
  the conditions or a project package the rule does not itself govern, and nothing detects that.
- **A package `./...` does not list is built in when a file imports it, and no check reads it.** A
  package at `propose/internal/testdata/w` called the sealed credential's statements inside the
  transaction helper, and `propose/internal/leak` imported it. The build, the aggregator, the
  `go vet` analysers and the grant check all passed with propose's list unchanged, so a role
  [ADR-0075](../data/0075-one-runtime-role-per-deployable.md) bars from the credential reached its
  statements. The same held, measured, for these:
  - a package at `propose/internal/_u`, one at `propose/internal/.d`, and one under
    `ui/browser/node_modules`, which `go.mod` ignores;
  - a symbolic link at `propose/internal/link` to a `testdata` directory;
  - a nested module at `propose/internal/sub`, reached through a local `replace`, through a
    `go.work` using it, or published under this module's path and fetched from a proxy;
  - a module with another path at `tools/other`, reached through a `go.work` alone;
  - a module replaced by a directory outside the repository.

  A pure core's list admits `core/` by prefix in the same way, so the soundness above also rests on
  the build-graph check.
- **A file whose build constraint keeps it from the lint's configuration is compiled into an image
  and read by nothing.** In `propose`, a file carrying `//go:build !integration`, `!gmail_live`,
  `!devloop`, `!cgo`, `!linux`, `!amd64`, `darwin`, `!integration || nothere` or a legacy
  `// +build !integration` line, a file named for `darwin`, `arm64` or `linux` on `arm64`, and a
  package whose only file is named for `darwin`, were each left out of the files `go list` selects
  with the configuration's tags for Linux on amd64 with cgo, which is what the lint job reads, while
  a build for a configuration code ships in compiled it. The script refuses each. A constraint that
  is a plain negation of a tag was already refused before this check, by the rule that only
  violation files may depend on the `banproof` tag, since that rule evaluates a constraint with
  every other tag set. The file names, the positive `darwin` constraint, the disjunction, the legacy
  line and the package are what this check adds. It passes a file carrying `!banproof`, which the
  gating lint reads because it sets no such tag, `!integration || devloop`, which the lint's tags
  make true, and `ignore`, `integration` or a name for `windows`, which no configuration code ships
  in compiles.

## Alternatives considered

### The import boundary, where there was a field

Four candidates, graded on the four-level scale used above. **4** means the candidate carries the
requirement natively. **3** means it is carried with bounded discipline or a named gotcha. **2**
means it is carried only against the tool's own grain. **1** means it cannot honestly satisfy it.

The first two rows were produced by writing both rules against a fixture laid out the way this
repository is laid out, with a violation file and a legitimate file for each rule, and reading what
each candidate reported. The suppression row was produced by attempting to suppress a confirmed
finding five different ways. The row on what a violation reports was read from the output of those
same runs. The remaining three rows, on configuration, on the Go version, and on footprint, were
read from each candidate's own documentation and from installing it.

One column is not a tool. The check written here does not exist, so its cells state what writing it
would have to achieve rather than what was observed, and that is why it carries no cell above the
ceiling anywhere. Its two lower grades are the two things a program this project maintains would
have to earn rather than inherit, which are a message a script can match and remaining correct as
the language moves.

| Requirement | depguard | go-arch-lint | A check written here | gomodguard |
| --- | --- | --- | --- | --- |
| Refuses a standard-library package the rule forbids | 4 | 1 | 4 | 1 |
| Expressed as what is permitted | 4 | 4 | 4 | 3 |
| A rule cannot be switched off quietly | 3 | 4 | 4 | 3 |
| A violation fails the build and says where | 4 | 4 | 3 | 4 |
| Configuration is checked in and readable | 4 | 4 | 3 | 4 |
| Works on the Go version the project builds with | 4 | 4 | 4 | 4 |
| Footprint | 4 | 3 | 4 | 4 |

**One requirement decided it and the rest are commentary.** A pure core's rule is almost entirely
about standard-library packages, because those are where reading a file, opening a socket and
running a command live. One candidate allows every standard-library import unconditionally, before
any permission check runs, which was confirmed by reading the code that does it and by watching a
core package import a networking package without being reported. That is disqualifying rather than
inconvenient, and it is disqualifying for the rule this record cares most about while the same
candidate handles the deployable rule correctly.

**Two candidates were the wrong category and are named so nobody proposes them again.** `importas`
enforces consistent import naming and `grouper` enforces how declarations are grouped. Neither has
any notion of permission. `gomodguard`, graded above, operates on whole external modules rather
than on this repository's own packages, so it answers which outside dependencies the project may
take, which is a real question and not this one.

**Where the chosen candidate stands alone.** Nowhere. It shares the ceiling on the deciding
requirement with the check written here, and loses to both remaining candidates on suppression.

### What the grid cannot show

| Candidate | What it would add to the repository | How a rule is written | What suppression exists |
| --- | --- | --- | --- |
| depguard | Nothing. It is inside the aggregator already being taken | A file glob and an allow list per rule | The aggregator's suppression comment, restricted separately |
| go-arch-lint | A second command in the pipeline | Named components with a dependency graph declared once | None at a line. Whole files and directories can be excluded in the shared configuration |
| A check written here | A program this project maintains | Whatever this project writes, over the list of packages a build reaches | None, because none would be written |

### The reading

**Whose worst cell is the best.** The check written here has the best worst cell and is not chosen,
which is the one place in this record where the grid and the decision point different ways. The
reason is in the next paragraph.

**Which strengths are guarded elsewhere and which guard something nothing else does.** The chosen
candidate's advantage is that it is already present, since the aggregator is taken for the
analysers that cannot run standalone on this Go version. Against that, the check written here would
have offered two things nothing else does, which are a walk through the whole set of packages a
build reaches rather than the imports written in one file, and no suppression mechanism at all
because none would exist. The first of those turns out not to be needed, because the allow list is
closed and every path runs through something checked. The second is real and is what this decision
gives up.

**What regretting each candidate would cost.** Leaving the chosen one costs a configuration block.
Leaving the check written here would cost a program and its tests. Leaving the architecture linter
would cost a configuration file and a pipeline step.

**Which strengths could be had without choosing the candidate, and which costs could be confined.**
The architecture linter keeps one advantage that is not detachable from it, which is naming
components once and declaring a dependency graph between them rather than repeating file globs. With
a list per component and per library the boundary is a graph of many components, so that advantage
is real, and it is worth reopening for the rules about this project's own packages. It will never be
worth reopening for the pure-core rule, because the standard-library gap is a property of how that
tool classifies imports rather than a setting.

### The candidates

**`depguard`, the pick.** The case for it is that it is the only candidate that can express the rule
this record cares most about. Standard-library packages are addressable to it, so they can be named
in an `allow` list and refused when absent from one, which was verified by adding a package that
appears nowhere in the configuration and watching it refused. It expresses the rule with no `deny`
key written anywhere, so the unbounded enumeration a forbidden list invites is not merely avoided
but unavailable. And it is already present, because the aggregator is taken for analysers that
cannot run standalone on current Go releases, so the marginal cost is a configuration block.

The case against it, which is the part worth not softening. Its allow-list behaviour lives in
`list-mode: strict`, which is not its default, so the denylist shape stays one configuration key
away and a later editor can reach it without the rule changing visibly, which is why the ban-proof
script refuses a list in any other mode and any `deny` key. Its upstream has been quiet since March
2025. Under the aggregator its findings are suppressible at the line, which is the whole reason
this record refuses the suppression comment wherever it could reach a control and writes a check to
enforce that. And its rules are file globs repeated per rule rather than named components, so a
boundary that grows into a graph of many components repeats itself in configuration where a
purpose-built tool would not.

**`go-arch-lint`.** The strongest rival on shape. Allow-list expression is its only mode, so the
denylist verb does not exist to reach for, and it names components once and declares a dependency
graph between them rather than repeating globs. It was released days before this decision. It is
rejected on a single fact that no setting changes: every standard-library import is allowed
unconditionally, before any permission check runs, which was confirmed by reading the code that
does it and by watching a core package import a networking package without being reported. Since
the pure-core rule is almost entirely about standard-library packages, that is disqualifying here
while leaving the tool perfectly good at the rule it does carry.

**A check written here.** Its case is real and it is the reason this was close. It would walk
whatever set the project decided rather than the imports written in one file, and it would have no
suppression mechanism because none would be written. What decides against it is not capability but
accumulation. This change already writes the suppression check, a compile-failure assertion and the
ban-proof script, and each owned mechanism is cheap alone while the set of them is a standing
maintenance surface with no upstream. Spending that budget on the one rule an existing tool already
expresses correctly is the wrong trade.

**`gomodguard`.** Named because a reader who finds it would reasonably wonder. It governs which
external modules a project may depend on at all, which is a real question this project may want
answered later. It has no notion of one package in this repository importing another, so it cannot
express any of the import rules here.

### The remaining questions, where the field reduced to one fact each

**Whether to run the analysers separately or under one command.** The expectation was that one
command buys consolidated configuration and costs frequent upgrades. What decided it is neither. The
exhaustiveness analyser's only tagged release is from 2023, and the dependency it pins no longer
builds against current Go releases, so installing it directly produces no binary. The same analyser
reports correctly when run through the aggregator, which pins a current version of that dependency
and is itself built with the current toolchain. The import-boundary analyser has the same shape of
problem. So the aggregator is not buying convenience, it is absorbing the staleness of several small
projects, which for a project whose enforcement rests on several of them is the whole product. Its
cost is a frequent release cadence that an update bot will surface, and a configuration format that
has changed across its own major versions.

**Which exhaustiveness analyser.** Two exist and they check different shapes. `exhaustive` checks
switches over enumerated constants, and `go-check-sumtype` checks switches over a closed set of
types declared by an annotation. They are therefore a choice about how a type is declared as much as
which analyser runs. The one that checks enumerated constants treats a default branch as not proving
completeness, and the one that checks closed type sets treats it as proving completeness. Measured
on the same code, the second reported nothing at all on a switch carrying a default branch with a
variant unhandled. Since [ADR-0042](./0042-implementation-stack.md) requires that default branch on
every verdict switch, that setting would silence the check on every switch this project is required
to write. The setting can be changed, and the remedy is one line, but the safer default belongs to
the analyser whose shape this project's types already have.

`go-check-sumtype` checks switches over a closed set of types, and this project declares none, so it
has no target here, which is why [ADR-0042](./0042-implementation-stack.md) names three linter kinds
rather than a fourth covering closed sets of types. It also carries a second hazard worth recording.
The `//sumtype:decl` annotation is what gives it anything to check, and removing that annotation
leaves it reporting nothing rather than complaining, so the analyser would sit idle without saying
so. No record in the set declares a closed set of types or sketches one. The one candidate structure
found while checking, an operation in a reorganization plan where one field belongs to a single kind
of operation, is not a sensitivity-carrying or verdict type, so that record's rules do not reach it,
and its representation is undecided.

**How to assert that a package does not compile.** No tool does this. Two mechanisms were built and
each was taken from failing to passing and back. One runs `go build` against a directory named
`testdata`, which the toolchain excludes from ordinary builds, and requires a non-zero exit. The
other loads the package with `golang.org/x/tools/go/packages` from inside an ordinary test and
requires type errors to be present. Both are about fifteen lines. The second is
chosen because it makes the assertion a test like any other rather than a separate step.

**How to prove a ban fires.** `analysistest`, from `golang.org/x/tools/go/analysis`, asserts
expected diagnostics from comments in a fixture file. It works on any analyser exposed as a library
value, including a third-party one, which is more than expected, but not on a compiled command or on
an analyser embedded inside `golangci-lint`. So the mechanism is a script running the aggregator
over the checked-in violation files and requiring each to be reported. It was built and taken from
failing to passing in both directions.

**How to refuse package-level state in a pure core.** The case for `gochecknoglobals` with
`reassign` is that both are off the shelf and run under the aggregator. `gochecknoglobals` reported
thirty-seven legitimate package-level variables outside pure cores, so it would need scoping to
pure-core files by an exclusion rule, and the ban-proof script refuses any exclusion rule naming a
linter that stands in for a control. It also admits exported error values, which a composition root
can overwrite, so `reassign` would run beside it over the whole repository, and neither refuses a
pure core overwriting its own error value. The operator chose on 2026-09-23 the rules in the
project's own `go vet` analysers, which need no carve-out and refuse that write too.

**How to refuse a package `./...` does not list.** Three mechanisms were weighed.

- **An import list** cannot carry it. Allow entries are prefixes, and the packages to refuse sit
  under the prefixes the lists admit.
- **A `go vet` analyser** reads each import as written and matches the path against the ways a
  package escapes `./...`. Those are a `testdata` element, a leading `_` or dot, the `ignore`
  entries, a nested `go.mod`, a local `replace` and a `go.work`. It was built and mutation-tested.
  Each review of it then found another way the build reached an unlisted package, a symbolic link
  and a nested module published under this module's path among them. An analyser sees one package's files, so it can
  only enumerate those ways, and the list is complete only until the next one is found.
- **The build-graph check in the ban-proof script** was chosen, because it states the rule itself.
  It compares what the build reaches with what `./...` lists, so it needs no list of ways. Measured
  against every case above, it refused each one and passed the tree as it stands, in about two
  seconds across the four configurations. It needs no dependency beyond the go command.

**How to refuse a route registered around the UI's recording mux, and where.** Three mechanisms
were weighed, over the UI and over the mediator, whose composition root mounts its two roots and its
probe routes on muxes with nothing recording them.

- **A `go vet` analyser over the UI** was chosen. Run over the whole module with its one exemption,
  the recording mux's own `Handle`, it reported nothing under `ui/`, so it refuses no legitimate
  registration there. It is about a hundred and thirty lines with its comments.
- **The same analyser over the mediator** was rejected. There it reported six registrations and all
  six are legitimate, the API root's generator registering each operation of the registry, and the
  composition root mounting `/api/` and `/mcp` and serving its three probe routes. The import lists
  already confine `net/http` in the mediator's code to three files, the two roots' generators and the
  composition root, so exempting those leaves the analyser refusing only a route registered in the
  MCP root's generator, a case no record names. The case left to review there, a route the
  composition root adds by hand, could be refused only by a list of the patterns the composition
  root may mount, which would copy `mediate/app` into the analyser and drift from it, or by
  exempting the functions that mount them, which lets a route added inside them through. The
  mediator's routes stay with review.
- **Moving the recording mux into a package of its own** was rejected. The compiler would then
  refuse a call on the mux beneath the recorder from package `api`, with no analyser. It needs a new
  directory, and it leaves a registration on a second mux built in package `api` unrefused, which the
  analyser refuses.

**How to refuse a file a build constraint keeps from the lint.** Three mechanisms were weighed.

- **Allowing a build constraint only over a list of known tags** refuses a negated tag, which the
  rule that only violation files may depend on the `banproof` tag already does for a plain
  negation. It is a second list beside the lint's own tags that drifts from them, and a file named
  for an operating system or an architecture, or built only without cgo, passes it untouched.
- **Running the lint and the analysers in every configuration code ships in** reads every such file,
  at the cost of one more aggregator run per configuration, and still misses a file whose
  constraint excludes a tag the lint sets. No file in the tree needs it.
- **Comparing the files each run of the build-graph check compiles with the files the lint's tags
  select** was chosen, because it states the rule itself and extends a check that already lists
  the build in every configuration. It adds two `go list` runs, measured at under a second
  together, and refused every constraint shape above that ships.

### The inter-component rules selected again at the regrouping into families

The regrouping of the shared libraries into families
([ADR-0050](./0050-shared-code-pure-or-narrow.md)) rewrote every list, which is the condition the
Consequences below set for selecting the tool for the inter-component rules again. The pure-core rule
was not part of it. The lists as they stood then load four requirements that order the field, each
the only mechanism holding a control.

| Requirement | Where the lists load it |
| --- | --- |
| A named standard-library package is refused, and a list can admit no standard-library package at all | The mediator's packages outside its two roots admit nothing under `net/http`, the UI's shipped code nothing under `crypto/` beyond the packages it uses, and the files of each root other than its generator no standard-library package ([ADR-0053](./0053-parity-by-construction.md), [ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)) |
| A rule covers single files inside a package | Each root's generator file takes a list of its own beside the rest of its package, the one file its protocol's library is admitted in ([ADR-0053](./0053-parity-by-construction.md), [ADR-0086](./0086-mcp-root-on-the-official-go-sdk.md)) |
| Test files and non-test files take different lists | The test tooling, rapid and go-cmp are refused to code that ships and admitted to tests ([ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)) |
| A list admits only what it names, and an empty one refuses | This record's decision |

| Candidate | Refuses a standard-library package | Covers single files | Splits test from non-test files | An empty list refuses |
| --- | --- | --- | --- | --- |
| `depguard` | Yes | Yes, by file glob | Yes | Yes, in `strict` mode, which the ban-proof script holds every list to |
| `go-arch-lint` | No. Every standard-library import is allowed before any permission check runs | No. Components are directories, and files are excluded only for the whole configuration | No, beyond that exclusion | Yes |
| `arch-go` | A named one, yes | No. Its unit is the package | No. It loads non-test packages only | No. An empty list of standard-library packages admits every one |
| A check written here | It would | It would | It would | It would |

`go-arch-lint` and `arch-go` each fail a requirement whose control nothing else holds, so a mediator
package importing `net/http`, or a test-only library reaching shipped code, would pass without a
finding. The check written here loses as it did above, on accumulation rather than capability.
`depguard` stays, for the inter-component rules as for the pure-core rule, and no list changed tool,
so no list was proven again for a new one. Its cost is the one named above: its rules repeat file
globs where a graph of components would declare each once.

## Consequences

- **What leaving these choices would cost.** A configuration file, in every case. The rules
  themselves are stated in the records that require them and survive any change of tool. The six
  checks written here are each small enough to rewrite in an afternoon.
- **What would re-argue this decision.** The aggregator's central argument is that it rebuilds
  analysers their own maintainers have not released against a current toolchain. If those projects
  resume releasing, that argument weakens to a consolidated configuration file, which is a much
  smaller claim.
- **[ADR-0042](./0042-implementation-stack.md) names three linter kinds**, and each has a tool
  here. A fourth analyser, `forbidigo`, is configured here too, and it is not one of those kinds.
  Those kinds are the enforcement that record's type conventions need, and `forbidigo` carries
  bans other records place on a named construction, which is a different job. Those records are
  [ADR-0070](./0070-unit-comparison-through-one-options-value.md),
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) and
  [ADR-0076](./0076-metrics-emitted-through-client-golang.md).
- **The allow list carries a property no tool checks.** Adding a package to it that the rule does
  not itself govern makes the pure-core rule unsound without any check failing. That is review
  discipline on the configuration and is dispositioned in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- **The lists now form a graph of many components**, the condition under which the architecture
  linter is worth reopening for the rules about this project's own packages. It is never worth
  reopening for the pure-core rule, for the reason the alternatives give. The condition is acted on
  the first time the lists are rewritten for a merge of deployables or a regrouping of libraries,
  since that change rewrites the inter-component rules anyway. That change selects the tool for the
  inter-component rules again, weighing real alternatives, and the pure-core rule stays on
  `depguard` whatever it finds. If the tool changes, every list rule is proven again by a violation
  file the new tool refuses. The regrouping into families acted on it and kept `depguard`, as
  [the selection at the regrouping](#the-inter-component-rules-selected-again-at-the-regrouping-into-families)
  states.
- **Assumptions about other components.** Continuous integration can install a pinned command and
  run it over the whole module whenever a change reaches Go code or the configuration. The packages
  a pure core is permitted to import reach nothing of this project's, which is what makes checking
  written imports enough.
