// Package fixture holds the synthetic mail fixtures tests share. Every fixture carries the marker
// text defined in core/marker, a body marker in its body and a field marker in every metadata
// field, so a test can search any output for leaked body text and find visible metadata by name.
//
// No fixture holds real mail (ADR-0044). Senders sit under the reserved .example domain, and which
// of them a test treats as restricted is the test's policy, never the fixture's.
//
// A fixture is a value of this package's own Message type, not of the canonical mail model in
// core/mail.
package fixture

import "github.com/ppat/mediated-mailbox-mcp/core/marker"

// Message is one synthetic message.
type Message struct {
	FromAddress string
	FromName    string
	Subject     string
	Body        string
}

// Bank is a notice from a financial institution, the kind of sender a policy restricts.
func Bank() Message {
	return Message{
		FromAddress: marker.Field("bankaddress") + "@bank.example",
		FromName:    marker.MarkupField("bankname"),
		Subject:     marker.Field("banksubject"),
		Body:        marker.Body("bank"),
	}
}

// Newsletter is a message from an ordinary sender.
func Newsletter() Message {
	return Message{
		FromAddress: marker.Field("newsletteraddress") + "@newsletter.example",
		FromName:    marker.Field("newslettername"),
		Subject:     marker.MarkupField("newslettersubject"),
		Body:        marker.Body("newsletter"),
	}
}

// OneTimeCode is a verification message with a six-digit code in its subject and its body, as the
// sanitizing converter's Markdown.
func OneTimeCode() Message {
	return Message{
		FromAddress: marker.Field("codeaddress") + "@security.example",
		FromName:    marker.Field("codename"),
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
		Subject:     marker.Field("alphasubject") + " Your sign-in code",
		Body:        marker.Body("alpha") + "\n\nUse this code to sign in.\n\n**7GX4Q2**\n",
	}
}

// LoginLink is a message carrying a one-click sign-in link.
func LoginLink() Message {
	return Message{
		FromAddress: marker.Field("linkaddress") + "@login.example",
		FromName:    marker.Field("linkname"),
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
		Subject:     marker.Field("receiptsubject") + " Your order has shipped",
		Body: marker.Body("receipt") + "\n\nOrder 20240917 shipped on 2024-09-17 with tracking number " +
			"1Z999AA10123456784.\n\n| Item | Price |\n| --- | --- |\n| Lamp | 42.00 |\n| Total | 42.00 |\n\n" +
			"Questions? Reply to this message or see https://shop.example/help/orders.\n",
	}
}

// All returns every shared fixture.
func All() []Message {
	return []Message{Bank(), Newsletter(), OneTimeCode(), AlphanumericCode(), LoginLink(), Receipt()}
}
