# 0087. Each client operation declares its effect once, and its HTTP method, path shape and MCP annotations are all derived from it

**Status:** Accepted ·
**Pillar:** [One gate, N dumb adapters](../../../DESIGN.md#one-gate-n-dumb-adapters) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

[ADR-0030](./0030-api-core-mcp-thin-adapter.md) gives the client surface one API operation per MCP
tool, with an OpenAPI document as the contract, and makes anything beyond parity a separate
decision. [ADR-0053](../engineering/0053-parity-by-construction.md) generates both roots from one
operation registry. Every operation takes the account explicitly, with no implicit current account
([ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)). Every
identifier a caller supplies comes from a read on the same surface
([ADR-0035](./0035-required-identifiers-are-discoverable.md)). Search takes a canonical query, never
a provider's query string ([ADR-0010](../provider/0010-one-provider-port.md)). Approval is in no
client's vocabulary ([DESIGN.md](../../../DESIGN.md#approval-is-not-in-any-clients-vocabulary)).
What is left to choose is how each operation appears on the wire. That is its HTTP method and
path, how its arguments and its account travel, and what the MCP tool declares about its behaviour.
This record binds the mediator's client surface only. The UI's read API is the UI's own and keeps
its own routes ([docs/UI.md](../../UI.md#17-the-read-api)).

The choice looks like a question of HTTP style. It is really a question of where the facts about an
operation's behaviour are declared. An HTTP method and an MCP annotation state the same fact twice,
whether the operation is safe, idempotent or destructive. Every system that declares them separately
lets them drift. Notion's MCP server derived its annotations from the HTTP method and labelled its
own `POST` search destructive
([notion-mcp-server](https://github.com/makenotion/notion-mcp-server)). Servers Stainless generated
gave every `POST` no hints, so a pure computation defaulted to destructive
([a generated server's tools](https://github.com/dedalus-labs/dedalus-agents-typescript/tree/main/packages/mcp-server/src/tools)).
Unannotated tools default to destructive and open-world under the protocol
([MCP tools specification](https://modelcontextprotocol.io/specification/2026-07-28/server/tools)).

| # | Requirement | Demands | From |
| --- | --- | --- | --- |
| R1 | Parity | Both roots expose the same operations, and the same argument object reaches the service layer from each | ADR-0030, ADR-0053 |
| R2 | One declaration per fact | An operation's safety, idempotency and destructiveness are declared once, and the HTTP method and the MCP annotations cannot disagree | This record |
| R3 | Reads are safe methods | A read uses a safe HTTP method wherever its arguments allow ([RFC 9110 §9.2.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.1)) | This record |
| R4 | Approval unrepresentable | No method, path, argument or tool can change a plan's status | DESIGN.md, [ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md) |
| R5 | Explicit account | Every operation but the accounts listing names its account, and nothing holds an ambient one | ADR-0085 |
| R6 | Deployable anywhere | The surface assumes nothing a conformant cluster's ingress may not pass | [ADR-0051](../engineering/0051-environment-contract.md), [ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md) |
| R7 | Gated content never cached | No response the Redaction Gate decided can outlive a change in its decision | [ADR-0002](../redaction/0002-fetch-time-re-evaluation.md) |
| R8 | Usable by an agent | Tools shaped around tasks, few enough to choose between, each with one risk level and accurate annotations | This record |
| R9 | Search terms stay out of URLs | Structured query input never travels in a URL, which intermediaries log ([RFC 9110 §17.9](https://www.rfc-editor.org/rfc/rfc9110.html#section-17.9)) | This record |

R1, R4, R5 and R6 are gates. R2, R3 and R8 order the field. R7 and R9 hold for every candidate that
passes the gates once each design adds the mechanism named below, so they break no tie.

## Decision

- **Each registry entry declares one effect class, and everything else is derived from it.** The
  HTTP method, and all four MCP annotations, `readOnlyHint`, `destructiveHint`, `idempotentHint`
  and `openWorldHint`, come from this table and are never declared per operation.

  | Effect | HTTP method | readOnly | destructive | idempotent | openWorld |
  | --- | --- | --- | --- | --- | --- |
  | Read | `GET` | true | false | true | false |
  | Structured read | `POST` to a `:search` collection route | true | false | true | false |
  | Create | `POST` to a collection | false | false | false | false |
  | Reversible | `POST` to a `:verb` collection route | false | false | true | false |
  | Disposal | `POST` to a `:verb` collection route | false | true | true | false |

  A read whose arguments are all scalars is Read. A read that takes structured input, as search
  takes the canonical query, is Structured read. A change the operator can undo, such as labelling,
  unlabelling, moving, archiving, marking read or starring, is Reversible. Trash and spam are
  Disposal. The protocol's schema calls a tool destructive when it may do more than add, and its
  wording leaves reversible removals ambiguous, so this table records the reading chosen. Every
  operation stays inside the one mailbox, so `openWorldHint` is false throughout.
- **Paths are resource-shaped, under `/api/accounts/{account_id}/`.** An operation on a collection
  that is not a standard read or create takes a `:verb` route on the collection, as
  `messages:search` and `messages:label` do. Go's `ServeMux` matches such a segment as a literal. A
  read of a single resource's sub-resource takes a trailing path segment, as
  `reorg-plans/{plan_id}/sample` does, because `ServeMux` cannot route `{plan_id}:sample`, a
  wildcard segment having to end at its closing brace
  ([pattern.go](https://github.com/golang/go/blob/master/src/net/http/pattern.go)). The accounts
  listing is `GET /api/accounts`, and its place in the tree is why it alone takes no account.
- **Structured reads are `POST`, not `QUERY`.** `QUERY` is the safe, idempotent method with a body
  ([RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html)), and `ServeMux` routes it. The Gateway
  API's `HTTPMethod` enum has no `QUERY`
  ([httproute_types.go](https://github.com/kubernetes-sigs/gateway-api/blob/main/apis/v1/httproute_types.go)),
  and whether Envoy forwards it on a path-only route is an open question in its tracker
  ([envoy#18819](https://github.com/envoyproxy/envoy/issues/18819)), so R6 excludes it today.
  `POST` for a read that needs a body is the exception Google's resource guide makes
  ([AIP-136](https://google.aip.dev/136)), and the MCP annotations still mark it read-only.
- **Writes are `POST`, and the surface derives no `PUT`, `PATCH` or `DELETE`.** A batch body is not
  a representation of the resource its URL names, so `PUT` does not fit it, and a label change is
  an action on messages, not a replacement of their state. `PATCH` on a plan is where a status
  change would enter, and R4 rules it out.
- **The account travels in the path on the API root and as the `account_id` argument on the MCP
  root.** Parity means the same argument object reaches `Registry.Call` from both roots. The API
  root rebuilds that object from the path, the query string typed by the input schema, and the
  body, each argument having exactly one location. A generated round-trip property test proves, for
  every operation, that the object the API root rebuilds equals the MCP arguments. The refusal of
  an ambiguous account extends to an `account_id` given in the query string or the body beside the
  path.
- **The MCP root offers tools only.** A read is a tool, not a resource, because hosts surface
  resources to the model mainly on a user's mention, and a client could cache a gated body through
  one. Operations are shaped around the agent's tasks, and each is one tool and one route, named
  verb_noun, following Anthropic's guidance
  ([Writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents)) and
  what curated production servers do. No operation mixes a read with a write, and Disposal
  operations stay apart from the reversible ones, as Gmail's own MCP server keeps trash and spam
  apart from labelling
  ([Gmail MCP reference](https://developers.google.com/workspace/gmail/api/reference/mcp)).
- **Dry-run is a `dry_run` argument of each write**, as
  [ADR-0031](../mutation/0031-dry-run-on-mutating-operations.md) requires, not a separate preview
  tool per write.
- **Every response carries `Cache-Control: no-store` and no `ETag`, and `HEAD` is refused on every
  route.** `ServeMux` lets a `GET` pattern match `HEAD`, which would spend rate budget and write an
  audit row with no MCP counterpart.
- **The registry refuses, at build time, anything that could express approval.** No effect class
  derives `PUT`, `PATCH` or `DELETE`. A plan operation's input schema may not declare `status`. An
  operation name or path segment may not be approve, apply or rollback. The MCP root never elicits a
  confirmation that stands for approval.
- **The registry never emits the `x-mcp-header` keyword.** It would copy the account into a header
  on the MCP side to mirror the path, and the body check already carries what it would add.
- **An operation's name is snake_case and begins with a verb**, and it is the MCP tool's name and
  the contract's `operationId`.
- **The contract document is generated from the registry and checked in, and is not served.**
  Serving it would give the API root an endpoint the MCP root lacks, which ADR-0030 reserves to a
  separate decision.
- **The surface carries no version of its own.** The release version of
  [ADR-0049](../engineering/0049-image-per-component-lockstep.md) is the surface's version, and a
  breaking change to it is a breaking release.
- **The MCP result carries the output as structured content and the same JSON as text**, so a client
  that reads only text content still receives the result.
- **The mediator serves TLS on its listener unless its configuration declares that an ingress in
  front terminates TLS**, the two places ADR-0030 allows.
- **The health and readiness probes and the metrics endpoint listen on a separate plain-HTTP port**,
  outside the registry and outside the bearer check, as ADR-0051 and ADR-0053 place them.

What the ordinary path of HTTP and MCP tooling does that this record forbids, and what stops it:

| Construction | Harm | What stops it |
| --- | --- | --- |
| A method or an annotation declared on one operation | The HTTP method and the hint disagree, and a client trusts the wrong one | The registry has no field for either, and a test asserts the derived pair for one operation of each effect class |
| An annotation derived from the HTTP method | A structured read over `POST` is labelled destructive | The derivation reads the effect class only, pinned by the same test |
| A route added outside the registry | An operation on one root only | The existing route violation files ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)) |
| An argument with two locations, or a query string decoded as text | The two roots hand the service layer different objects | The round-trip property test, with a mutation that drops the typing |
| An `account_id` in the query string or body beside the path | The operation acts on an account other than the one checked | The ambiguous-account refusal, extended, with a test per location |
| A `HEAD` request on a `GET` route | Rate budget spent and an audit row written with no MCP counterpart | The refusal, with a test |
| A cacheable response to a gated read | A released body outlives a deny-listing | `no-store` on every response, with a test |
| A `PATCH` route, a `status` input or an approve path | Approval enters the client vocabulary | The registry's build-time refusals, each proven by a violation case |
| The `x-mcp-header` keyword in a schema | The account's binding differs between SDK revisions | The registry refuses it at build time, proven by a violation case |

| Requirement | Met by |
| --- | --- |
| R1 | One registry, the object rebuilt by the API root, and the round-trip property test |
| R2 | The effect class and the derivation table |
| R3 | Read derives `GET`, and Structured read is read-only on MCP though `POST` on HTTP |
| R4 | The registry's build-time refusals |
| R5 | The account in the path and as a required argument, and the extended ambiguous-account refusal |
| R6 | Only `GET` and `POST` on the wire |
| R7 | `no-store` on every response, no `ETag`, and `HEAD` refused |
| R8 | Task-shaped tools, one risk level per tool, derived annotations |
| R9 | Structured input in the body |

Left open, and settled where each is first needed:

- **The denial of a gated body read** is a successful result carrying the denial, or a failure. The
  failure contract of [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart) decides it,
  and either shape is derived the same way on both roots.
- **Paging** takes a cursor argument and returns the next cursor, bound to the account and the
  filter. Its exact fields are the first listing's.

What an implementer would otherwise pay to discover:

- `ServeMux` cannot route a custom verb on a single resource, since a wildcard must end its segment,
  and it matches `HEAD` on every `GET` pattern.
- A confirmation-prompting host prompts on a dry run of a destructive tool, because annotations are
  per tool. That cost is accepted.

## Alternatives considered

A cell reads "holds" when the design meets the requirement by its own structure, "weakens" when it
meets it only with added discipline or with a gap a check must close, and "fails" when it cannot
meet it. Every cell is a judgment over the evidence the passages cite. The Gateway API cell and the
`ServeMux` facts were read from source, and nothing on this grid was measured.

| Design | R1 | R2 | R3 | R4 | R5 | R6 | R8 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **Effect-derived, resource paths, `POST` for structured reads** | holds | holds | holds for scalar reads | holds | holds | holds | holds |
| Every operation `POST /api/<name>` | holds | holds | fails | holds | holds | holds | holds |
| As chosen, with `QUERY` for structured reads | holds | holds | holds for every read | holds | holds | fails today | holds |
| As chosen, with `PUT` for writes | holds | holds | holds for scalar reads | weakens | holds | holds | weakens |
| HTTP API primary, MCP generated from OpenAPI | weakens | fails | holds | weakens | holds | holds | weakens |
| Reads as MCP resources | weakens | holds | holds | holds | holds | holds | fails |
| A separate preview tool per write | holds | holds | holds | holds | holds | holds | weakens |

R7 and R9 separate none of these once each carries `no-store` and keeps structured input in a body.

- **Effect-derived, as chosen.** For it, parity now covers the method and the annotations as well as
  the operation set, and reads are safe methods wherever their arguments allow. Against it, a
  registry entry is no longer only a name, two schemas and a handler. The API root gains a path
  matcher and a schema-typed query-string decoder, generated code that needs its own property test.
  The account travels differently on the two roots. Structured reads remain `POST`.
- **Every operation `POST /api/<name>`**, the shape this record first took, which JMAP also uses
  ([RFC 8620](https://www.rfc-editor.org/rfc/rfc8620)). For it, the API root is a pass-through with no
  route table, and the account travels the same way on both roots. Against it, reads give up the
  safe method, which the operator rejected as making no sense, and nothing ties a hint to a method.
- **`QUERY` for structured reads.** For it, the correct method, safe and idempotent with a body.
  Against it, the Gateway API cannot express it, so a chart that assumes nothing about its cluster
  cannot rely on it.
- **`PUT` for writes.** For it, idempotency visible in the method. Against it, the batch body names
  no single resource to replace, and label membership alone fits a `PUT`, which would split one
  task across per-message calls. A resource open to `PUT` also invites a replacement of a plan,
  status included, which the registry would then have to refuse by rule.
- **HTTP API primary, MCP generated from OpenAPI.** For it, the canonical HTTP design, and
  generators exist. Against it, the generators flatten path, query and body by rules of their own
  and rename on collision ([FastMCP OpenAPI](https://gofastmcp.com/integrations/openapi)), none
  derives useful annotations from the method, and vendors who shipped one tool per route have moved
  away from it, as Stainless did when it kept only its code-mode tools
  ([Stainless MCP changelog](https://www.stainless.com/changelog/products/mcp)).
- **Reads as MCP resources.** For it, the protocol's application-controlled primitive for data.
  Against it, hosts show resources to the model mainly on a user's mention, so an agent would
  rarely find them, and a client may cache a gated body.
- **A separate preview tool per write.** For it, a dry run carries a read-only hint and draws no
  confirmation. Against it, it doubles the write surface past thirty tools.

## Consequences

- Leaving this design costs the derivation table and the API root's binder. The registry, the
  service layer and the MCP root survive.
- What would re-argue it. The Gateway API's method enum gains `QUERY` and a live run shows the
  common ingresses pass it, which changes one row of the derivation table. The protocol settles the
  meaning of `destructiveHint` for reversible changes differently.
- The API root decodes by declared type and validates nothing, and each operation's schema and the
  service layer's checks are what keep an operation's input honest.
- Assumptions about other components. The registry holds a name, two schemas, an effect class, a
  path and a handler per operation. The service layer refuses a missing or unknown account for
  every operation but the accounts listing.
