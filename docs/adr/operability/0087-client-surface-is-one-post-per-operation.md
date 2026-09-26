# 0087. Each client operation is one `POST` to a path named for it, carrying the account and every argument in one JSON object, the same object the MCP tool takes

**Status:** Accepted ·
**Pillar:** [One gate, N dumb adapters](../../../DESIGN.md#one-gate-n-dumb-adapters) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

[ADR-0030](./0030-api-core-mcp-thin-adapter.md) gives the client surface one endpoint per operation
on the API root, mirrored one-to-one by an MCP tool, with an OpenAPI document as the contract, and
makes anything beyond parity a separate decision. [ADR-0053](../engineering/0053-parity-by-construction.md)
generates both roots from one operation registry whose entries carry each operation's schema and
identity. Every operation takes the account explicitly, with no implicit current account
([ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)). Every
identifier a caller supplies comes from a read on the same surface
([ADR-0035](./0035-required-identifiers-are-discoverable.md)). Search takes a canonical query, never
a provider's query string ([ADR-0010](../provider/0010-one-provider-port.md)). None of these
records says which HTTP method an operation uses, where its account travels, or what its path
looks like. The UI's read API chose resource-style routes with the account as a path segment, which
binds only the UI ([docs/UI.md](../../UI.md#17-the-read-api)).

## Decision

- **Every operation is `POST /api/<operation name>`, and its request body is one JSON object
  holding every argument**, validated by the service layer against the operation's input schema.
  The response body is one JSON object matching its output schema. A registry entry then needs a
  name and two schemas and nothing else, and the API root has no route table to hand-write.
- **The MCP tool of the same name takes the same object as its arguments**, so the two roots carry
  identical inputs and outputs and parity holds on the arguments as well as on the set of
  operations.
- **The account is a required argument of every operation except the accounts listing**, named
  `account_id`, in the same object on both roots. The service layer refuses an operation whose
  account is missing or unknown.
- **An operation's name is snake_case and begins with a verb**, as the documents already name
  `get_message_body`, `list_masking_events`, `describe_reorg_plan` and `sample_reorg_plan`. The
  name is the path's last segment and the tool's name.
- **The contract document is generated from the registry and checked in, and is not served.**
  Serving it would give the API root an endpoint the MCP root lacks, which ADR-0030 reserves to a
  separate decision. A client reads it from the repository at the release it runs.
- **The surface carries no version of its own.** The release version of
  [ADR-0049](../engineering/0049-image-per-component-lockstep.md) is the surface's version, and a
  breaking change to it is a breaking release.
- **The MCP result carries the output as structured content and the same JSON as text**, so a
  client that reads only text content still receives the result.
- **The mediator serves TLS on its listener unless its configuration declares that an ingress in
  front terminates TLS**, the two places ADR-0030 allows.
- **The health and readiness probes and the metrics endpoint listen on a separate plain-HTTP
  port**, outside the registry and outside the bearer check, as
  [ADR-0051](../engineering/0051-environment-contract.md) and ADR-0053 place them.

## Alternatives considered

- **Resource-style routes**, `GET` for reads with the account and identifiers in the path or the
  query string and `POST` for mutations, as the UI's read API does. For it, idiomatic HTTP and one
  style across the project's two APIs. Against it, the MCP tool takes one argument object, so the
  same operation carries its account and identifiers in the path on one root and in the arguments
  on the other, every registry entry needs a method and a path template, and a structured canonical
  query does not fit a query string.
- **`POST /api/{account}/<operation name>`.** For it, the account is visible in every path.
  Against it, the MCP tool still carries the account as an argument, so the two roots carry it
  differently, and the accounts listing needs an exception route.
- **The account in a header.** No case was tabled for it. A header set once per MCP connection
  behaves as the ambient current account ADR-0085 rejects.
- **Serving the contract at a well-known path.** For it, a client of the API can fetch its
  description at run time, as an MCP client lists its tools. Against it, an endpoint beyond parity,
  which is its own decision if a client ever needs it.
- **A version in the path or a header.** For it, two versions can be served side by side. Against
  it, the compatibility matrix ADR-0049 rejected, for a system one operator deploys in lockstep.

## Consequences

- The failure contract of [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart), paging,
  search input and the dry-run form ride on this shape and are decided by the operations that
  first need them.
- Both roots validate nothing themselves, so an operation's schema and the service layer's checks
  can disagree, and each operation's tests are what keep them in step.
- Assumptions about other components. The registry holds only a name, two schemas and a handler per
  operation, and the service layer refuses a missing or unknown account for every operation but the
  accounts listing.
