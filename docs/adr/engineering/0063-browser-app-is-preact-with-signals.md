# 0063. The browser app is Preact, with signals carrying data state and a generated URL grammar

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

[ADR-0021](../mutation/0021-approval-surface.md) fixes the UI's shape and
[ADR-0042](./0042-implementation-stack.md) fixes TypeScript in the browser, bundled by bun and
shipped as static files, and left the framework open until the work that needed it approached.
[docs/UI.md](../../UI.md#16-framework-requirements) states the twelve requirements a candidate is
judged against, numbered there, and this record cites them by those numbers. It also states the
spike that proves four of them on the plan reviewer's skeleton. By the time the design settled, most
of what a framework ecosystem provides was already designed out. Tables are plain and
server-paginated, charts are hand-drawn SVG, one row component renders every message-derived row,
and every zoom step is a server aggregate. What remained to choose on was routing with the query
string as state, a small data cache keyed by URL, how a streamed object replacement reaches the DOM,
how little the framework drags in, and how well coding agents write its idioms without reaching for
the ecosystem's defaults.

The operator set the weights on 2026-09-10. Fluency for coding agents is paramount without being the
deciding factor. Evolvability, growth, and the ability to add capabilities as the product evolves
rank above it, together with what the design itself requires. Decisions rest on browser development
as it stands in 2026, not as it stood in 2023. The governing lenses are simple made easy, Pareto,
and asymmetric payoff, with the un-braided pillar critical, and no option that creates more work or
removes optionality. On the bundler the operator's words were that the bun clause of ADR-0042 "is
probably the easiest to reverse if needed," so building with bun alone weighs as a tiebreaker and
not as a gate.

The candidates were researched and measured on 2026-09-10. Each hello-world was built and served
under the exact policy of [ADR-0062](../operability/0062-ui-content-security-policy.md) in a
headless browser, and the plan reviewer's skeleton was then built on the chosen candidate against
fixture responses carrying markup marker text. That skeleton met the four requirements the spike
proves, escaping by default (1), the query string as routing state (2), streamed updates without a
full re-render (5), and contract-generated types (10), under a DOM shim and under a real browser,
with the two agreeing on every assertion, element counts and attribute values included, and its
mutation went red under both. The operator accepted this decision on 2026-09-10.

## Decision

- **The browser app is written in Preact, with `@preact/signals` and `preact-iso`.** Its whole
  production dependency set is those three packages and what they depend on. It is built by bun
  ([ADR-0042](./0042-implementation-stack.md)) with nothing added, no compiler plugin and no
  second bundler.
- **Every build passes `process.env.NODE_ENV="production"` as a define.** Bun's browser target
  substitutes that expression with the literal `"development"` on every build, its production
  flag included, so any dependency that carries a development build ships it unless the define is
  passed. Preact carries none, and the define is passed anyway so that nothing added later can
  ship one.
- **State follows [docs/UI.md](../../UI.md#19-building-it)'s model with one mechanism each.** Data
  state and the objects the stream replaces are signals, passed into JSX rather than read, so a
  replaced object updates only the nodes that read it and no component function re-runs. State a
  single component owns, an open menu, the row cursor, the typed restatement, is hooks. Reading a
  signal's `.value` inside a rendering position re-renders the component and is forbidden by a lint
  rule, because the two forms differ by one property access and only one of them holds the rule. The
  lint is syntactic and over-broad, since it cannot tell a signal from a contract field named
  `value`, so it fires on both. A read of such a field in a rendering position carries a per-line
  suppression, with the field named alongside it, and a live surface also asserts zero component
  re-renders with a render counter, the exact instrument, as
  [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) states.
- **No signal may be reachable from a route's props.** The router memoizes a route on a
  serialization of every prop it was given, and a signal serializes to its value, so a signal in
  a route prop re-renders the whole route subtree on every write with no warning. Dependencies
  reach screens through a context, and a route receives only stable, cheap-to-serialize props.
  The render counter of a live surface is what catches a breach.
- **Routing is `preact-iso`, and the URL grammar of
  [docs/UI.md](../../UI.md#5-information-architecture-and-the-url) lives in one module.** A dataset
  descriptor table generated from the contract of
  [ADR-0065](./0065-contract-built-from-registry-consumed-as-generated-types.md), a parser from the
  query string to a typed view state, a canonicalizer that fills a dataset's defaults, a serializer
  every link builds its URL through, which restores the grammar's literal comma and exclamation mark
  after the platform percent-encodes them because the canonicalizer compares strings, and one effect
  at the route boundary that replaces a non-canonical URL. That effect follows the location's full
  URL, because the router's own route-change hook fires on path changes only and every navigation
  below a screen change is a query-string change. The parser, canonicalizer, and serializer are pure
  functions of a string and the table, in [ADR-0040](./0040-pure-core-decisions-as-values.md)'s
  shape, and the effect is the only shell.
- **The data cache is the UI's own module**, keyed by URL, expiring after a few seconds, deduping
  in-flight requests. No query library.
- **The stream client is one module over `EventSource`**, the only module that depends on
  [ADR-0058](../operability/0058-live-surfaces-stream-over-server-sent-events.md). It is split
  into a transport, the one place `EventSource` is opened, and a handler that replaces the
  matching object's signal, because bun's runtime declares `EventSource` in its types and does
  not provide it, so only the handler can run under the test runner and the transport is proven
  in a browser.
- **The raw-markup escape hatch, `dangerouslySetInnerHTML`, is forbidden repository-wide by lint**,
  with the DOM's own hatches beside it. The same identifier is what the inert-rendering mutation
  demonstration of ADR-0064 swaps in, so one list serves both. The bans are expressed as rules
  forbidding a named construction by its shape, carried by the tools
  [ADR-0072](./0072-browser-bans-under-oxlint-and-ast-grep.md) chooses, which need no second
  runtime, and each ban is proven by a checked-in file that violates it, as
  [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) requires.
- **Direct dependencies are enumerated in a roster file beside the package manifest**, one line
  each with the reason it is there, and CI fails when the roster and the manifest disagree. The
  check is the same shape as [ADR-0053](./0053-parity-by-construction.md)'s generator monopoly and
  [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)'s import lint, a structural
  check on an explicit list rather than a count.
- **The compatibility layer `preact/compat` is unused.** Nothing the design needs is a React-shaped
  library. Adopting one later re-argues this record.

How the decision meets each of the twelve requirements of
[docs/UI.md](../../UI.md#16-framework-requirements):

| Requirement | Met by |
| --- | --- |
| 1, escapes text by default | Preact's text binding creates text nodes. The raw-markup hatch is banned by lint and is the mutation the inert-rendering demonstration swaps in |
| 2, the query string as routing state | `preact-iso` matches paths. The generated descriptor table, the parser, the canonicalizer, the serializer, and the replace effect on the full URL carry the grammar |
| 3, a component model for the plan reviewer | Components with props, a context for dependencies, nested routes for the panel over the list, and one message-row component |
| 4, plain server-paginated tables | Plain table elements over one page of rows. No grid library |
| 5, streamed updates without a full re-render | A signal passed into JSX binds the node. One event replaces one object's signal. A render counter asserts that no component re-ran |
| 6, theming through CSS custom properties | One stylesheet of custom properties, the OS preference read from the browser, the override stored per browser. No runtime style injection, so the policy's `style-src 'self'` holds |
| 7, keyboard navigation and focus | Hand-written on the components. Nothing in this decision provides it or stands in its way |
| 8, charts as inline SVG | Hand-drawn SVG with geometry set as attributes. No charting dependency |
| 9, a small tree, strict TypeScript, bundled by bun, static files | Three direct dependencies enumerated in the roster, strict mode with no type assertions, `bun build` with the production define, the output embedded in the Go binary |
| 10, types generated from the contract | The types and the descriptor table of [ADR-0065](./0065-contract-built-from-registry-consumed-as-generated-types.md), regenerated in CI with drift failing the build |
| 11, fluency for coding agents | React's component idioms, with agent-readable documentation published by the project. The two rules an agent gets wrong, the signal read and the route prop, are enforced by lint and the render counter rather than remembered |
| 12, accessibility basics | Hand-written ARIA in the document's own tree, with no shadow root in the way of identifier references |

## Alternatives considered

Each candidate was graded against the twelve requirements, against the long-term factors the
stack research applied to languages (platform trajectory, dependency longevity, guarantee
durability under years of edits, maintenance burden on one operator, completion risk, review
burden after open-sourcing, and the agent-tooling trajectory), and on measurements taken on
2026-09-10.

The first table scores every candidate on the twelve requirements, on the research's scale. 4
means the framework carries the requirement with a built-in mechanism, 3 means one small
maintained package or a few lines of the builder's code carry it, 2 means it is carried with a
named gotcha, and 1 means the builder writes and maintains the mechanism. The grades are the
research's of 2026-09-10, with the server-rendered column graded by the designing session on the
same scale because the research treated it in prose. The chosen candidate is the first column.

| Requirement | Preact | Solid | React | Svelte 5 | Vue | Lit | No framework | Go templates with htmx |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1, escapes text by default | 4 | 4 | 4 | 4 | 4 | 4 | 3 | 4 |
| 2, the query string as routing state | 3 | 4 | 4 | 2 | 4 | 1 | 1 | 4 |
| 3, a component model for the plan reviewer | 4 | 4 | 4 | 4 | 4 | 2 | 1 | 1 |
| 4, plain server-paginated tables | 4 | 4 | 4 | 4 | 4 | 3 | 3 | 4 |
| 5, streamed updates without a full re-render | 4 | 4 | 3 | 4 | 4 | 3 | 3 | 3 |
| 6, theming through CSS custom properties | 4 | 4 | 4 | 3 | 3 | 2 | 4 | 4 |
| 7, keyboard navigation and focus | 3 | 3 | 3 | 3 | 3 | 2 | 3 | 1 |
| 8, charts as inline SVG | 4 | 4 | 4 | 4 | 4 | 2 | 4 | 4 |
| 9, a small tree, strict TypeScript, bun, static files | 4 | 2 | 4 | 2 | 3 | 4 | 4 | 1 |
| 10, types generated from the contract | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 |
| 11, fluency for coding agents | 4 | 2 | 4 | 3 | 3 | 2 | 2 | 2 |
| 12, accessibility basics | 3 | 3 | 3 | 3 | 3 | 1 | 3 | 3 |

On the requirements alone, Preact, React, and Vue have no cell below 3, Solid and Svelte each carry
two cells at 2, Solid on its build and its corpus and Svelte on its router and its build, and Lit,
no framework, and the server-rendered shape each carry a 1. The requirements sort the field into
those three tiers and no further. The second table, with the measured facts of 2026-09-10 and the
chosen candidate in the first row, is what separates the top tier and what puts Solid back among the
contenders:

| Candidate | Streamed update | Footprint and build | Maintenance | Cost of leaving | Fluency for coding agents |
| --- | --- | --- | --- | --- | --- |
| Preact | one mechanism, a signal passed into JSX | 5 packages, 8.5 KB, bun alone | four maintainers within a factor of two, no major since 2019, the next in release candidate | an alias to React for components reading no signal, a rewrite of the data-state layer | React's idioms, a fifth of React's corpus, agent-readable documentation published |
| Solid | one mechanism from the ordinary read | 5 packages with its native router, more with the typed adapter its requirement 2 score assumes, 6.9 KB, an owned plugin over Babel | one maintainer at 44 to 1, 2.0 in release candidate as a package restructure | a rewrite | a fortieth of React's corpus, the largest agent-readable documentation of the set |
| React | two mechanisms, a cache and a subscription | 13 packages with the typed router or 5 with the plain one, 66 KB, bun alone with the production define | Meta, one major since 2023 | a rewrite, or an alias to Preact | the largest corpus, whose defaults pull toward what the records forbid |
| Svelte 5 | one mechanism | 21 packages with 2 at runtime, 19.5 KB, an owned plugin over its compiler | a team with employer backing, two majors since 2023 and a third signalled | a rewrite | a thirtieth of React's corpus |
| Vue | property-level tracking with a subtree redraw | 26 to 62 packages, 25.7 KB, an owned plugin over its compiler | one maintainer at 10 to 1, the successor model in release candidate | a rewrite | a twelfth of React's corpus |
| Lit | template parts | 7 packages, 7.8 KB, bun alone | Google | a rewrite | a twenty-fifth of React's corpus, no agent-readable documentation |
| No framework | own code | 0 packages, 1.1 KB, bun alone | none | none, but the owned machinery stays | no idiom to be fluent in |
| Go templates with htmx | a fragment swap per event | none in the browser, no bundler | a small team | four records rewritten | a small attribute vocabulary |

Escaping by default did not separate them. Every candidate rendered the markup markers as text under
the policy, and three, Solid, Preact, and Svelte, shipped client-side escaping regressions since
2025, which is why the inert-rendering test exists and not a reason to choose. Read by the
operator's lenses, Preact is the candidate with no requirement below 3 that also meets streamed
updates with one mechanism, builds with nothing added at a fraction of React's footprint, and has
the cheapest exit. React ties it on the requirements and loses on footprint and on the braided
state. Vue ties it on the requirements and loses on footprint, on the owned build plugin, and on a
rendering model in transition. Solid matches it on structure and footprint and loses on the two
requirements it scores 2 on, on maintenance, and on the cost of leaving.

- **Solid.** The case for it is the best structural fit in the field. A store write reaches exactly
  the DOM nodes reading the changed fields from the ordinary read, with no rule to learn, its router
  reached a stable release with zero dependencies, and its footprint matched Preact's. Rejected
  because its maintenance rests on one person, its 2.0 rewrites every effect on an unannounced date,
  its compiler needs a Babel toolchain pinned a major behind that the repository would own as a bun
  plugin (the Vite route depends on the same Babel and adds a second bundler and Node on top), its
  escaping regression's proof of concept is this project's threat model, its corpus is the smallest
  of the component frameworks, and the exit from it is a rewrite.
- **React.** The case for it is the strongest governance, the cleanest rendering-escape record in
  the set, the largest corpus for coding agents, and two mature library-mode routers. Rejected
  because its floor is eight times Preact's for the same idioms, its data state needs two braided
  mechanisms where signals need one, its production build falls into bun's development-build trap
  without the define, and its ecosystem gravity toward runtime style injection, global stores, and
  meta-framework servers is a standing pull on agent-written code that a roster and a browser test
  must hold against. React stays the fallback this decision keeps cheap. Preact's component API is
  React's, so the components that do not read signals survive an alias in the bundler unchanged, and
  the cost of the exit is the data-state layer, which binds signals into JSX and would be rewritten
  against React's own subscription mechanism.
- **Svelte 5.** The case for it is the model the field converged on, expressed with the least
  code per view, two packages at runtime, and a team with employer backing. Rejected because it
  replaced its own reactivity model within the last two years and signals a further major, its
  standalone routers are pre-1.0 and the maintained one is carried by a single maintainer, its
  compiler needs an owned bun plugin, and `.svelte` files port to nothing.
- **Vue.** The case for it is competence everywhere and an official router. Rejected because the
  design removed what its single-file-component toolchain would earn, its current router pulls
  dozens of build-time packages into the production tree, and its successor rendering model is in
  release candidate, so adopting it today adopts the outgoing half of a transition.
- **Lit and web components.** The case for it is standards over frameworks and a small runtime.
  Rejected on requirement 12, accessibility basics. ARIA identifier references cannot cross a shadow
  root and the standards fix is unshipped, and tables, menus, and live regions are the three
  surfaces the requirement names. Under the policy its `style` binding is blocked on first paint,
  the one candidate whose ordinary binding the policy breaks.
- **No framework, TypeScript over the DOM.** The case for it is zero dependencies and one banned
  identifier for requirement 1, escaping by default, which the measurements bore out. Rejected
  because the component model (3), streamed updates (5), keyboard and focus (7), and accessibility
  (12) become machinery the repository writes and maintains, accruing at the point the project would
  otherwise be finishing.
- **Go templates with htmx, the server-rendered shape.** The case for it is real. Every zoom step is
  a server aggregate already, Go's `html/template` escapes by context so requirement 1, escaping by
  default, becomes a type-level absence, and one language replaces two. Rejected because
  [docs/UI.md](../../UI.md#4-the-zoom-ladder)'s ladder is client-held interaction state, a chip
  stack, a pinned summary strip, a panel over an inert list, and a row cursor, and the hypermedia
  model needs a companion script exactly there. It is compatible with ADR-0062 only with its
  expression evaluation disabled from the entry document, which removes event filters, attribute
  handlers, and computed values, and ADR-0042's static-files clause does not bend to it by
  interpretation. Datastar, the other server-rendered candidate, requires `'unsafe-eval'` by its own
  security reference and was disqualified on that mechanism.

## Consequences

- A new view is a registry entry, a row renderer, and nothing else, as
  [ADR-0057](../operability/0057-one-dataset-endpoint-behind-a-registry.md) promises. The descriptor
  table the router reads is generated, so a new dataset reaches the browser's URL grammar without a
  hand edit.
- The URL grammar module, the cache, the stream client's handler, and the fixtures owe nothing to
  Preact and survive a framework change. The components that read no signal survive the alias
  exit to React, and the data-state layer is the part a framework change rewrites.
- Preact's next major is in release candidate and its changes are enumerated. They are caught by
  the compiler except one, effect cleanup deferring until after paint, which lands in the stream
  client's subscribe and unsubscribe, the one module [docs/UI.md](../../UI.md#9-live-surfaces)
  isolates.
- The one advisory in Preact's recent record is a type confusion that needs the API to return an
  object where the app assumes a string. It cannot arise here while every field rendered in text
  position is a scalar or an array of strings in the contract, because a Go string field
  marshals as a string and a changed type fails the drift check first. The rule is therefore
  named. No field of type `any`, `unknown`, or free-form JSON reaches a text position without
  re-arguing this consequence.
- Bun's development server serves no policy header and injects an inline script the policy
  would block, so the dev loop of [docs/UI.md](../../UI.md#18-repository-and-build-layout) never
  exercises ADR-0062. The policy is proven against the built output served by the Go handler,
  as ADR-0064 records.
- The built output's hashed asset names must reach the entry document
  [ADR-0061](../operability/0061-ui-browser-security-posture.md) has Go render. Either the build
  script writes a manifest or Go reads the bundler's emitted page. The builder picks.
- Assumptions about other components: the read API's row types are declared explicitly in the
  registry with closed value sets (ADR-0057) and are scalars or arrays of scalars for every
  message-derived field ([ADR-0016](../data/0016-schema.md)). The contract document of ADR-0065
  exists before the browser's descriptor table can be generated. The entry document is rendered
  by Go with the request token (ADR-0061). The stream's event shape is the one
  [docs/UI.md](../../UI.md#175-the-streams-event-shape) states, whatever ADR-0058's transport
  becomes.
- The escape-hatch ban, the `.value` rule with the route-prop rule it protects, and the roster are
  controls. Their violation injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
