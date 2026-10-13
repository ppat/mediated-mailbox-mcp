package gmail

import (
	"fmt"
	"mime"
	"net/url"
	"strings"

	"golang.org/x/text/encoding/htmlindex"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/core"
)

// partPrefix starts every body part the adapter issues.
const partPrefix = tokenPrefix + "p."

// heldApart is a rendered part Gmail holds apart from the message, as a part token names it: the
// message it belongs to, the slot it fills, Gmail's reference to it, and the charset its
// Content-Type declares, since Gmail returns a referenced part in that charset.
type heldApart struct {
	message    string
	slot       core.Slot
	attachment string
	charset    string
}

// bodyPart returns the part token naming h. Gmail hands out a new reference on every read of the
// message, each of which keeps working for a while, which is harmless, since the second call that
// names the part follows the first.
func (h heldApart) bodyPart() mail.BodyPart {
	q := url.Values{"m": {h.message}, "k": {string(h.slot)}, "a": {h.attachment}, "c": {h.charset}}
	return mail.BodyPart(partPrefix + q.Encode())
}

// readBodyPart returns the held-apart part a token names for the message id. A token the adapter
// did not issue, or one issued for another message, is ErrInvalid.
func readBodyPart(part mail.BodyPart, id string) (heldApart, error) {
	encoded, ok := strings.CutPrefix(string(part), partPrefix)
	if ok {
		if q, err := url.ParseQuery(encoded); err == nil {
			h := heldApart{message: q.Get("m"), slot: core.Slot(q.Get("k")), attachment: q.Get("a"), charset: q.Get("c")}
			if h.message == id && (h.slot == core.HTML || h.slot == core.Text) && validID(h.attachment) {
				return h, nil
			}
		}
	}
	return heldApart{}, fmt.Errorf("gmail: body part %q of message %q: %w", part, id, mail.ErrInvalid)
}

// messageBody enacts the decision on a message read through the body mask. A rendered part held
// apart within the bound is answered with a HeldApartError naming it, once its reference is checked
// to be safe to send back (ADR-0131).
func messageBody(account string, m gmailMessage) (mail.MessageBody, error) {
	d, text := core.MessageBody(m.Payload, mime.ParseMediaType)
	switch d.Act {
	case core.Empty:
		return mail.MessageBody{AccountID: account, MessageID: m.ID}, nil
	case core.Inline:
		return core.Fill(account, m.ID, d.Slot, text), nil
	case core.Fetch:
		if !validID(d.Attachment) {
			return mail.MessageBody{}, fmt.Errorf("gmail: the %s part names a reference it cannot read: %w", d.Slot, mail.ErrProvider)
		}
		h := heldApart{message: m.ID, slot: d.Slot, attachment: d.Attachment, charset: d.Charset}
		return mail.MessageBody{}, mail.HeldApartError{Part: h.bodyPart()}
	case core.Stop:
	}
	return mail.MessageBody{}, problemError(d.Slot, d.Problem)
}

// partBody enacts the decoding of a messages.attachments.get answer for the part held apart h into
// the body it fills.
func partBody(account string, h heldApart, response []byte) (mail.MessageBody, error) {
	var raw struct {
		Size int64  `json:"size"`
		Data string `json:"data"`
	}
	if err := unmarshal(response, &raw, "a body part"); err != nil {
		return mail.MessageBody{}, err
	}
	text, problem := core.PartText(raw.Data, h.charset, whatwg)
	if problem != core.None {
		return mail.MessageBody{}, problemError(h.slot, problem)
	}
	return core.Fill(account, h.message, h.slot, text), nil
}

// whatwg is the WHATWG Encoding Standard's label table, from golang.org/x/text (ADR-0134).
var whatwg = core.Labels{Get: htmlindex.Get, Name: htmlindex.Name}

// problemError is the port's error for a body not returned for problem.
func problemError(slot core.Slot, problem core.Problem) error {
	return fmt.Errorf("gmail: the %s part is not returned, as %v: %w", slot, problem, portError(problem))
}

// portError is the port error a problem is: its refusal for a body refused for good, and otherwise
// the provider's failure.
func portError(problem core.Problem) error {
	if problem.Refused() {
		return mail.ErrRefused
	}
	return mail.ErrProvider
}
