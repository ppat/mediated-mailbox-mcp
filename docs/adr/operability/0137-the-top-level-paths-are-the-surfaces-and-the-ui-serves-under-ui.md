# 0137. The product's top-level paths are its surfaces, and the UI serves everything under `/ui`

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

The product serves two surfaces over HTTP. The client surface is the API server's API under
`/api/accounts/{account_id}/…` and its MCP root at `/mcp`, the paths its clients call
([ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)). The UI
serves its pages, its bundle's files and its read API. Only the browser bundle shipped in the same
binary calls the read API ([docs/UI.md section 17](../../UI.md#17-the-read-api)).

Two forces bear on the paths the UI serves.

- **One origin for both surfaces.** A deployment may serve both surfaces from one origin, through
  one process or through a gateway that routes by path. That works only when no path pattern of
  one surface conflicts with a pattern of the other, matching the same requests with neither more
  specific, which Go's router refuses at registration.
- **Identifiers at the position of a word.** An identifier the operator chooses, an account's or a
  rule's, is stored once and kept. When an address puts that identifier at a position where a
  route word can also sit, every word at that position has to be refused as an identifier. The
  words then form a closed list that tests pin, because a word added later would make an account
  or a rule stored under it unreachable. The client surface puts each identifier after a word
  naming it, `/api/accounts/{account_id}`, so no word can take an identifier's place.

## Decision

- **The top-level words are the surfaces.** `/api/…` and `/mcp` are the client surface's, as
  [ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md) derives
  them. `/ui/…` is the UI's, everything it serves. Nothing else sits at the top level but `/`, which
  the UI answers with a redirect to `/ui/`. The probes and the metrics endpoint are on each
  process's own plain-HTTP listener ([ADR-0051](../engineering/0051-environment-contract.md)), outside
  this namespace.
- **Each surface owns its subtree.** Under `/ui` the pages are `/ui/accounts/{account}/…` and the
  installation's `/ui/setup/…`, the read API is `/ui/api/…`, the bundle's files are `/ui/main.js`,
  `/ui/main.css` and `/ui/fonts/…`, and `/ui/` sends the browser to the account last used. The UI's
  cookies are scoped to its subtree, so a browser sends them to no other
  surface ([docs/UI.md section 15](../../UI.md#15-security-of-the-ui-itself)).
- **An identifier someone chooses follows a word naming what it identifies**, never at a position
  where a route word sits. An account's identifier follows `accounts`, a rule's identifier follows
  `rules`, and a base rule seen from an account follows `base`. So no list of route words binds what
  an identifier may be, and none is refused for equalling a word. The one exception is an OAuth
  client's name, which sits beside `new` at `/ui/setup/{provider}/{client}`. Its own grammar, a
  Google Cloud project ID's shape of at least six characters, cannot spell `new`
  ([docs/UI.md section 8.11](../../UI.md#811-oauth-client-setup)).
- **Every route the UI's server registers sits under `/ui/`**, apart from `/`, and a test holds it
  ([docs/VERIFICATIONS.md](../../VERIFICATIONS.md)).

## Alternatives considered

- **The UI at `/`, with its pages at `/accounts/{account}/…` and its read API under a top-level
  word of its own.** The case made for it is that it keeps the read API apart from the client
  surface and every identifier after a word, with the UI's pages one segment shorter. It loses
  because the top level then holds the UI's words, `accounts`, `setup`, the read API's word and
  the bundle's names, beside the client surface's. Routing the surfaces by path needs every one of
  those words listed, and each word the UI adds changes that routing. One word per surface is
  simpler.
- **The client surface under another top-level word, with the UI at `/`.** The case made for it is
  that it touches the fewest paths in the code. It loses because the client surface's paths are the
  external contract its clients call, while the read API is private to the bundle, so the contract
  would bend to the private surface. `/api` is also the name a public API conventionally takes.
- **Account-first pages at the top level, with `mcp` among the words account setup refuses.** The
  case made for it is that it is the smallest change that lets one process serve `/mcp` beside the
  UI. It loses because it keeps the closed lists of words and the read API under `/api`, so the two
  surfaces still cannot share one router.

## Consequences

- **Assumed of the client surface.** Its paths stay `/api/…` and `/mcp`, so the API server and the
  UI can share one origin, served by one process or by a gateway routing by path, with no pattern
  of one conflicting with the other's.
- **Assumed of the account identifier grammar.** One lowercase DNS label
  ([ADR-0128](../data/0128-account-identifiers-are-one-lowercase-dns-label.md)) is all that bounds an
  account's identifier, and a word the UI adds under its root cannot make a stored account or rule
  unreachable.
- **A rule's identifier is refused when it is exactly `.` or `..` or holds a `/`**, since no path
  segment can hold it ([docs/UI.md section 8.7](../../UI.md#87-policy)).
- **The browser builds every page address and every read of the read API from one module holding
  the root**, and a build test holds the stylesheet's font URLs to it
  ([docs/UI.md section 18](../../UI.md#18-repository-and-build-layout)).
- **A deployment routing by path** sends `/ui` and `/` to the UI and the rest to the API server,
  which the chart's routes for the UI default to
  ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)).
- **The UI's paths are the operator's bookmarks and a deployment's routing**, so changing the root
  after a release changes both, and is argued here if it is ever wanted.
