// Package fixture holds the synthetic mail fixtures tests share. Every fixture carries the marker
// text defined in testsupport/marker, a body marker in its body and a field marker in every
// metadata field, so a test can search any output for leaked body text and find visible metadata
// by name.
//
// No fixture holds real mail (ADR-0044). Senders sit under the reserved .example domain, and which
// of them a test treats as restricted is the test's policy, never the fixture's.
//
// A fixture is a value of this package's own Message type, not of the canonical mail model in
// core/mail. A Message holds what a message says about itself, its headers and its body. The
// contract suite in provider/contract turns the fixtures into the canonical messages of the mailbox
// every port implementation is tested against, adding what the mailbox holds about each message
// rather than the message itself, its identifiers, thread, date, labels and flags.
package fixture

import "github.com/ppat/mediated-mailbox-mcp/testsupport/marker"

// Message is one synthetic message. The fields after Body are empty on the fixtures that lack them.
//
// Attachment names are derived from the body, so the Redaction Gate withholds them with it
// (ADR-0001). They carry field markers all the same, because they are visible for a normal sender,
// and a body marker is found only in text that is never visible. A test of their withholding
// searches for the one fixture's attachment marker.
type Message struct {
	FromAddress string
	FromName    string
	ToAddress   string
	Subject     string
	Body        string
	CcAddress   string
	ListID      string
	Attachments []string
}

// Bank is a notice from a financial institution, the kind of sender a policy restricts.
func Bank() Message {
	return Message{
		FromAddress: marker.Field("bankaddress") + "@bank.example",
		FromName:    marker.MarkupField("bankname"),
		ToAddress:   marker.Field("bankto") + "@home.example",
		Subject:     marker.Field("banksubject"),
		Body:        marker.Body("bank"),
		Attachments: []string{marker.Field("statement") + ".pdf"},
	}
}

// Newsletter is a message from an ordinary sender.
func Newsletter() Message {
	return Message{
		FromAddress: marker.Field("newsletteraddress") + "@newsletter.example",
		FromName:    marker.Field("newslettername"),
		ToAddress:   marker.Field("newsletterto") + "@home.example",
		Subject:     marker.MarkupField("newslettersubject"),
		Body:        marker.Body("newsletter"),
		ListID:      marker.Field("newsletterlist") + ".newsletter.example",
	}
}

// NewsletterReply is a reply in the newsletter's conversation. Its subject is the newsletter's with
// a reply prefix, because a provider threads a reply only when the subjects match, so it is the one
// fixture whose field contains another fixture's.
func NewsletterReply() Message {
	return Message{
		FromAddress: marker.Field("replyaddress") + "@reader.example",
		FromName:    marker.Field("replyname"),
		ToAddress:   marker.Field("replyto") + "@newsletter.example",
		Subject:     "Re: " + Newsletter().Subject,
		Body:        marker.Body("reply") + "\n\nThanks, this issue was useful.\n",
	}
}

// OneTimeCode is a verification message with a six-digit code in its subject and its body, as the
// sanitizing converter's Markdown.
func OneTimeCode() Message {
	return Message{
		FromAddress: marker.Field("codeaddress") + "@security.example",
		FromName:    marker.Field("codename"),
		ToAddress:   marker.Field("codeto") + "@home.example",
		Subject:     marker.Field("codesubject") + " Your code is 419283",
		Body: marker.Body("code") + "\n\nYour verification code is 419283. It expires in 10 minutes.\n\n" +
			"# 419283\n",
	}
}

// AlphanumericCode is a sign-in message whose code mixes letters and digits, which only tier 2's
// scoring catches.
func AlphanumericCode() Message {
	return Message{
		FromAddress: marker.Field("alphaaddress") + "@signin.example",
		FromName:    marker.Field("alphaname"),
		ToAddress:   marker.Field("alphato") + "@home.example",
		Subject:     marker.Field("alphasubject") + " Your sign-in code",
		Body:        marker.Body("alpha") + "\n\nUse this code to sign in.\n\n**7GX4Q2**\n",
	}
}

// LoginLink is a message carrying a one-click sign-in link.
func LoginLink() Message {
	return Message{
		FromAddress: marker.Field("linkaddress") + "@login.example",
		FromName:    marker.Field("linkname"),
		ToAddress:   marker.Field("linkto") + "@home.example",
		Subject:     marker.Field("linksubject") + " Sign in to your account",
		Body:        marker.Body("link") + "\n\n[Sign in](https://login.example/auth/magic/Q7fT2kLm9ZpX4vRw?token=h3J9dK2mQ8xR5tY1vB7n)\n",
	}
}

// Receipt is an order receipt full of numbers that are not codes, an order number, a date, a
// tracking number and prices.
func Receipt() Message {
	return Message{
		FromAddress: marker.Field("receiptaddress") + "@shop.example",
		FromName:    marker.Field("receiptname"),
		ToAddress:   marker.Field("receiptto") + "@home.example",
		CcAddress:   marker.Field("receiptcc") + "@home.example",
		Subject:     marker.Field("receiptsubject") + " Your order has shipped",
		Body: marker.Body("receipt") + "\n\nOrder 20240917 shipped on 2024-09-17 with tracking number " +
			"1Z999AA10123456784.\n\n| Item | Price |\n| --- | --- |\n| Lamp | 42.00 |\n| Total | 42.00 |\n\n" +
			"Questions? Reply to this message or see https://shop.example/help/orders.\n",
	}
}

// All returns every shared fixture.
func All() []Message {
	return []Message{Bank(), Newsletter(), NewsletterReply(), OneTimeCode(), AlphanumericCode(), LoginLink(), Receipt()}
}
