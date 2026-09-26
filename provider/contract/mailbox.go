// Package contract is the contract suite every Provider Port implementation passes, the fake and
// each real adapter alike (ADR-0043). It is what keeps the one contract of ADR-0010 an executable
// artifact rather than prose.
//
// It is ordinary Go code rather than a test file, because Go cannot import another package's test
// files. Each implementation's own tests call Run with a Harness that builds the implementation and
// adds mail to it from the provider's side, outside the port.
//
// The suite assumes nothing about what a mailbox holds beyond what it added, because a real
// provider's run goes against a test account whose other contents are unknown (ADR-0043). Each case
// adds its own messages, marks them with a text in the subject that no other case or run shares,
// and checks and reports only the messages carrying it. A listing is read until every message the
// case added has been seen, a change set is read for the messages the case added, and a message
// that does not carry the mark is never compared or printed, since a real run's logs may be public.
// The suite deletes nothing. A case that needs a message gone from the mailbox runs only against an
// implementation whose harness can remove one, which a real provider's cannot.
//
// The suite never assumes an implementation keeps the identifiers a message was added with, since a
// real provider assigns its own, and compares only what the seed decides. Every expectation is
// written out as a literal here rather than computed by logic an implementation could share, so an
// implementation that ignores a query, drops a field or mutates the wrong message fails.
package contract

import (
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// Message is one message the mailbox holds, as the harness adds it. Metadata.ID and
// Metadata.ThreadID are keys naming the message and its thread within one case's messages, which
// an implementation may keep or replace with its own. The snippet, the size and the authentication
// results are left for the implementation to derive, as a provider does.
type Message struct {
	Metadata mail.MessageMetadata
	Body     mail.MessageBody
}

// The labels the mailbox's messages carry beyond the three the disposal verbs manage. They are
// metadata, so they carry field markers (ADR-0044).
var (
	finance  = marker.Field("financelabel")
	reading  = marker.Field("readinglabel") + "/" + marker.Field("digestlabel")
	receipts = marker.Field("receiptslabel")
)

// defaultBase is 2024-09-01T09:00:00Z, the date of the oldest message Mailbox returns.
const defaultBase = mail.UnixMilli(1725181200000)

// The messages are dated a step apart from the base. A margin is less than a step, so an instant a
// margin from a message's date falls between it and its neighbours. The step is a minute, so a real
// provider's run can date its messages just before it starts, and they are the account's newest.
const (
	step   = mail.UnixMilli(60000)
	margin = mail.UnixMilli(10000)
)

// Mailbox returns the messages every implementation is tested over, the shared synthetic fixtures
// placed in a mailbox, dated from 2024-09-01T09:00:00Z and carrying no case's mark. It adds what the
// mailbox holds about each message rather than the message itself, the keys, the thread, the date,
// the labels and the flags. The newsletter and its reply share a thread. Two messages sit outside
// the inbox, one of them unlabelled.
func Mailbox() []Message { return mailbox("", defaultBase) }

// mailbox returns Mailbox's messages with mark appended to every subject and dated from base.
func mailbox(mark string, base mail.UnixMilli) []Message {
	return []Message{
		seed(mark, base, "bank", "bank", fixture.Bank(), 0, mail.Flags{Read: true}, mail.Inbox, finance),
		seed(mark, base, "newsletter", "newsletter", fixture.Newsletter(), 1, mail.Flags{Read: true, Starred: true}, mail.Inbox, reading),
		seed(mark, base, "code", "code", fixture.OneTimeCode(), 2, mail.Flags{}, mail.Inbox),
		seed(mark, base, "alpha", "alpha", fixture.AlphanumericCode(), 3, mail.Flags{}, mail.Inbox),
		seed(mark, base, "link", "link", fixture.LoginLink(), 4, mail.Flags{Read: true}),
		seed(mark, base, "receipt", "receipt", fixture.Receipt(), 5, mail.Flags{Read: true}, receipts),
		seed(mark, base, "reply", "newsletter", fixture.NewsletterReply(), 6, mail.Flags{}, mail.Inbox, reading),
	}
}

func seed(mark string, base mail.UnixMilli, key, thread string, f fixture.Message, steps mail.UnixMilli, flags mail.Flags, labels ...string) Message {
	m := mail.MessageMetadata{
		ID:              key,
		ThreadID:        thread,
		From:            mail.Address{Email: f.FromAddress, Name: f.FromName},
		To:              []mail.Address{{Email: f.ToAddress}},
		Subject:         f.Subject + mark,
		Date:            base + steps*step,
		Labels:          labels,
		Flags:           flags,
		HasAttachments:  len(f.Attachments) > 0,
		AttachmentNames: f.Attachments,
		ListID:          f.ListID,
	}
	if f.CcAddress != "" {
		m.Cc = []mail.Address{{Email: f.CcAddress}}
	}
	return Message{Metadata: m, Body: mail.MessageBody{Text: f.Body}}
}

// late is a message a case delivers, as mail arriving. It is the receipt fixture with a subject of
// its own, so the suite tells it apart by subject like every other message.
func late(mark string, base mail.UnixMilli) Message {
	f := fixture.Receipt()
	f.Subject = marker.Field("latesubject") + " Your order has shipped"
	return seed(mark, base, "late", "late", f, 7, mail.Flags{}, mail.Inbox)
}

// brief is a message a case delivers and then removes.
func brief(mark string, base mail.UnixMilli) Message {
	f := fixture.Receipt()
	f.Subject = marker.Field("briefsubject") + " Your order has shipped"
	return seed(mark, base, "brief", "brief", f, 8, mail.Flags{}, mail.Inbox)
}
