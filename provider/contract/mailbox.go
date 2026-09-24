// Package contract is the contract suite every Provider Port implementation passes, the fake and
// each real adapter alike (ADR-0043). It is what keeps the one contract of ADR-0010 an executable
// artifact rather than prose.
//
// It is ordinary Go code rather than a test file, because Go cannot import another package's test
// files. Each implementation's own tests call Run with a Harness that builds the implementation over
// a mailbox holding exactly the messages Mailbox returns, and that delivers and removes mail from the
// provider's side, outside the port.
//
// The suite never assumes an implementation keeps the identifiers a message was seeded with, since a
// real provider assigns its own. It finds each message by its subject, which is distinct across the
// mailbox, and compares only what the seed decides. Every expectation is written out as a literal
// here rather than computed by logic an implementation could share, so an implementation that
// ignores a query, drops a field or mutates the wrong message fails.
package contract

import (
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
)

// Message is one message the mailbox holds, as the harness seeds it. Metadata.ID and
// Metadata.ThreadID are keys naming the message and its thread within the mailbox, which an
// implementation may keep or replace with its own. The snippet, the size and the authentication
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

// base is 2024-09-01T09:00:00Z, the date of the oldest message. The messages are a day apart.
const (
	base = mail.UnixMilli(1725181200000)
	hour = mail.UnixMilli(3600000)
	day  = 24 * hour
)

// Mailbox returns the messages every implementation is tested over, the shared synthetic fixtures
// placed in a mailbox. It adds what the mailbox holds about each message rather than the message
// itself, the keys, the thread, the date, the labels and the flags. The newsletter and its reply
// share a thread. Two messages sit outside the inbox, one of them unlabelled.
func Mailbox() []Message {
	return []Message{
		seed("bank", "bank", fixture.Bank(), 0, mail.Flags{Read: true}, mail.Inbox, finance),
		seed("newsletter", "newsletter", fixture.Newsletter(), 1, mail.Flags{Read: true, Starred: true}, mail.Inbox, reading),
		seed("code", "code", fixture.OneTimeCode(), 2, mail.Flags{}, mail.Inbox),
		seed("alpha", "alpha", fixture.AlphanumericCode(), 3, mail.Flags{}, mail.Inbox),
		seed("link", "link", fixture.LoginLink(), 4, mail.Flags{Read: true}),
		seed("receipt", "receipt", fixture.Receipt(), 5, mail.Flags{Read: true}, receipts),
		seed("reply", "newsletter", fixture.NewsletterReply(), 6, mail.Flags{}, mail.Inbox, reading),
	}
}

func seed(key, thread string, f fixture.Message, days mail.UnixMilli, flags mail.Flags, labels ...string) Message {
	m := mail.MessageMetadata{
		ID:              key,
		ThreadID:        thread,
		From:            mail.Address{Email: f.FromAddress, Name: f.FromName},
		To:              []mail.Address{{Email: f.ToAddress}},
		Subject:         f.Subject,
		Date:            base + days*day,
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

// late is a message the suite delivers during a case, as mail arriving. It is the receipt fixture
// with a subject of its own, so the suite finds it by subject like every other message.
func late() Message {
	f := fixture.Receipt()
	f.Subject = marker.Field("latesubject") + " Your order has shipped"
	return seed("late", "late", f, 7, mail.Flags{}, mail.Inbox)
}

// brief is a message the suite delivers and then removes during a case.
func brief() Message {
	f := fixture.Receipt()
	f.Subject = marker.Field("briefsubject") + " Your order has shipped"
	return seed("brief", "brief", f, 8, mail.Flags{}, mail.Inbox)
}
