# 0001. The redaction matrix: metadata survives restriction, content never does

**Status:** Accepted ·
**Pillar:** [Sensitivity is two independent axes](../../../DESIGN.md#sensitivity-is-two-independent-axes) ·
**Serves:** [C1](../../../USE_CASES.md#c1--metadata-always-visible), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

The invariant demands both halves at once, full organizational visibility and zero sensitive
content. Between "everything visible" and "body denied" sit fields that are partly content, which
are subjects, snippets and attachment filenames. Each needs an explicit disposition per sensitivity
state, or the boundary gets drawn ad hoc at implementation time.

## Decision

The Redaction Gate applies one matrix to every message.

| Field | Normal | Restricted sender | MFA code | Login link |
| --- | --- | --- | --- | --- |
| Sender, thread, date, labels, flags | visible | visible | visible | visible |
| **Subject** | visible | **visible** | visible, **code masked** | visible |
| Snippet | visible | **null** | **null** | **null** |
| Attachment filenames | visible | type only | type only | type only |
| **Body** | visible | **denied** | **denied** | **denied** |

These rules carry it.

- **Subjects survive restriction.** `"Overdraft notice — action required"` reaching the agent is a
  feature, because the agent can escalate to the operator without reading a word of the body.
- **Subjects do not survive an MFA code.** `"Your code is 419283"` → `"Your code is ██████"`. The
  code is payload, not context, and this applies to restricted senders too. Skipping the body scan
  ([ADR-0008](./0008-restricted-senders-are-never-scanned.md)) never skips subject masking.
- **Any content flag denies the body outright.** A login link is an account-takeover primitive, and
  no partial-release story is worth the complexity.
- **Snippets and attachment filenames are body-derived**, so they follow the body, not the subject.
- **Axes compose by union.** Restrictions accumulate and never cancel.

The matrix's inputs are carried as one typed, immutable value.

```python
class SenderClass(Enum):
    NORMAL     = "normal"
    RESTRICTED = "restricted"

class ContentFlag(Enum):
    MFA_CODE   = "mfa_code"
    LOGIN_LINK = "login_link"

class ScanState(Enum):
    PENDING            = "pending"              # not yet reached
    SCANNED            = "scanned"              # verdict recorded
    SKIPPED_RESTRICTED = "skipped_restricted"   # never scanned: sender restricted
    SKIPPED_GATE       = "skipped_gate"         # never scanned: accepted risk

@dataclass(frozen=True)
class Sensitivity:
    sender_class:  SenderClass
    class_rule_id: str | None               # the policy rule that set sender_class, if one did
    content_flags: frozenset[ContentFlag]
    rule_ids:      tuple[str, ...]          # the content rules that set content_flags
    scan_state:    ScanState
```

Concretely, this is what the agent receives for an unremarkable message, shown first for contrast.

```json
{ "account_id": "personal", "thread_id": "t_88f2",
  "subject": "Re: dinner Saturday",
  "from": {"email": "sam@example.com", "display_name": "Sam"},
  "date": "2026-07-21T18:04:00Z", "labels": ["INBOX"],
  "snippet": "sounds good, I'll book the table for 7…",
  "sensitivity": {"sender_class": "normal", "content_flags": [],
                  "scan_state": "scanned"},
  "body_available": true,
  "allowed_mutations": ["label", "move", "archive", "trash", "spam"] }
```

And this is what it receives for a restricted sender.

```json
{ "account_id": "personal", "thread_id": "t_91c0",
  "subject": "Overdraft notice — action required",
  "from": {"email": "alerts@fidelity.com", "display_name": "Fidelity"},
  "date": "2026-07-20T09:12:00Z", "labels": ["INBOX", "Finance"],
  "has_attachments": true, "attachment_types": ["pdf"],
  "snippet": null,
  "sensitivity": {"sender_class": "restricted",
                  "class_rule_id": "financial.brokerage.fidelity",
                  "content_flags": [], "rule_ids": [],
                  "scan_state": "skipped_restricted"},
  "body_available": false,
  "allowed_mutations": ["label", "move"],
  "note": "Metadata only. Body access denied by policy and cannot be granted by request." }
```

The subject is actionable, so the agent can flag this for attention and file it correctly without
reading anything.

## Alternatives considered

- **Withhold subjects for restricted senders.** No case was tabled for it. Rejected: it converts
  restricted mail from "organizable but unreadable" into a pile of anonymous entries, undermining
  the visibility half of the invariant. The agent could no longer triage or escalate what it cannot
  name.
- **Partial body release for content-flagged messages** (redact the code, serve the rest). No case
  was tabled for it. Rejected: every partial-release mechanism is a new leak surface, and the value
  of the remaining body text does not cover the complexity.
- **Hide restricted messages entirely.** No case was tabled for it. Rejected outright, because it
  falsifies [C1](../../../USE_CASES.md#c1--metadata-always-visible) by construction.

## Consequences

- Metadata exposure is a deliberate, accepted position, not an oversight. Subjects, sender
  identities, and traffic patterns of restricted mail are visible to the agent, mitigated only
  where the metadata itself carries a secret (the masked MFA code). This is the "metadata is
  deliberately exposed" row in [DESIGN.md's Known limits](../../../DESIGN.md#3-known-limits).
- The policy schema keeps a **per-rule subject-masking switch**, off by default.
- The matrix is the Redaction Gate's complete field-level specification. The form a released
  snippet and attachment filename take is
  [ADR-0100](./0100-message-text-without-html-is-released-as-a-literal-code-block.md)'s. What an
  attachment's type is, a word of a closed vocabulary rather than any text the message carries, is
  [ADR-0123](../provider/0123-attachment-types-are-words-of-a-closed-vocabulary.md)'s.
