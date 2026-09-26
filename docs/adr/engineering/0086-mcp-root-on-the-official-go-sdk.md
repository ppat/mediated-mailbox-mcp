# 0086. The MCP root speaks the protocol through the official Go SDK, stateless and tools only, with its capabilities declared explicitly

**Status:** Accepted ·
**Pillar:** [One gate, N dumb adapters](../../../DESIGN.md#one-gate-n-dumb-adapters) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

[ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md) makes the MCP endpoint a thin
protocol adapter over the service layer, with exact parity between the MCP and API roots, and
[ADR-0053](./0053-parity-by-construction.md) generates its tool set from the operation registry.
[ADR-0042](./0042-implementation-stack.md) leaves each major library to its own decision.

Much of what a protocol library offers is already settled elsewhere. The Redaction Gate and the
Mutation Authorizer sit below the service layer, so no fault in the MCP root can release a body.
A protocol fault costs utility, never confidentiality. Parity forbids every protocol feature but
tools, so prompts, resources, completions, logging, sampling and subscriptions are surface to
fence, not strength. Each tool carries the output schema and the four annotations the registry
derives from its operation's effect class
([ADR-0087](../operability/0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)),
and those are fields of a tool, not protocol features. Bearer authentication and TLS sit in front
of both roots. Argument validation, UTC timestamps
([ADR-0033](../operability/0033-utc-only-timestamps.md)) and the shape of a failure belong to the
service layer, shared by both roots. What is left for this choice is who implements the protocol,
how its correctness is proven, what surface the root exposes, and what it links into the
mediator, a process that holds full-mailbox credentials.

The choice looks like picking a library. It is really whether outside code implements the
protocol inside the mediator at all.

| Requirement | What it demands | From |
| --- | --- | --- |
| R1 Footprint | Modules and bytes added to a process that holds full-mailbox credentials | [ADR-0028](../operability/0028-trust-anchor-hardening.md), "nothing beyond what it needs", and ADR-0042 on a deep package tree inside the trust anchor |
| R2 Revisions | Who absorbs each protocol revision, indefinitely | [ADR-0076](./0076-metrics-emitted-through-client-golang.md), which rejected owning a protocol indefinitely |
| R3 Proof | A test that goes red when the root stops conforming | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| R4 Tools only | No protocol feature beyond tools, and each tool carries only what the registry derives for it | ADR-0030's exact parity, ADR-0053 |
| R5 Governance | The longevity of what is depended on | [ADR-0066](../data/0066-data-access-generated-from-sql.md), which broke ties on what abandonment costs |
| R6 Revisions spoken | The agent's current revision and older clients' | ADR-0030, every client |
| R7 Registration from data | Tools registered at run time from the registry's input and output schemas and annotations, all as data | ADR-0053, ADR-0087 |
| R8 Arguments untouched | Arguments reach the service layer as the client sent them | ADR-0030, no logic in a frontend |
| R9 Stateless | No session or stream kept between requests | [ADR-0051](./0051-environment-contract.md), any process killed at any point |
| R10 Failures intact | A failed call's structured result reaches the client | [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart) |
| R11 Mounting | An `http.Handler` behind the mediator's TLS and bearer check | ADR-0030 |
| R12 Environment | No undeclared environment input | ADR-0051, [O6](../../../USE_CASES.md#o6--deployable) |
| R13 Annotations | All four hints, `readOnlyHint`, `destructiveHint`, `idempotentHint` and `openWorldHint`, set explicitly from data on raw registration and reaching the client with the meaning the registry derived | ADR-0087 |

R6, R7 and R13 are gates, and R13 separates none of the candidates that pass the other two. R2,
R3 and R4 order the field. Footprint breaks ties, as the operator ruled, consistent with how
ADR-0066, [ADR-0074](../redaction/0074-html-to-markdown-v2-converts-bodies.md) and ADR-0076 weighed
it for code in or near the same processes. Throughput, licences and payload logging were not
graded. One operator's agent sets no throughput bar, both remaining libraries are permissively
licensed, and neither logs payloads on the paths used.

## Decision

- **`github.com/modelcontextprotocol/go-sdk`, the SDK the protocol's organization publishes**, at
  its latest version. It is one of the protocol's Tier 1 SDKs, which ship each revision before the
  specification releases
  ([SDK tiers](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/community/sdk-tiers.mdx)),
  and it runs the protocol's conformance suite in its own CI
  ([its conformance workflow](https://github.com/modelcontextprotocol/go-sdk/blob/main/.github/workflows/conformance.yml)).
  It supports every revision from 2024-11-05 to 2026-07-28, the one the agent speaks
  ([its version table](https://github.com/modelcontextprotocol/go-sdk#version-compatibility)).
- **Stateless.** `NewStreamableHTTPHandler` runs with `Stateless` set, so no session is kept
  between requests.
- **Tools-only capabilities, declared explicitly.** The server is constructed with capabilities
  naming tools and nothing else. With no options the SDK advertises logging and a changing tool
  list ([`capabilities()`](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/server.go#L657-L695)),
  and an agent then holds a subscription stream open against the server, one per client.
- **Raw tools only.** Tools are registered with their schemas as data and a raw handler, so the
  arguments reach the service layer untouched. The SDK's typed registration validates on its own
  ([`ToolHandlerFor`](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/tool.go#L32-L42))
  and is not used. Each tool also carries its output schema as data, and annotations built from its
  operation's effect class with all four hints set. The SDK leaves a nil `DestructiveHint` or
  `OpenWorldHint` out of the tool it lists
  ([`ToolAnnotations`](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/protocol.go#L1964-L1991)),
  and the specification then defaults both to true, so a tool left with either unset reads as
  destructive and open-world.
- **A receiving middleware inside the SDK refuses the protocol surface parity forbids.** The SDK
  answers requests for prompts, resources and a log level with empty or accepting replies whatever
  its options ([its method table](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/server.go#L1876-L1880)).
  The middleware answers them as method not found, on the method the SDK itself parsed, so no
  request can read one way to the refusal and another way to the SDK, and the root exposes tools
  and nothing else.
- **Only the root's generator file imports the SDK**, and it registers only registry operations.

What the SDK's ordinary path does that the design forbids, and what stops it:

| Construction | Harm | What stops it |
| --- | --- | --- |
| A server built with no options | It advertises logging and a changing tool list, and each agent parks an open stream against a stateless server | A test asserting the advertised capabilities are exactly tools at both the legacy and the 2026-07-28 revision, with a mutation removing the option |
| A prompt, a resource or a log level answered | Surface on one root that the other lacks | The receiving middleware, and a test that sends each, a request with a case-variant method key and a batch, and expects every one refused |
| Typed tool registration | Arguments validated or reshaped in the frontend | A root that uses raw handlers only, reviewed in its one generator file |
| The SDK imported outside the root's generator | Protocol code spread into the service layer | The import lists and their violation files ([ADR-0071](./0071-static-enforcement-toolchain.md)) |
| A tool registered with a hint pointer left nil | The client reads the tool as destructive and open-world | A test asserting the literal annotations of one tool per effect class at both revisions, with mutations dropping each pointer |
| `MCPGODEBUG` set in the mediator's environment | SDK behaviour switched without a change here. `allowsessionsinstateless` puts sessions back into the stateless handler, and `hintomitempty` changes the annotation bytes the literal test pins, though not their meaning, since both hints it omits default to false | The mediator refuses to start while it is set, even to an empty value, proven by a test with a mutation removing the refusal |
| The `x-mcp-header` keyword in a registry schema | The SDK binds a header to an argument, and the account's binding differs between revisions | A registry rule refuses it at generation, proven by a violation case |

| Requirement | Met by |
| --- | --- |
| R1 Footprint | Accepted at seven modules and about 1.8 MB on a mediator of sixteen modules and about 13.9 MB |
| R2 Revisions | The SDK, Tier 1 |
| R3 Proof | The SDK's own conformance CI, and the root's tests at both revision eras |
| R4 Tools only | The capabilities option, the receiving middleware, the import fence and a literal test |
| R5 Governance | The protocol's own organization |
| R6 Revisions spoken | 2024-11-05 to 2026-07-28 |
| R7 Registration from data | Tools added with their input and output schemas and their annotations as data |
| R8 Arguments untouched | Raw handlers |
| R9 Stateless | `Stateless` plus the capabilities option |
| R10 Failures intact | Error results carrying the service layer's structured content |
| R11 Mounting | The streamable HTTP handler is an `http.Handler` |
| R12 Environment | The mediator refuses to start while `MCPGODEBUG` is set |
| R13 Annotations | `ToolAnnotations` with all four hints set, and the literal annotation test |

What an implementer would otherwise pay to discover:

- A conforming client does not prove a server. A test through the official client stayed green
  under faults the conformance suite caught, so the root's own tests assert the exact replies.
- A proxy in the same pod that forwards over loopback without rewriting `Host` is refused by the
  SDK's protection against DNS rebinding
  ([`DisableLocalhostProtection`](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/streamable.go),
  [the specification's guidance](https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices#local-mcp-server-compromise)).
  An ingress in front connects over the pod network and is unaffected.
- A tool whose `DestructiveHint` or `OpenWorldHint` pointer is nil is listed without it, and a
  client then applies the specification's default of true. At the 2026-07-28 revision a
  `tools/list` reply also carries `ttlMs` and `cacheScope`, which a literal test of the listing
  expects.

## Alternatives considered

Six candidates were graded, four of them built in scratch programs against the same registry, the
real agent and the protocol's conformance suite. Grades run 4 (carries it natively), 3 (with
bounded discipline or one small package), 2 (only by convention or with a named gotcha) and 1
(cannot).

| Candidate | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | R12 | R13 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **Official SDK** | 2 | 4 | 2 | 3 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 |
| Hand-written on the standard library | 4 | 2 | 2 | 3 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 |
| mark3labs/mcp-go | 2 | 3 | 2 | 3 | 2 | 4 | 3 | 3 | 3 | 4 | 4 | 4 | 3 |
| creachadair/jrpc2 under an owned MCP layer | 3 | 2 | 2 | 3 | 2 | 4 | 4 | 4 | 3 | 4 | 4 | 4 | 4 |
| metoro-io/mcp-golang | 2 | 1 | 1 | 3 | 1 | 1 | 1 | 1 | 1 | 3 | 3 | 4 | not graded |
| ThinkInAIXYZ/go-mcp | 2 | 1 | 1 | 3 | 2 | 1 | 4 | 3 | 2 | 2 | 3 | 4 | not graded |

R13 was graded after the gates had removed the last two rows. mcp-go takes a 3 because its raw
schema tool fails at run time when the library's own options set its annotations, so they are
assigned as struct fields.

The grid shows the following.

- The gates remove two. metoro-io/mcp-golang and ThinkInAIXYZ/go-mcp speak only the oldest
  revisions, and the first cannot register tools from data.
- mcp-go is dominated by the SDK, lower on revisions, proof, governance, registration, arguments
  and statelessness and equal elsewhere. jrpc2 under an owned layer is dominated by the hand-written
  root, no better anywhere and a module worse.
- R8 to R12 separated no one among the survivors, because the service layer and the mediator's own
  front already carry what they ask. R13 is met by every survivor, so it separates none either.
- The SDK and the hand-written root tie on proof. The SDK's own CI passes the whole suite on its
  reference server, while the configuration this record picks, run through the suite, failed one
  check, a prompt list answered without prompts declared, which the receiving middleware closes.
  The hand-written root passed with no real failure across sixty checks after one round of fixes,
  and its proof is the suite alone.
- The two leaders split on R1 and R2 only. Every cell of the SDK and mcp-go rows on R1, R6, R7, R9,
  R10 and R11 was measured, the hand-written row was measured throughout, and the rest were read
  from source and documentation.

| | Official SDK | Hand-written |
| --- | --- | --- |
| Who implements the next revision | The SDK, before the specification ships | The project |
| Who proves conformance | The SDK's CI, plus the root's tests | The project's CI, running a pre-release third-party suite |
| Modules added to the mediator | 7 | 0 |
| Bytes added to the mediator | about 1.8 MB | about 53 KB |
| Lines the project owns in the root | about 50, plus the receiving middleware | about 275 |

The reading. Neither leader has a 1. Each has two cells at 2, and they share R3. The SDK's other
weak cell is footprint, which the records let break ties, and the hand-written root's is
revisions, which orders the field. Regretting the SDK is a vulnerable or malicious release among
seven modules inside the credential holder, quiet until an advisory. Regretting the hand-written
root is a revision not adopted or a conformance gap, loud in the agent. Both exits replace one
file. Everything below the root survives either way, and the SDK's conformance proof is partly
detachable, since the same suite can run against a hand-written root, while its footprint is not.

- **The official SDK.** For it, the protocol's own organization carries every revision, with its
  conformance suite in its CI and a record of fixing its
  [security advisories](https://github.com/modelcontextprotocol/go-sdk/security/advisories), and
  the project owns about fifty lines. Against it, seven modules and about 1.8 MB inside the trust
  anchor, most of it code the root never calls, among them an OAuth client and an
  assembly-accelerated JSON decoder that parses every untrusted request. Its defaults open surface
  parity forbids, so tools-only is held by an option, a middleware, a fence and a test rather than
  by absence. A tool with an unset hint reads as destructive, which a test closes rather than the
  type. It changes behaviour inside version 1 through
  [`MCPGODEBUG` compatibility flags](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/mcpgodebug.md)
  that expire.
- **Hand-written on the standard library.** For it, about 275 lines, no module and about 53 KB,
  conforming after one round and working with the real agent at the 2026-07-28 revision, which
  [removed sessions, the server's stream and server-initiated requests](https://modelcontextprotocol.io/specification/2026-07-28/changelog)
  and so made the protocol far smaller. Against it, the project owns every revision indefinitely,
  and only the conformance suite proves it, whose 2026-07-28 scenarios exist only in
  [a pre-release package](https://www.npmjs.com/package/@modelcontextprotocol/conformance?activeTab=versions).
- **mark3labs/mcp-go.** For it, a mature community SDK. Against it, dominated by the official SDK,
  with [most commits from one maintainer](https://github.com/mark3labs/mcp-go/graphs/contributors)
  and [no conformance suite in its CI](https://github.com/mark3labs/mcp-go/tree/main/.github/workflows).
- **creachadair/jrpc2 under an owned MCP layer.** For it, a maintained JSON-RPC layer. Against it,
  framing is a small part of the hand-written root, so it adds a module and saves little.
- **metoro-io/mcp-golang and ThinkInAIXYZ/go-mcp.** Small libraries. Neither speaks a current
  revision.
- **A separate MCP deployment** on another language's SDK, or a gateway deriving tools from the
  API's contract. For it, the reference implementation and a process boundary between the adapter
  and the credential holder. Against it, ADR-0030 rejected a second deployment, ADR-0042 runs no
  JavaScript server in production, and a gateway would move parity into a third party's mapping.

## Consequences

- Leaving the SDK replaces the root's one generator file with a hand-written root and adds the
  conformance suite to CI. The registry, the service layer, the API root and every test above the
  wire survive.
- What would re-argue it. Footprint in the trust anchor weighed to order the field rather than
  break ties, the SDK's server package growing further or an advisory landing in a module the root
  never calls, the receiving middleware failing to close the conformance gap, the protocol's
  deprecation policy holding across its next revision so that it becomes small and stable in
  ADR-0076's sense, an SDK release that changes how `ToolAnnotations` is serialised, or adopting
  `x-mcp-header` for any parameter.
- Each SDK release is read for retired `MCPGODEBUG` flags and protocol changes before it is taken.
  Its flag documentation schedules every flag now live, `hintomitempty` among them, for removal in
  v1.9.0 ([`mcpgodebug.md`](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/mcpgodebug.md)).
- Assumptions about other components. The service layer validates arguments, rejects timestamps
  that are not UTC and shapes failures for both roots. The registry derives all four hints from the
  effect class, and a registry rule refuses `x-mcp-header` at generation, proven by its violation
  case. The bearer check runs before the root on every request.
