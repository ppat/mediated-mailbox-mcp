# 0019. Asymmetric mutation: organize everything, dispose by sensitivity, permanently delete nothing

**Status:** Accepted ·
**Pillar:** [Sensitivity is two independent axes](../../../DESIGN.md#sensitivity-is-two-independent-axes) ·
**Serves:** [A1](../../../USE_CASES.md#a1--asymmetric-mutation), [A2](../../../USE_CASES.md#a2--no-destructive-action-on-sensitive-mail), [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

"Organize my inbox" implies mutation, and mutation carries risks reading does not: losing an IRS
notice to spam is the concrete harm to guard against. But collapsing "cannot read" into "cannot
touch" would gut organizational capability — organizing must work across the whole mailbox,
restricted messages included, or whole-mailbox visibility is undermined.

## Decision

The Mutation Authorizer applies this matrix to every operation:

| Verb | Normal | Restricted sender | Content-flagged (normal sender) |
| --- | --- | --- | --- |
| `label` / `unlabel` | ✅ | ✅ | ✅ |
| `move` | ✅ | ✅ | ✅ |
| `mark_read` / `star` | ✅ | ✅ | ✅ |
| `archive` | ✅ | ❌ | ✅ |
| `trash` | ✅ | ❌ | ✅ |
| `spam` / `junk` | ✅ | ❌ | ✅ |
| `delete` (permanent) | ❌ **never** | ❌ | ❌ |

The rules behind the matrix:

- **Sender class governs mutation; content flags govern readability.** The axes compose rather
  than override: a content-flagged message from a normal sender inherits the sender's full
  mutation rights — an expired MFA mail is exactly the clutter the agent should archive, even
  though its body was withheld.
- **Restricted means organize-only.** Label and move, nothing that removes a message from view:
  no archive, trash, or spam.
- **Permanent delete exists nowhere.** It is absent from the client surface *and* from the granted
  token capability ([ADR-0011](../provider/0011-gmail-auth-installed-app-oauth.md)) — the
  guarantee is structural twice over, not a policy check.
- **Batches are all-or-nothing per authorization class.** A batch mixing normal and restricted
  messages with a disposal verb fails entirely, with a clear error, rather than applying to the
  allowed subset and leaving a surprising partial state. This is one case of the general rule —
  any validation failure fails the batch whole, before the first write
  ([ADR-0032](./0032-whole-batch-validation.md)).

## Alternatives considered

- **One axis: sensitive means untouchable.** Rejected: restricted mail could not be filed, which
  undermines whole-mailbox organization — the design's purpose.
- **Disposal rights by content flag as well as sender class.** Rejected: it would forbid archiving
  expired-code clutter for no protective gain — the flag guards the *content*, which disposal does
  not reveal.
- **Best-effort batches (apply what is authorized, report the rest).** Rejected: partial
  application of a mixed batch is a surprising state that reads as success; a whole-batch failure
  is legible and retryable after the caller splits it deliberately.
- **A `mute` verb.** Rejected because Gmail's API cannot mute a thread. It refuses the `MUTED`
  label as invalid, and [ADR-0010](../provider/0010-one-provider-port.md) counts an operation one
  backend cannot express as a bug in the contract. Archive takes a message out of view, though
  unlike mute it does not keep later replies out of the inbox. If a provider's API gains mute, mute
  returns as a new verb.

## Consequences

- The authorizer is a peer of the Redaction Gate, not part of it — read-rights and write-rights
  are independent checks, and merging them would force sensitivity to mean one thing when it means
  two.
- The authorizer runs on every mutation path, including the application of an approved reorg plan
  ([ADR-0020](./0020-reorg-plan-approve-apply-rollback.md)) — approval never overrides the matrix.
