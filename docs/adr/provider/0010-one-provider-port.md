# 0010. One provider port: a canonical contract the adapters compile to

**Status:** Accepted ·
**Pillar:** [Everything above the port speaks canonical](../../../DESIGN.md#everything-above-the-port-speaks-canonical) ·
**Serves:** [P1](../../../USE_CASES.md#p1--one-contract), [P2](../../../USE_CASES.md#p2--backend-swap)

## Context

Gmail and JMAP disagree on nearly everything observable: query syntax, label semantics, change
feeds, batching, metadata guarantees. The system above them — gate, classifier, authorizer,
client surface, batch workloads — must not know which is in use, and the contract must be shaped
so the safety-relevant distinctions (metadata versus body) are structural, not conventional.

## Decision

A single port, with a canonical metadata model:

```python
@dataclass(frozen=True)
class MessageMetadata:
    id: str
    account_id: str
    thread_id: str
    from_: Address
    to: list[Address]
    cc: list[Address]
    subject: str
    date: datetime                   # UTC
    labels: list[str]
    flags: MessageFlags
    size_bytes: int
    has_attachments: bool
    attachment_names: list[str]
    snippet: str | None              # redactable
    list_id: str | None              # RFC 2919 — high-signal
    auth_results: AuthResults        # SPF/DKIM/DMARC

class MailProvider(Protocol):
    async def list_threads(self, q: Query, page: Page) -> Page[ThreadMetadata]: ...
    async def get_thread_metadata(self, thread_id: str) -> ThreadMetadata: ...
    async def get_message_metadata(self, ids: list[str]) -> list[MessageMetadata]: ...
    async def get_message_body(self, id: str) -> MessageBody: ...     # gate-guarded
    async def list_labels(self) -> list[Label]: ...
    async def ensure_label(self, path: str) -> Label: ...             # for reorg
    async def mutate(self, ops: list[MutationOp]) -> MutationResult: ...
    async def changes_since(self, cursor: str) -> ChangeSet: ...
    async def enumerate_all(self, cursor: str | None) -> Page[MessageMetadata]: ...

    # Cost declaration — the adapter is the only layer that knows what
    # an operation costs on its provider. See ADR-0023.
    def rate_profile(self) -> RateLimitProfile: ...
```

The choices inside the contract, each with its reason:

| Choice | Rationale |
| --- | --- |
| `get_message_body` separate from metadata | Structural: metadata paths physically cannot carry a body; a leak requires calling the one gate-guarded method |
| `Query` is a canonical AST | Gmail `q=` and JMAP `Filter` differ; each adapter compiles the AST. Keeps Gmail syntax out of the agent's model |
| `changes_since` returns an opaque cursor | Gmail `historyId`, JMAP `state` — same semantics, different tokens |
| `enumerate_all` distinct from `changes_since` | Full traversal is resumable but not a delta; backfill needs it and sync must not use it |
| `ensure_label` in the port | Reorg creates taxonomy; without it each adapter invents its own create-if-missing |
| Batched mutation ops | Gmail `batchModify` and JMAP `Email/set` both batch natively |
| `auth_results` in metadata | Required for the spoofing posture in [ADR-0004](../classification/0004-sender-list-decides.md) |
| `account_id` everywhere | Multi-account correctness enforced by type, not convention |
| `rate_profile()` on the port | Cost models are provider knowledge; see [ADR-0023](../operability/0023-adapter-declares-cost.md) |

What each adapter compiles, per concern:

| Concern | Gmail | JMAP |
| --- | --- | --- |
| Delta sync | `history.list` from `historyId`; gap → resync | `Email/changes` from `state`; `cannotCalculateChanges` → resync |
| Full enumeration | `messages.list` + `pageToken` | `Email/query` position/anchor |
| Metadata fetch | `format=METADATA` — **provider guarantees no body** | `Email/get` with explicit `properties`, omitting `bodyValues` |
| Labels | flat IDs | mailbox tree; adapter flattens to paths |
| Batch | `batchModify`, 1000 ids | `Email/set` multi-update |

Gmail's `format=METADATA` is a genuine asset: on the enumeration path the provider itself
guarantees no body crosses the wire, making gate-side redaction defense-in-depth rather than sole
defense. The contract is shaped so adapters can exploit such guarantees wherever a provider offers
them.

## Alternatives considered

- **Expose the richer provider's API and emulate on the other.** Rejected: Gmail-shaped concepts
  would spread above the boundary, and every emulation gap becomes a behavioral difference the
  gate and tools must know about — the exact provider-awareness the port exists to prevent.
- **Pass provider-native query strings through.** Rejected: query syntax in the agent's vocabulary
  couples the client surface to one backend and makes queries unanalyzable by the mediator.
- **One combined `get_message` returning metadata plus optional body.** Rejected: it makes "does
  this call carry content" a runtime question. Two methods make it a structural one.

## Consequences

- The contract is the intersection both backends can honour, normalized — a canonical operation one
  backend cannot express is a contract bug, not an adapter branch.
- Adapters are deliberately dumb: full data in, no redaction responsibility, no policy knowledge.
- The contract is proven only when the second mail adapter is built; if that adapter forces a
  change above the port, the contract was wrong. That test is deliberately deferred and tracked in
  [ROADMAP.md](../../../ROADMAP.md).
