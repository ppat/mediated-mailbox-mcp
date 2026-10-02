# 0108. The index reads select messages by an index query of the client surface's own, a conjunction of terms over the served metadata, never by the Provider Port's canonical query

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [G2](../../../USE_CASES.md#g2--historical-understanding), [C1](../../../USE_CASES.md#c1--metadata-always-visible)

## Context

[G1](../../../USE_CASES.md#g1--whole-mailbox-visibility) is falsified by an agent that cannot answer
a structural question needing only metadata, "how many unread financial notices, how many with
attachments, oldest date", for restricted threads, and by a restricted thread missing from counts,
groupings or search results because its body is denied.
[G2](../../../USE_CASES.md#g2--historical-understanding) is falsified by an agent unable to derive
label distributions, senders with no label, and which senders account for most unfiled volume.
Every one of those questions selects messages from the index by their metadata.

[ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md) makes a
read that takes structured input a structured read, posted to a `:search` route with its input in
the body. Search there takes a structured query and never a provider's query string, citing
[ADR-0010](../provider/0010-one-provider-port.md), whose concern is that no provider's syntax
reaches the agent. Which structured query the index reads take is this record's.

The Provider Port's canonical query (`core/mail/query.go`) selects by date, label and sender
address, and by their conjunction. Every node is one that Gmail's search and JMAP's filter both
express exactly, because each adapter compiles the tree, and a node one backend cannot express is a
contract bug ([P1](../../../USE_CASES.md#p1--one-contract)). It has no term for whether a message
was read, whether it carries attachments, or its sender class. The sender class is not a provider
property at all. It is the Redaction Gate's classification under the policy in force
([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)), and only the index and the policy hold
what it is decided from.

The pillar [Everything above the port speaks canonical](../../../DESIGN.md#everything-above-the-port-speaks-canonical)
keeps provider concepts, label identifiers and query syntax among them, below the port. A selection
the client surface makes over the index by canonical metadata keeps to it, since it names no
provider concept and no provider receives it. The pillar does not make the port's query the only
selection above the port.

The index keeps every label a message carries, the canonical `INBOX`, `TRASH` and `SPAM` and
whatever other system labels an adapter maps, such as Gmail's `SENT` and `DRAFT`. So "a message with
no label" and "an unfiled message" are not the same set, and which labels count as filing is the
client's question, not the surface's.

## Decision

- **The index reads take an index query, the client surface's own selection over the metadata it
  serves.** It is an object of optional terms, all of which a message must satisfy, and an absent
  query or term selects every message. It is compiled to the statements of the data-access
  subsection for messages and never handed to an adapter.

  | Term | Selects the messages |
  | --- | --- |
  | `after`, `before` | Sent at or after, or before, a UTC timestamp ending in `Z`. Any other offset is refused, as [ADR-0033](./0033-utc-only-timestamps.md) requires |
  | `from` | Whose sender address equals the one given, ignoring case |
  | `from_domain` | Whose sender domain equals the one given, ignoring case |
  | `labels` | Carrying every label listed |
  | `excluded_labels` | Carrying none of the labels listed |
  | `labels_within` | All of whose labels are listed. The empty list selects the messages with no label, and a list of the labels a client does not count as filing selects the unfiled ones |
  | `subject` | Whose subject, as stored and served, contains the text given, ignoring case and taking the text literally |
  | `unread`, `starred`, `has_attachments` | Whose state equals the value given. A message not marked read is unread |
  | `sender_class` | Whose sender the Redaction Gate classifies `normal` or `restricted` under the policy the call took |

- **No term reads a message's scan state, its content flags or the class the index stored.** A
  message whose body is denied is selected like any other, which is
  [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)'s third falsifier held by the query's
  shape.
- **The sender class term is decided as the Redaction Gate decides a message's sender class.** The
  service layer classifies every sender address the account's messages hold under the policy the
  call took once, the policy the messages it serves are presented under, and passes the addresses it
  classified normal into the statement. A domain listed after its messages were stored as normal
  selects as restricted on the next call ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)).
- **An address the call did not classify counts as restricted.** The statement treats every address
  outside the normal set as restricted, so a set and a statement that ever disagree fail closed, as
  the classifier fails closed on an address it cannot read
  ([Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere)). The classification and
  every statement of a call also read one snapshot of the index
  ([ADR-0109](./0109-the-index-is-read-through-search-count-and-the-sender-listing.md)), so the two
  agree.
- **The subject term searches the subject the index stores, which subject masking has already
  masked** ([ADR-0003](../redaction/0003-subject-masking.md)). A masked code is not in the index, so
  no query can find a message by it.
- **Every term travels in the request body**, inside the structured read's arguments, so no search
  text reaches a URL ([ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)'s
  R9).
- **The terms are those of the metadata the client surface serves**
  ([ADR-0001](../redaction/0001-redaction-matrix.md)). Nothing body-derived is a term, because the
  index holds nothing body-derived ([ADR-0016](../data/0016-schema.md)).
- **Each value a term takes is discoverable on the surface**
  ([ADR-0035](./0035-required-identifiers-are-discoverable.md)). Labels come from the labels listing,
  addresses from any served message, and domains from the sender listing.

## Alternatives considered

- **Extend the canonical query with the missing terms.** For it, one query language above the port,
  and search through the index and search through a provider would speak the same tree. Against it,
  every adapter would have to compile terms its provider cannot express, which
  [P1](../../../USE_CASES.md#p1--one-contract)'s intersection rule forbids, and the sender class
  would have to reach adapters that know nothing of policy. The port's query exists for the
  workloads that search the provider. The index reads never call a provider.
- **Take the canonical query and add a few index terms beside it.** For it, the shared terms keep
  their port spelling. Against it, one argument would mix two languages with two different
  compilers, and a term moving from one half to the other would change every client's input for no
  gain.
- **A free-text query string with operators.** For it, compact and familiar to an agent that knows
  Gmail's search. Against it, it is a provider's query string in all but name, which
  [ADR-0010](../provider/0010-one-provider-port.md) keeps out of the agent's vocabulary, and a
  parser is a surface the structured object does not need.
- **A fixed set of system labels the surface treats as not filing.** For it, "unfiled" becomes a
  single term. Against it, which labels file a message differs by provider and by operator, and
  naming `SENT`, `DRAFT` or Gmail's categories above the adapter couples the surface to one provider.
  `labels_within` lets the client name the set from the labels listing.
- **The restricted addresses passed into the statement, every other address counting as normal.** For
  it, the set passed is usually the smaller one. Against it, an address the call did not classify,
  such as one written after the classification read, counts as normal, which fails open.
- **The sender class term read from the class the index stored.** For it, one column in the
  statement and no classification per call. Against it, a domain listed after its messages were
  stored would select as normal while the same message is served as restricted, the disagreement
  [ADR-0002](../redaction/0002-fetch-time-re-evaluation.md) rules out, and the mediator holds no grant
  on that column ([ADR-0075](../data/0075-one-runtime-role-per-deployable.md)).

## Consequences

- The index query and the canonical query evolve apart. A term added to one is not owed to the
  other, and a term an adapter could express is added to the canonical query only when a workload
  that searches the provider needs it.
- Every term is a null-guarded parameter of fixed statements, so the set of terms is closed per
  statement, as [ADR-0066](../data/0066-data-access-generated-from-sql.md) shapes the UI's dataset
  filters. A new term is a change to every statement that takes the query.
- The sender class term classifies every distinct sender address of the account on each call that
  uses it. At the corpus [ADR-0016](../data/0016-schema.md) assumes, about a hundred thousand
  messages, that is at most as many addresses, classified in memory. A corpus an order of magnitude
  larger re-argues it.
- A cursor is bound to the query it paged, so a policy change between two pages of a sender class
  selection moves messages in or out of the pages that follow. The next call reads the policy in
  force, as every read does.
- Assumptions about other components. The index stores the subject already masked
  ([ADR-0003](../redaction/0003-subject-masking.md)). A message's flags record `read` and `starred`
  as booleans, as backfill writes them. The classifier decides a sender's class from its address and
  the policy alone ([ADR-0004](../classification/0004-sender-list-decides.md)).
- The controls this record states, that a message whose body is denied stays selected and that the
  sender class term is the policy in force's, are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
