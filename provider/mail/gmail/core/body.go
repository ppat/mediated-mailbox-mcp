// Package core holds the Gmail adapter's decisions on a message body, made from Gmail's part tree
// and returned as values: which part is the rendered part, whether it came inline, is held apart
// within the bound, or is refused, the text a part's data decodes to, each read's cap and the refusal
// of an answer past it (ADR-0040, ADR-0131, ADR-0134). The adapter enacts each decision. It sends
// the requests, reads each answer up to its cap, issues the part token and builds the port's errors. The media-type parser and the charset label table
// arrive as parameters, since both reach the operating system through what they import.
package core

import (
	"encoding/base64"
	"strings"

	"golang.org/x/text/encoding"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Part is one part of a message as a messages.get answer carries it, the message's payload being
// the root.
type Part struct {
	MimeType string   `json:"mimeType"`
	Filename string   `json:"filename"`
	Headers  []Header `json:"headers"`
	Body     Body     `json:"body"`
	Parts    []Part   `json:"parts"`
}

// Header is one header of a part.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Body is a part's body. A part Gmail sends inline carries its data, and a part it holds apart
// carries a reference to it instead.
type Body struct {
	AttachmentID string `json:"attachmentId"`
	Size         int64  `json:"size"`
	Data         string `json:"data"`
}

// HeaderValue returns the value of the first header named name, ignoring case, or empty.
func HeaderValue(headers []Header, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// The caps on the answers to a body's two reads, derived from the port's bound on the rendered part
// (ADR-0131). The message read's answer carries every part Gmail sends inline, the rendered part and
// its alternative among them, in base64, with the headers and the part tree beside them, so it is
// capped at four thirds of two parts at the bound plus 256 KiB. The part read's answer carries one
// part in base64, so it is capped at four thirds of the bound plus 64 KiB. An answer past its cap
// is PastCap, never truncated and misread.
const (
	MessageReadCap = 4*(2*mail.MaxBodyBytes)/3 + 256<<10
	PartReadCap    = 4*mail.MaxBodyBytes/3 + 64<<10
)

// Slot is the field of the port's body a rendered part fills. Its values are the words a part token
// carries.
type Slot string

// The two slots.
const (
	HTML Slot = "html"
	Text Slot = "text"
)

// Act is what the adapter does with a message's body.
type Act int

// The acts.
const (
	// Empty returns the empty body, since the message has no rendered part.
	Empty Act = iota + 1
	// Inline returns the text that came with the decision.
	Inline
	// Fetch answers the call with the part held apart, for a second call to read.
	Fetch
	// Stop returns no body, for the decision's Problem.
	Stop
)

// Problem is why a body is not returned.
type Problem int

// The problems.
const (
	// None is no problem.
	None Problem = iota
	// NotBase64 is data that does not decode, which is Gmail's answer failing.
	NotBase64
	// PastBound is a part, or its decoded text, larger than the port's bound.
	PastBound
	// ContentTypeUnparsed is a part held apart whose Content-Type is present and does not parse, so
	// the charset it names cannot be read and may be any.
	ContentTypeUnparsed
	// CharsetRefused is a part held apart in a charset the label table does not name, or one it maps
	// to an encoding that would read the bytes as other text than they hold.
	CharsetRefused
	// Undecodable is data its charset's decoder fails on.
	Undecodable
	// PastCap is an answer to a body read larger than the read's cap.
	PastCap
)

// Refused reports whether p refuses the body for good, as the port's refusal, rather than being the
// provider's failure, which a later call may not repeat.
func (p Problem) Refused() bool {
	switch p {
	case PastBound, ContentTypeUnparsed, CharsetRefused, Undecodable, PastCap:
		return true
	case None, NotBase64:
		return false
	}
	return false
}

// String names p for an error message.
func (p Problem) String() string {
	switch p {
	case None:
		return "no problem"
	case NotBase64:
		return "its data is not base64"
	case PastBound:
		return "it passes the bound"
	case ContentTypeUnparsed:
		return "its Content-Type does not parse"
	case CharsetRefused:
		return "its charset cannot be decoded"
	case Undecodable:
		return "its data does not decode from its charset"
	case PastCap:
		return "its answer passes the read's cap"
	}
	return "an unknown problem"
}

// Decision is what the adapter does with a message's body. It names the rendered part and never
// holds its data, which an Inline decision's text carries beside it.
type Decision struct {
	Act  Act
	Slot Slot
	// Problem is why a Stop decision returns no body.
	Problem Problem
	// Attachment is Gmail's reference to the part a Fetch decision holds apart, and Charset the
	// charset its Content-Type declares, empty for none.
	Attachment string
	Charset    string
}

// ParseMediaType reads a Content-Type value into its media type and parameters, as mime's
// ParseMediaType does.
type ParseMediaType func(value string) (mediaType string, params map[string]string, err error)

// MessageBody decides on a message's body from its payload. The rendered part is the first HTML part
// without a file name that holds content, depth first, and otherwise the first such text part. A
// part holds content when it carries data or a reference to data held apart, so an empty part is
// passed over. A part that came inline is decoded and returned with an Inline decision, since Gmail
// sends inline data as UTF-8. A part held apart is fetched when its declared size is within the bound
// and its Content-Type, if it has one, parses, and refused otherwise, so nothing past the bound is
// downloaded (ADR-0131, ADR-0134).
func MessageBody(payload Part, parse ParseMediaType) (Decision, string) {
	p, slot, ok := rendered(payload)
	if !ok {
		return Decision{Act: Empty}, ""
	}
	if p.Body.Data == "" {
		if p.Body.Size > mail.MaxBodyBytes {
			return stop(slot, PastBound), ""
		}
		charset, problem := declaredCharset(p, parse)
		if problem != None {
			return stop(slot, problem), ""
		}
		return Decision{Act: Fetch, Slot: slot, Attachment: p.Body.AttachmentID, Charset: charset}, ""
	}
	data, problem := decodeData(p.Body.Data)
	if problem != None {
		return stop(slot, problem), ""
	}
	if len(data) > mail.MaxBodyBytes {
		return stop(slot, PastBound), ""
	}
	return Decision{Act: Inline, Slot: slot}, string(data)
}

// Labels is a charset label table, the WHATWG Encoding Standard's in the adapter: Get maps a label to
// its encoding and Name gives an encoding's canonical name.
type Labels struct {
	Get  func(label string) (encoding.Encoding, error)
	Name func(e encoding.Encoding) (string, error)
}

// PartText decodes the data of a part held apart, which Gmail sends in the charset the part declared,
// to UTF-8. The data and the decoded text are each held to the bound, so a part whose declared size
// understated its data is refused too. A part that declares no charset is US-ASCII, MIME's default,
// which the standard reads as windows-1252. A label the table does not name is refused, and so is
// one it maps to the replacement encoding, which decodes to nothing but a replacement character, to
// UTF-16, which MIME does not allow in a text part (RFC 2781 Appendix A) and whose byte order would
// be guessed, or to x-user-defined, whose bytes above 0x7F decode to private-use characters that
// carry no text. Bytes the charset does not define decode to U+FFFD, so the text is always valid
// UTF-8 (ADR-0134).
func PartText(data, charset string, labels Labels) (string, Problem) {
	raw, problem := decodeData(data)
	if problem != None {
		return "", problem
	}
	if len(raw) > mail.MaxBodyBytes {
		return "", PastBound
	}
	if charset == "" {
		charset = "us-ascii"
	}
	enc, err := labels.Get(charset)
	name := ""
	if err == nil {
		name, err = labels.Name(enc)
	}
	switch {
	case err != nil, name == "replacement", name == "utf-16le", name == "utf-16be", name == "x-user-defined":
		return "", CharsetRefused
	}
	out, err := enc.NewDecoder().Bytes(raw)
	if err != nil {
		return "", Undecodable
	}
	if len(out) > mail.MaxBodyBytes {
		return "", PastBound
	}
	return string(out), None
}

// Fill returns the port's body of message for account, holding text in slot.
func Fill(account, message string, slot Slot, text string) mail.MessageBody {
	b := mail.MessageBody{AccountID: account, MessageID: message}
	if slot == HTML {
		b.HTML = text
	} else {
		b.Text = text
	}
	return b
}

func stop(slot Slot, p Problem) Decision {
	return Decision{Act: Stop, Slot: slot, Problem: p}
}

// rendered returns the rendered part and the slot it fills, and false when there is none.
func rendered(p Part) (Part, Slot, bool) {
	if html, ok := first(p, "text/html"); ok {
		return html, HTML, true
	}
	if text, ok := first(p, "text/plain"); ok {
		return text, Text, true
	}
	return Part{}, "", false
}

// first returns the first part of mediaType without a file name that holds content, depth first.
func first(p Part, mediaType string) (Part, bool) {
	if p.Filename == "" && strings.EqualFold(p.MimeType, mediaType) && (p.Body.Data != "" || p.Body.AttachmentID != "") {
		return p, true
	}
	for _, c := range p.Parts {
		if found, ok := first(c, mediaType); ok {
			return found, true
		}
	}
	return Part{}, false
}

// declaredCharset returns the charset a part's Content-Type declares, empty when the part has no
// Content-Type or one naming no charset, and ContentTypeUnparsed for one that is present and does
// not parse.
func declaredCharset(p Part, parse ParseMediaType) (string, Problem) {
	value := HeaderValue(p.Headers, "Content-Type")
	if value == "" {
		return "", None
	}
	_, params, err := parse(value)
	if err != nil {
		return "", ContentTypeUnparsed
	}
	return params["charset"], None
}

// decodeData decodes a part's data, which Gmail sends in URL-safe base64.
func decodeData(data string) ([]byte, Problem) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(data, "="))
	if err != nil {
		return nil, NotBase64
	}
	return b, None
}
