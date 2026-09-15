# 0068. The test substrate is a container started directly, not a container library

**Status:** Accepted

## Context

[ADR-0043](./0043-no-mocking.md) requires the database layer to test against real PostgreSQL,
containerized and ephemeral, and [ADR-0048](../data/0048-forward-only-migrations.md) requires the
full migration chain to apply from an empty database on every run. Neither names how the container
is started.

The field offers libraries that manage container lifecycle from inside the test process, and the
alternative of starting the container with the ordinary container command and pointing the tests
at it. What those libraries mainly sell is convenience this project's shape does not need, because
every run gets a fresh container, nothing is reused between runs, and the chain that has to run is
one command and a few statements.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| Real PostgreSQL, ephemeral | An actual server per run, not a simulation and not a shared instance | [ADR-0043](./0043-no-mocking.md) |
| The schema's extensions installable | Case-insensitive text, trigram search and vectors, because the chain applies whole after the bootstrap creates them, and its first migration records them | [ADR-0016](../data/0016-schema.md), [ADR-0048](../data/0048-forward-only-migrations.md) |
| Works where the developer works | Reachable from both continuous integration and a workstation, whatever container access each has | [ADR-0051](./0051-environment-contract.md)'s posture, applied to the test harness |
| Test-path footprint | What the choice adds to the dependency tree the tests carry | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture, applied to a layer outside the runtime |

**What orders the field is the extensions requirement**, because it is the only one any candidate
fails outright, and failing it is terminal rather than inconvenient. **This record carries no
scored table**, because after that requirement removes one candidate the remainder turns on two
facts that prose carries better than cells would, and grading the rest would mean inventing
comparisons nothing measured.

This decision is treated at reduced depth deliberately. It is the cheapest reversal in the data
layer and nothing above the test harness depends on it.

## Decision

- **Tests start PostgreSQL with the ordinary container command and connect to it. No container
  library is taken.** The chain applies from empty, which is one command and a few statements.
- **One container serves a whole test run, and every test package gets its own database.** `go test`
  runs each package's test binary as a separate process in parallel, so a container started by each
  package collides on the fixed host port. A test-support program starts the one container, applies
  the superuser bootstrap and the chain from empty into a template database, runs the test command
  with the connection details in its environment, and removes the container. Each integration test
  package creates its own database from the template, so packages running at the same time never
  share one. The program fails a run in which no test package created a database, and an integration
  test package fails rather than skips when it is started without the program, so a job that leaves
  out either one cannot pass. Where the program and the integration tests sit is
  [CLAUDE.md](../../../CLAUDE.md#tests)'s.
- **`embedded-postgres` is excluded because it cannot apply this schema's chain.**
  [ADR-0016](../data/0016-schema.md) declares a vector column and
  [ADR-0048](../data/0048-forward-only-migrations.md) creates the extensions the schema needs before
  the chain and records them in its first migration, so a substrate lacking that extension fails
  from the first run rather than at some later one.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Real PostgreSQL, ephemeral | One container started and removed per run, shared by every test package through a template database |
| The schema's extensions installable | An image carrying them, which is a choice of image rather than of harness |
| Works where the developer works | A fixed host port, and nothing else where the container daemon is local |
| Test-path footprint | Nothing. The container is started by the tool already installed to talk to a daemon |

### Implementation notes

- **Where the container daemon is remote, reaching the database needs a route to the container's
  published port and not only to the daemon's own interface.** Forwarding the daemon's interface
  makes the container start and says nothing about whether the database inside it can be reached.
  This applies equally to a library and to the ordinary command. It is the reason a fixed host port
  is worth choosing over a random one, because a fixed port can be paired with a single standing
  forward where a random one cannot.

## Alternatives considered

- **A container library managing lifecycle from the test process**, meaning `testcontainers-go` or
  `ory/dockertest`. The case for it is genuine. It is the field's default, it handles readiness
  waiting and cleanup, and it removes a small amount of scripting. Rejected on its test-path
  dependency tree, which for the more widely used of the two pulls in process-inspection and
  telemetry packages this project has no use for, and on the fit between its conveniences, meaning
  a reaper process and curated images, and a posture where every run gets a fresh container. One
  thing that is not a reason to reject it, stated because it looks like one. Its client cannot
  address a container daemon over a secure shell. A forwarded port routes around that, and the
  obstacle that remains afterwards hits the ordinary command identically.
- **`embedded-postgres`.** The case for it is no container runtime at all and the fastest start.
  Rejected by the extensions requirement. Asked in a public issue, a maintainer replied that they
  have no current plans to add the vector extension and that the capability would have to come from
  the upstream project supplying their binaries. That is a hedge rather than a refusal and is
  recorded at that strength. What makes it decisive is not the strength of the hedge but that the
  dependency sits upstream of the people who would have to act on it.
- **A database for each test from the prepared template**, rather than one for each test package.
  The case for it is speed at high test counts. Not chosen, because it is an optimisation over the
  per-package databases the Decision already creates rather than an alternative to them, and there
  is no test count to optimise yet.

## Consequences

- **This is the cheapest decision in the data layer to reverse.** Nothing above the test harness
  depends on it, and the migration chain and schema are unchanged under any substrate.
- **One reason recorded above expires.** A workstation whose only container daemon is remote is a
  fact about one machine rather than about the project, and continuous integration is unaffected
  either way. If every daemon in use becomes local, or remote daemons' published ports become
  directly reachable, the forwarding cost falls away from both approaches equally and a library
  becomes a reasonable choice again. The footprint argument survives that change and the
  workstation argument does not.
- **Assumptions about other components.** Continuous integration can run a container and reach it.
  The extensions [ADR-0016](../data/0016-schema.md)'s schema declares exist in whatever image is
  used, which the bootstrap then creates and the chain's first migration records.
- One new control, that an integration run which reaches no database cannot pass, catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md). Otherwise this record decides how an existing
  obligation of [ADR-0043](./0043-no-mocking.md) is met.
