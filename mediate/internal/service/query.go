package service

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/messages"
)

// The index query is the selection the index reads take, a conjunction of terms over the metadata the
// surface serves, each term optional and an absent term selecting every message (ADR-0108). It is the
// surface's own, compiled to the statements of db/messages, and never the canonical query of the
// Provider Port, which every adapter would have to compile. No term reads a message's scan state, its
// content flags or the class the index stored, so a message whose body is denied is selected like any
// other (ADR-0001). The sender class term is the Redaction Gate's classification under the policy the
// call took (ADR-0002).

// indexQuery is the query argument as a client sends it.
type indexQuery struct {
	After          *string  `json:"after"`
	Before         *string  `json:"before"`
	From           *string  `json:"from"`
	FromDomain     *string  `json:"from_domain"`
	Labels         []string `json:"labels"`
	ExcludedLabels []string `json:"excluded_labels"`
	LabelsWithin   []string `json:"labels_within"`
	Subject        *string  `json:"subject"`
	Unread         *bool    `json:"unread"`
	Starred        *bool    `json:"starred"`
	HasAttachments *bool    `json:"has_attachments"`
	SenderClass    *string  `json:"sender_class"`
}

// queryTerms are the names of the index query's terms, spelled as its schema declares them.
var queryTerms = []string{
	"after", "before", "from", "from_domain", "labels", "excluded_labels", "labels_within", "subject",
	"unread", "starred", "has_attachments", "sender_class",
}

// indexQuerySchema is the JSON Schema of the index query.
const indexQuerySchema = `{"type":"object","properties":{` +
	`"after":` + timestampSchema + `,"before":` + timestampSchema + `,` +
	`"from":{"type":"string","minLength":1},"from_domain":{"type":"string","minLength":1},` +
	`"labels":{"type":"array","items":{"type":"string"}},` +
	`"excluded_labels":{"type":"array","items":{"type":"string"}},` +
	`"labels_within":{"type":"array","items":{"type":"string"}},` +
	`"subject":{"type":"string","minLength":1},` +
	`"unread":{"type":"boolean"},"starred":{"type":"boolean"},"has_attachments":{"type":"boolean"},` +
	`"sender_class":{"type":"string","enum":["normal","restricted"]}},` +
	`"additionalProperties":false}`

// queryDescription says what the index query's terms select, for the operations that take it.
const queryDescription = "query selects messages by every term it holds, and an absent query or term selects every message. " +
	"after and before are UTC timestamps ending in Z, after inclusive and before exclusive, and any other offset is refused. " +
	"from is a sender address and from_domain a sender domain, both compared ignoring case. " +
	"labels keeps the messages carrying every label it lists, excluded_labels those carrying none of them, and " +
	"labels_within those all of whose labels it lists, so an empty labels_within keeps the messages with no label. " +
	"Labels are the values list_labels returns. subject keeps the messages whose subject, as served and masked, contains it, ignoring case. " +
	"unread, starred and has_attachments keep the messages whose state equals the value given, and a message not marked read is unread. " +
	"sender_class keeps the messages whose sender the policy in force classifies as normal or restricted. " +
	"A message whose body is denied is selected like any other."

// selection is an index query read and checked, as the statements take it, and its canonical text,
// which a cursor is bound to. The sender class term is not yet resolved to addresses.
type selection struct {
	params messages.SearchSummaryParams
	class  *string
	text   string
}

// readQuery reads the query argument. A null or missing query selects every message. A term the
// schema does not declare, one named in another case, a null term, an empty address, domain or
// subject, a sender class outside the two, and a timestamp that is not UTC are refused.
func readQuery(account string, raw json.RawMessage) (selection, error) {
	sel := selection{params: messages.SearchSummaryParams{AccountID: account}}
	var q indexQuery
	if len(raw) > 0 {
		if err := strictObject(raw, &q, "query", queryTerms...); err != nil {
			return selection{}, err
		}
	}
	if q.After != nil {
		t, err := parseUTC("query.after", *q.After)
		if err != nil {
			return selection{}, err
		}
		sel.params.After = pgtype.Timestamptz{Time: t, Valid: true}
		canonical := formatUTC(t)
		q.After = &canonical
	}
	if q.Before != nil {
		t, err := parseUTC("query.before", *q.Before)
		if err != nil {
			return selection{}, err
		}
		sel.params.Before = pgtype.Timestamptz{Time: t, Valid: true}
		canonical := formatUTC(t)
		q.Before = &canonical
	}
	for name, v := range map[string]*string{"query.from": q.From, "query.from_domain": q.FromDomain, "query.subject": q.Subject} {
		if v != nil && *v == "" {
			return selection{}, Refuse(name + " may not be empty")
		}
	}
	if q.From != nil {
		sel.params.FromEmails = []string{*q.From}
	}
	if q.FromDomain != nil {
		sel.params.FromDomains = []string{*q.FromDomain}
	}
	sel.params.Labels = q.Labels
	sel.params.ExcludedLabels = q.ExcludedLabels
	sel.params.LabelsWithin = q.LabelsWithin
	if q.Subject != nil {
		sel.params.SubjectPatterns = []string{"%" + likeEscape(*q.Subject) + "%"}
	}
	sel.params.Unread = oneOf(q.Unread)
	sel.params.Starred = oneOf(q.Starred)
	sel.params.HasAttachments = oneOf(q.HasAttachments)
	if q.SenderClass != nil {
		if *q.SenderClass != "normal" && *q.SenderClass != "restricted" {
			return selection{}, Refuse("query.sender_class must be normal or restricted")
		}
		sel.class = q.SenderClass
	}
	text, err := json.Marshal(q)
	if err != nil {
		return selection{}, err
	}
	sel.text = string(text)
	return sel, nil
}

// strictObject decodes the JSON object raw, the argument named name, into v, refusing a key none of
// names spells exactly and a null value, as arguments does for an operation's top-level arguments.
func strictObject(raw json.RawMessage, v any, name string, names ...string) error {
	if string(raw) == "null" {
		return Refuse(name + " may not be null")
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return Refuse(name + " is not an object")
	}
	for key, value := range members {
		if !slices.Contains(names, key) {
			return Refuse(name + " holds a term the operation does not take")
		}
		if strings.TrimSpace(string(value)) == "null" {
			return Refuse(name + "." + key + " may not be null")
		}
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return Refuse(name + " does not match its schema")
	}
	return nil
}

// likeEscape escapes the characters a LIKE pattern gives a meaning, so the subject term matches its
// text literally.
func likeEscape(text string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
}

// oneOf returns the one value a boolean term takes, or nil for an absent term.
func oneOf(b *bool) []bool {
	if b == nil {
		return nil
	}
	return []bool{*b}
}

// classes are the sender classes the policy a call took gives the senders the account's messages hold,
// decided as the Redaction Gate decides a message's (ADR-0002).
type classes struct {
	// normal are the addresses classified normal. A statement counts every other address as
	// restricted, so an address the call did not classify fails closed (ADR-0108).
	normal []string
	// domains holds, for each sender domain, whether any of its addresses is restricted.
	domains map[string]bool
}

// classifySenders classifies every sender address rows hold under p.
func classifySenders(rows []messages.SenderAddressesRow, p policy.Composed, l classify.Lookups) classes {
	c := classes{normal: []string{}, domains: map[string]bool{}}
	for _, r := range rows {
		restricted := classify.Classify(p, r.FromEmail, l).Class().Restricted()
		if !restricted {
			c.normal = append(c.normal, r.FromEmail)
		}
		c.domains[strings.ToLower(r.FromDomain)] = c.domains[strings.ToLower(r.FromDomain)] || restricted
	}
	return c
}

// resolve returns the selection's parameters with the sender class term resolved to the addresses c
// classifies normal.
func (s selection) resolve(c classes) messages.SearchSummaryParams {
	p := s.params
	if s.class != nil {
		p.Restricted = []bool{*s.class == "restricted"}
		p.NormalSenders = c.normal
	}
	return p
}

// needsClasses reports whether reading the selection needs the senders classified.
func (s selection) needsClasses() bool { return s.class != nil }
