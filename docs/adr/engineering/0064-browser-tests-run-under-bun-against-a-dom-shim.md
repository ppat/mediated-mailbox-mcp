# 0064. The browser's tests run under bun against a DOM shim, on fixture responses recorded from the real server

**Status:** Accepted ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

[TESTING.md](../../../TESTING.md) fixes the test kinds and, before this record, gave the UI's
rendering layer one row, example-based tests against fixture responses carrying marker text,
asserting every marker arrives as text. [ADR-0043](./0043-no-mocking.md) forbids mocks in any form,
[ADR-0044](./0044-synthetic-fixtures-marker-text.md) puts markup-shaped markers in fixture metadata,
[ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) demands that a verification-proving test
never retires, that an automatable control's tests be seen to go red, that a control's test carry
its expected value independently of the code under test, and that a lint ban standing in for a
control be proven by a violation file. Before this record,
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md) keyed seven controls to the UI. The toolchain that
proves them was open. The operator's weights, set on 2026-09-10, were that a test which both the
server and the browser could carry belongs in the server, that the UI may need no property-based
test, that validation in the server is necessary whatever the browser does, and that nothing is
built today that is not needed today.

The research of 2026-09-10 found that of those seven controls, five are server behaviors proven
in Go against real Postgres, one, inert rendering, is provable only where rendering happens, and
one, the content security policy, splits into three assertions Go can prove and one act only a
browser performs. Where each is proven under this decision, with the deciding record named by
the catalogue's row:

| Control | Proven where | Instrument |
| --- | --- | --- |
| The database grant bounds the UI's writes | Postgres | a Go integration test against the real database |
| Message-derived text renders inert | the browser's rendering layer | `bun test` against the shim in the required assertion form, and once in a real browser as a drill |
| The registry refuses what it does not declare | Go | an HTTP test against the real server, plus the build check |
| Every lens is per account | Go | an HTTP test against the real server |
| The content security policy holds | Go for the header, the bundle, and the fonts, a browser for the block | three Go tests, and one browser drill |
| A decision needs its token and its declared identity | Go | an HTTP test constructing the harder case, the cookie present and the token absent |
| A decision writes all its rows or none | Go against real Postgres | example tests with a fault injected between the writes |

The research also found that the DOM shims carry no reported defect on the text-node path
every framework's default binding uses, that the two JavaScript test tools able to drive a real
browser do not run under bun and would bring Node and, for one of them, a second bundler into the
repository, and that a Go test can drive a browser without Node. The spike of
[ADR-0063](./0063-browser-app-is-preact-with-signals.md) then ran the inert-rendering test under
the shim and under a real browser and the two agreed on every assertion, and its mutation went
red under both. The operator ruled on the policy row's disposition and accepted this decision on
2026-09-10.

## Decision

- **The browser's tests are example-based and run under `bun test` with a DOM shim registered
  before any test module loads.** The shim is `happy-dom`, the one bun documents, registered at
  a URL with a real origin because the router builds URLs against the location's origin. No
  second JavaScript runtime and no browser sit in the permanent gate.
- **The inert-rendering assertion has a required form.** The marker's full literal text, its
  angle brackets included, is present in the surface's text content, no element was created from
  it, and the surface's element count matches the component's own structure. The form that
  merely checks the marker's inner text is present passes whether or not the value was parsed as
  markup, because text content concatenates the text inside a script element too, and is
  forbidden. The test runs against every surface a message-derived field reaches. Where the
  property under test is that no component re-renders, the assertion is a render counter, which
  is exact and runs in the shim and in a browser alike.
- **Fixture responses are recorded from the real server.** A Go test runs the UI's server
  against the fixture database and writes each response the browser tests need to a JSON file
  beside them. CI regenerates and diffs, so a fixture that drifts from what the server produces
  fails the build. The browser tests receive fixtures through a fetch function passed as a
  parameter, [ADR-0040](./0040-pure-core-decisions-as-values.md)'s rule that nothing is ambient,
  and nothing intercepts the network. Marker text enters the fixtures through the fixture
  database, where ADR-0044 already places it.
- **A recorded fixture is data, and not a mock, on three grounds.** It substitutes no behavior,
  since the function returns a value and nothing about the server's conduct is asserted. Its
  shape is not a belief, since it is a recording typed by the contract of
  [ADR-0065](./0065-contract-built-from-registry-consumed-as-generated-types.md). And the belief
  a mock would leave untested, that the real server produces that shape, is proven where the
  recording is made. This is the discipline ADR-0043 applies to the provider fake and its
  contract suite, one level cheaper, because a recording needs no suite of its own.
- **Three lint rules keep that structural.** No type assertion and no `any` inside a fixture
  module, so a fixture cannot drift from the contract silently. None of the runner's own mock,
  spy, or stub functions anywhere, since the runner ships them and only lint keeps them out. And
  the raw-markup escape hatch nowhere, the rule ADR-0063 states. Each ban is proven by a
  checked-in violation file, as ADR-0046 requires. The named request-interception libraries,
  `msw`, `nock`, `fetch-mock`, `miragejs`, `sinon`, and their kind, are excluded by ADR-0043's
  first rule and are listed here so nobody argues one in.
- **The inert-rendering mutation swaps one field's text render for the escape hatch.** Under
  ADR-0046's harness the demonstration must redden the dedicated inert-rendering test and not
  only screen-level tests, or it has found a vacuous test.
- **The content security policy's proof is split, by the operator's ruling.** Three assertions
  are permanent Go tests. The policy header is present and exact, name and value, on every
  response including the entry document, compared against the policy written out literally in
  the test as ADR-0046 requires. The entry template and the built bundle contain no inline
  script and no reference to another origin, where the bundle's scan carries a short allow list
  of absolute strings that cannot become a request, such as XML namespace identifiers, each
  certified by a person. The stylesheet references fonts on the UI's own origin only. The fourth
  assertion, that a browser blocks an inline script and an external fetch under the policy, is a
  drill in ADR-0046's sense, observed in a real browser when the UI's tests first land and again
  after any change to the policy, its proof holding for its date. **A person performs it**, against
  a checked-in page carrying an inline script and a script fetching another origin, served by the
  UI's own handler, with the browser's developer console open. No tool is taken, because every one
  considered proves the same thing and leaves no better evidence, for something run a handful of
  times over the project's life. **The dated note records the browser's own refusal message
  verbatim**, rather than a summary of it, since a note saying the block was observed is a claim a
  later reader cannot check.
- **No property-based test exists in the browser.** The four decision write paths are covered by
  the example tests with fault injection that [ADR-0060](./0060-no-code-in-the-database.md)
  requires, and no browser rule needs generated inputs.
- **The browser tests run in the gating suite** on every change under the UI's directory, through
  [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)'s allow-list path filters,
  and the recorded fixtures are regenerated in the same job that runs the Go tests they come from.

## Alternatives considered

The shapes weighed, with the chosen one in the first row:

| Shape | Tools | Runtimes in CI | Browser in the permanent gate | Second bundler | Proves the policy's block |
| --- | --- | --- | --- | --- | --- |
| `bun test` with a DOM shim | bun, `happy-dom` | bun | no | no | as a drill |
| The shim plus Playwright for one test | bun, `happy-dom`, Node, Playwright, a browser | bun and Node | yes | no | continuously |
| The shim plus a Go-driven browser | bun, `happy-dom`, a Go module, a browser | bun and Go | yes | no | continuously |
| Playwright's component testing for everything | Node, Playwright, a browser, bun as the dev server | Node and bun | yes | no | continuously |
| Vitest in browser mode | Node, Vite, Vitest, a browser | Node | yes | yes | continuously |

- **Vitest in browser mode, driving a real browser for every component test.** Its case was
  deleting the shim question rather than answering it, with official renderers for the major
  frameworks. Rejected because it requires Node and Vite, so the test path would transform
  through a second bundler while bun ships, a braid the un-braided pillar counts against, and
  because its maintainers state it does not support the bun runtime.
- **Playwright's component testing for every component test.** Its case was one tool in a real
  browser, framework-agnostic, driving the project's own dev server with no second bundler.
  Rejected because it requires Node, costs a story file per component, and makes every test a
  browser test for assertions that are text-content reads. It is the only shape that would satisfy
  Lit's own advice against shims, and Lit was not chosen.
- **Playwright for the one browser-observed assertion only.** Its case was familiarity and the
  richest browser-test API. Rejected because it puts a second runtime in the repository for one
  test of a few dozen lines.
- **Driving a browser from a Go test for the policy's block, keeping its proof continuous.** Its
  case was the un-braided arrangement, the policy test living beside the Go integration tests
  whose server and database it needs, with no Node. The operator chose the drill instead,
  because the policy is a constant that changes rarely, so continuous proof buys little against
  the standing cost of a browser binary in CI. This route is the one to take if continuous proof
  is ever wanted.
- **Hand-written fixture modules typed by the contract.** Its case was the same three grounds with
  less machinery. Not chosen as the target form because the third ground then rests on an argument
  rather than on a recording, and the recording makes it structural, the operator's stated
  preference. Hand-written fixtures are acceptable until the server they would be recorded from
  exists, and the spike of ADR-0063 used that form, a typed module from which the fixture JSON was
  emitted.

## Consequences

- TESTING.md's UI row cites this record beside ADR-0044 and ADR-0056, its "When tests run"
  section places the browser tests, and its "Deliberately absent" list names the runner's mocking
  functions.
- The request-token row of the catalogue has a clause a browser cannot construct, a page on
  another origin sending the session cookie, which `SameSite=Strict` prevents. Go proves the
  harder case, a request carrying the cookie without a matching token, so no browser is needed
  there. That asymmetry, the browser as a mitigation the server can test past versus the browser
  as the mechanism itself, is what separates the token row from the policy row.
- The shim's fidelity on the text-node path is a drill, the inert-rendering test run once in a
  real browser when the UI's tests first land. If the two ever disagree, the one test moves to
  the browser and nothing else changes.
- The stream client's transport cannot run under the test runner, because bun's runtime lacks
  `EventSource` (ADR-0063), so the transport is exercised in a browser at the unit that builds it
  and the handler is what the shim tests drive.
- Accessibility basics on tables, menus, and live regions, requirement 12 of
  [docs/UI.md](../../UI.md#16-framework-requirements), need no browser. The structural rules run in
  the shim if run at all, and the one rule that needs rendering, color contrast, is the palette
  validator's job under
  [ADR-0059](../operability/0059-two-palettes-derived-in-oklch-and-checked-for-contrast.md). No
  document makes accessibility a control, so it is not owed a permanent test.
- The recording step needs the UI's Go server and an ephemeral Postgres in the same job as the
  browser tests.
- Assumptions about other components: the UI's Go server and the fixture database exist before the
  browser tests are written, which is the order the M3 unit in [ROADMAP.md](../../../ROADMAP.md)
  states. The framework's default binding creates text nodes and never routes a value through the
  HTML parser (ADR-0063). The policy header is a constant in the Go handler
  ([ADR-0062](../operability/0062-ui-content-security-policy.md)).
- The fixture lint rules, the ban on the runner's mocking functions, and the fixture drift check
  are controls, and the escape-hatch ban rides ADR-0063's row. Their violation injections are
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), where the policy row is split
  into its permanent and its drill halves.
