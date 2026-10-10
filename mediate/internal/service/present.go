package service

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// stored is one message's row in the index, the metadata the surface serves and the content flags and
// scan state the Redaction Gate decides from. The index holds no body, no snippet and no attachment
// filename (ADR-0016), so no message the surface serves carries one.
type stored struct {
	MessageID       string
	ThreadID        string
	FromEmail       string
	FromName        *string
	Subject         *string
	SentAt          time.Time
	Labels          []string
	Flags           []byte
	HasAttachments  bool
	AttachmentMedia []mail.AttachmentMedia
	ContentFlags    []string
	ScanState       string
}

// message is one message as the client surface serves it, the fields ADR-0001's matrix shows for
// every sensitivity state. The subject is served as stored, since subject masking masks it at rest
// (ADR-0003). Attachments are named by type only, each type a word the mapping derives from the
// stored media, never a stored value (ADR-0123).
type message struct {
	AccountID       string          `json:"account_id"`
	MessageID       string          `json:"message_id"`
	ThreadID        string          `json:"thread_id"`
	Subject         *string         `json:"subject"`
	From            sender          `json:"from"`
	Date            string          `json:"date"`
	Labels          []string        `json:"labels"`
	Flags           json.RawMessage `json:"flags"`
	HasAttachments  bool            `json:"has_attachments"`
	AttachmentTypes []string        `json:"attachment_types"`
	Sensitivity     shownState      `json:"sensitivity"`
	BodyAvailable   bool            `json:"body_available"`
}

type sender struct {
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
}

// shownState is the message's sensitivity as the gate decided it. The sender class is the gate's
// classification against the policy in force, never the class the index stored (ADR-0002).
type shownState struct {
	SenderClass  string   `json:"sender_class"`
	ContentFlags []string `json:"content_flags"`
	ScanState    string   `json:"scan_state"`
}

// storedFlags reads the content flags the index stores. A value the schema does not name reads as
// both flags, the most restrictive state, so an unreadable row is never released (ADR-0042).
func storedFlags(names []string) sensitivity.ContentFlags {
	mfa, login := false, false
	for _, n := range names {
		switch n {
		case "mfa_code":
			mfa = true
		case "login_link":
			login = true
		default:
			return sensitivity.Flags(true, true)
		}
	}
	return sensitivity.Flags(mfa, login)
}

// storedScanState reads the scan state the index stores. A value the schema does not name reads as
// pending, which the gate never releases.
func storedScanState(name string) sensitivity.ScanState {
	switch name {
	case "scanned":
		return sensitivity.Scanned()
	case "skipped_gate":
		return sensitivity.SkippedGate()
	case "skipped_restricted":
		return sensitivity.SkippedRestricted()
	default:
		return sensitivity.Pending()
	}
}

// present returns the message m of account as the client surface serves it, under the Redaction
// Gate's decision against the account policy p (ADR-0001, ADR-0002).
func present(account string, m stored, p policy.Composed, l classify.Lookups) message {
	flags, scan := storedFlags(m.ContentFlags), storedScanState(m.ScanState)
	v := redact.Decide(p, m.FromEmail, l, flags, scan)
	class := "normal"
	if v.Sender().Class().Restricted() {
		class = "restricted"
	}
	shownFlags := []string{}
	if flags.MFACode() {
		shownFlags = append(shownFlags, "mfa_code")
	}
	if flags.LoginLink() {
		shownFlags = append(shownFlags, "login_link")
	}
	out := message{
		AccountID:       account,
		MessageID:       m.MessageID,
		ThreadID:        m.ThreadID,
		Subject:         m.Subject,
		From:            sender{Email: m.FromEmail, DisplayName: m.FromName},
		Date:            formatUTC(m.SentAt),
		Labels:          nonNil(m.Labels),
		Flags:           json.RawMessage(`{}`),
		HasAttachments:  m.HasAttachments,
		AttachmentTypes: types(m.AttachmentMedia),
		Sensitivity:     shownState{SenderClass: class, ContentFlags: shownFlags, ScanState: scan.String()},
		BodyAvailable:   v.ReleasesBody(),
	}
	if json.Valid(m.Flags) && len(m.Flags) > 0 && m.Flags[0] == '{' {
		out.Flags = m.Flags
	}
	return out
}

// types returns the types of a message's attachments, derived from their stored media by the one
// mapping, each word once and sorted, so no stored value is served (ADR-0123).
func types(media []mail.AttachmentMedia) []string {
	out := []string{}
	for _, t := range mail.AttachmentTypes(media) {
		out = append(out, string(t))
	}
	return out
}

// nonNil returns s, or an empty list for nil, so a list is never written as null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return slices.Clone(s)
}

// messageSchema is the JSON Schema of a served message.
const messageSchema = `{"type":"object","properties":{` +
	`"account_id":{"type":"string"},"message_id":{"type":"string"},"thread_id":{"type":"string"},` +
	`"subject":{"type":["string","null"]},` +
	`"from":{"type":"object","properties":{"email":{"type":"string"},"display_name":{"type":["string","null"]}},"required":["email","display_name"]},` +
	`"date":` + timestampSchema + `,` +
	`"labels":{"type":"array","items":{"type":"string"}},` +
	`"flags":{"type":"object"},` +
	`"has_attachments":{"type":"boolean"},` +
	`"attachment_types":{"type":"array","items":{"type":"string"}},` +
	`"sensitivity":{"type":"object","properties":{` +
	`"sender_class":{"type":"string","enum":["normal","restricted"]},` +
	`"content_flags":{"type":"array","items":{"type":"string","enum":["mfa_code","login_link"]}},` +
	`"scan_state":{"type":"string","enum":["pending","scanned","skipped_gate","skipped_restricted"]}},` +
	`"required":["sender_class","content_flags","scan_state"]},` +
	`"body_available":{"type":"boolean"}},` +
	`"required":["account_id","message_id","thread_id","subject","from","date","labels","flags","has_attachments","attachment_types","sensitivity","body_available"]}`
