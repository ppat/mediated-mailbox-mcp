# 0010. One provider port: a canonical contract the adapters compile to

**Status:** Accepted ·
**Pillar:** [Everything above the port speaks canonical](../../../DESIGN.md#everything-above-the-port-speaks-canonical) ·
**Serves:** [P1](../../../USE_CASES.md#p1--one-contract), [P2](../../../USE_CASES.md#p2--backend-swap)

## Context

Gmail and JMAP disagree on nearly everything observable, among them query syntax, label semantics,
change feeds, batching and metadata guarantees. The system above them (gate, classifier,
authorizer, client surface, batch workloads) must not know which is in use, and the contract must
be shaped so the safety-relevant distinctions (metadata versus body) are structural, not
conventional.

## Decision

The decision is a single port, with a canonical metadata model.

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
    async def current_cursor(self) -> str: ...                        # where delta sync starts
    async def enumerate_all(self, cursor: str | None) -> Page[MessageMetadata]: ...  # may carry a total

    # Cost declaration — the adapter is the only layer that knows what
    # an operation costs on its provider. See ADR-0023.
    def rate_profile(self) -> RateLimitProfile: ...
```

These are the choices inside the contract, each with its reason.

| Choice | Rationale |
| --- | --- |
| `get_message_body` separate from metadata | Structural: metadata paths physically cannot carry a body; a leak requires calling the one gate-guarded method |
| `Query` is a canonical AST | Gmail `q=` and JMAP `Filter` differ; each adapter compiles the AST. Keeps Gmail syntax out of the agent's model. The agent's own reads of the index select by the client surface's index query, which no adapter compiles and which carries no provider's syntax either ([ADR-0108](../operability/0108-index-reads-select-by-an-index-query-of-the-surfaces-own.md)) |
| `changes_since` returns an opaque cursor | Gmail `historyId`, JMAP `state` — same semantics, different tokens |
| `enumerate_all` distinct from `changes_since` | Full traversal is resumable but not a delta; backfill needs it and sync must not use it |
| `enumerate_all`'s page may carry a total | The backfill card shows a pass's page of total pages and an estimated time left ([docs/UI.md section 8.1](../../UI.md#81-home)), which needs to know how many pages an enumeration takes. The port reports the total rather than the card's design changing. The total counts items, and the page also carries the most items any page of the listing holds, so a caller derives a page count from the two numbers and the next-page token alone. The items are the messages a full enumeration returns as the provider counts them. That keeps the caller to what the contract states, which [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) requires, since how an adapter sizes its pages is no promise of the port. The total is optional, so a backend or the provider fake that does not count reports none, and the card shows no estimate. That keeps it inside [P1](../../../USE_CASES.md#p1--one-contract)'s intersection: Gmail counts with `messagesTotal` on [`users.getProfile`](https://developers.google.com/workspace/gmail/api/reference/rest/v1/users/getProfile), "the total number of messages in the mailbox", and JMAP's `/query` returns `total` when the request sets `calculateTotal` and clamps its own `limit`, returning the limit it enforced ([RFC 8620 section 5.5](https://www.rfc-editor.org/rfc/rfc8620#section-5.5)). How the two numbers travel on the page is [ADR-0095](./0095-enumeration-total-on-every-page.md)'s |
| Listings and the metadata read reach the trash and spam | A message in the trash or marked as spam is still in the mailbox, carrying `Trash` or `Spam`, so a thread listing returns it as it returns any other mail, and the metadata read returns it by its identifier. Delta sync's gap recovery reads a stored message that the listing left out and the metadata read no longer returns as removed ([ADR-0105](../data/0105-a-cursor-gap-is-recovered-from-the-last-cursors-write-time.md)). Gmail lists them by setting `includeSpamTrash` |
| `current_cursor` in the port | Delta sync needs a first cursor to start from, which Gmail's profile `historyId` and JMAP's `state` each supply |
| `ensure_label` in the port | Reorg creates taxonomy; without it each adapter invents its own create-if-missing |
| Batched mutation ops | Gmail `batchModify` and JMAP `Email/set` both batch natively |
| Only a label an op adds must exist | Removing a label a message does not carry, or one the account does not have, changes nothing and succeeds, so a retried batch never fails on work already done. Adding an absent label is refused, because it would mean creating taxonomy outside `ensure_label` |
| `auth_results` in metadata | Carried as message metadata. [ADR-0004](../classification/0004-sender-list-decides.md) lets the policy list alone decide sender class, so classification does not read it |
| `account_id` everywhere | Multi-account correctness enforced by type, not convention |
| `rate_profile()` on the port | Cost models are provider knowledge; see [ADR-0023](../operability/0023-adapter-declares-cost.md) |

This is what each adapter compiles, per concern.

| Concern | Gmail | JMAP |
| --- | --- | --- |
| Delta sync | `history.list` from `historyId`; gap → resync | `Email/changes` from `state`; `cannotCalculateChanges` → resync |
| Full enumeration | `messages.list` + `pageToken` | `Email/query` position/anchor |
| Enumeration total | `messagesTotal` on `users.getProfile` | `total` on `Email/query` with `calculateTotal` |
| Metadata fetch | `format=FULL` with a `fields` mask naming no `body` — **provider guarantees no body** | `Email/get` with explicit `properties`, omitting `bodyValues` |
| Labels | flat IDs | mailbox tree; adapter flattens to paths |
| Batch | `batchModify`, 1000 ids | `Email/set` multi-update |

Gmail's field mask is an asset, because Google filters the response to the fields it names, so on
the enumeration path the provider itself guarantees no body crosses the wire, making gate-side
redaction defense-in-depth rather than sole defense. `format=METADATA` would give the same
guarantee but returns only headers, with no MIME parts and so no attachment names, which the port
promises. The mask names each part's type and file name to a fixed nesting depth, so an attachment
nested deeper than that depth is not seen. The contract is shaped so adapters can exploit such
guarantees wherever a provider offers them.

## Alternatives considered

- **Expose the richer provider's API and emulate on the other.** No case was tabled for it.
  Rejected: Gmail-shaped concepts would spread above the boundary, and every emulation gap becomes a
  behavioral difference the gate and tools must know about, which is the exact provider-awareness
  the port exists to prevent.
- **Pass provider-native query strings through.** No case was tabled for it. Rejected: query syntax
  in the agent's vocabulary couples the client surface to one backend and makes queries unanalyzable
  by the mediator.
- **The enumeration's total reported in pages rather than items.** The case for it is one field
  rather than two, with the page size kept private to the adapter, which would divide its own count
  by it. Rejected in favour of the item count above.
- **One combined `get_message` returning metadata plus optional body.** No case was tabled for it.
  Rejected: it makes "does this call carry content" a runtime question. Two methods make it a
  structural one.

## Consequences

- The contract is the intersection both backends can honour, normalized. A canonical operation one
  backend cannot express is a contract bug, not an adapter branch.
- Adapters are deliberately dumb, taking full data in with no redaction responsibility and no
  policy knowledge.
- Only a second mail adapter proves the contract. If that adapter forces a change above the port,
  the contract was wrong.
