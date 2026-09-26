# 0004. The sender list decides; heuristics only propose

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

Sender classification carries the highest stakes in the system: it is the input that decides
whether a body can ever be released. Three facts constrain the answer:

- The set of sensitive senders is personal and open-ended — the operator adds domains over time.
- The classification must be auditable after the fact: "why was this body released" needs an
  answer better than a model score.
- Institutions send from domains their customers have never heard of (`chase.com` customers
  receive mail from `chasealerts.com`), so a purely manual list goes stale silently.

## Decision

**An explicit, operator-editable list is the only authority on sender class. Heuristics generate
candidates for that list; nothing they emit takes effect without operator confirmation.**

A probabilistic model deciding whether the operator's brokerage is sensitive would be strictly
worse than a list: unauditable, non-reproducible, and silently altered by retraining.

The list is a gazetteer with normalization and suffix matching. It lives in the database as rows,
one per rule ([ADR-0016](../data/0016-schema.md)'s `policy_rules`), and every process takes it as
an immutable snapshot ([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)). A file
form exists for import and export, and it is the form shown here:

```yaml
rules:
  - id: financial.brokerage.fidelity
    domain_suffix: [fidelity.com, fidelity.co.uk, fmr.com]
    class: restricted
  - id: gov.federal.irs
    domain_suffix: [irs.gov]
    class: restricted
  - id: infra.vendor.cloudflare
    domain_suffix: [cloudflare.com, cloudflareclient.com]
    class: restricted
```

Before matching, both the sender's domain and each rule's suffixes are lowercased and
punycode-decoded, and the sender's registrable domain is resolved via the public-suffix list, where
a domain with none is classified restricted. Matching is on domain suffix at label boundaries, so
`alerts.fidelity.com` hits without a separate rule and `notfidelity.com` does not.

**Candidate generation** runs as a periodic job writing to a review queue the operator confirms
through the UI. Confirmation inserts a policy rule row, written by the UI itself in the same
transaction as the candidate's status
([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)):

| Heuristic | Signal | Cost |
| --- | --- | --- |
| **Display-name matching** | `From: "Chase Bank" <x@unlisted.io>` — the strongest available signal | free |
| Registrable-domain clustering | `chase.com` listed → `chasealerts.com` co-occurring, unlisted | cheap |
| Institution keyword in domain | `bank`, `credit`, `capital`, `.gov` | free |
| Transactional pattern | no `List-Id` + `noreply@` + never labeled | free |
| Embedding similarity | vector over (domain tokens, display name, subject distribution) vs. the confirmed-sensitive centroid — one embedding per **sender**, not per message | cheap |

The embedding row is where machine learning earns its place — surfacing candidates from a corpus
nobody would hand-review — and its output is a ranked list for confirmation, never an autonomous
classification.

**Spoofing posture.** The `From` header is forgeable, but note the attack's actual shape: spoofing
*into* the deny list yields more redaction, not less. The real risk is the inverse — the unlisted
co-brand domain. The list alone decides a sender's class. SPF, DKIM and DMARC results and display
names are not inputs to the classification, so an authentication failure can never downgrade a
listed sender and a display name can never make a sender restricted. A display name suggesting a
listed institution is a signal for the display-name heuristic above, which proposes a candidate.

## Alternatives considered

- **ML classifier as the authority.** Rejected on stakes: the highest-consequence decision in the
  system would become unauditable and non-reproducible, and would shift under retraining without
  anyone deciding anything.
- **Static list with no candidate generation.** Rejected: the list decays as institutions add
  sending domains, and the decay is invisible until a leak reveals it. The heuristics exist
  precisely to make staleness observable and cheap to correct.
- **A configuration file as the store, hot-reloaded.** The original decision, and the default
  shape a hot-reload implementation takes. The operator ruled on 2026-09-10 that the policy lives
  in the database, with a file only for import and export. The candidate confirmation is the
  reason that shows. A confirmed candidate becomes a rule by one row written in the same
  transaction as the decision, with nothing to copy into a file and no process to own the copy.
- **Exact-domain matching without normalization.** Rejected: it multiplies rules per institution
  and turns every new subdomain into a silent gap.
- **Authentication results and display names as inputs to classification.** The case for it is
  that mail from an unlisted co-brand domain that fails SPF, DKIM or DMARC and carries a listed
  institution's display name would be restricted at once, rather than released until the operator
  confirms that domain as a candidate. Rejected by the operator on 2026-09-23. The list alone
  decides, and a display name suggesting a listed institution is a signal for the display-name
  heuristic, which proposes the domain as a candidate. That window of release until confirmation is
  the accepted cost.

## Consequences

- Classification is deterministic and reproducible: the same message against the same policy
  always classifies identically, and every classification names the rule that produced it.
- The operator inherits a small standing duty — reviewing the candidate queue — in exchange for
  the guarantee that nothing reclassifies itself.
- Full-history backfill makes the first candidate report comprehensive on day one rather than
  accumulating over months, which is when list staleness would otherwise bite hardest.
- Assumptions about other components: the classifier is a pure core
  ([ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)), so it imports only what
  meets [ADR-0071](../engineering/0071-static-enforcement-toolchain.md)'s conditions for a pure
  core's imports. Neither `golang.org/x/net/idna`, which decodes punycode, nor
  `golang.org/x/net/publicsuffix`, which holds the public-suffix list, meets them. Each caller
  passes punycode decoding and the registrable-domain lookup into the classifier as functions, and
  an address whose domain has no registrable domain is classified restricted.
