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

// All returns every shared fixture.
func All() []Message {
	return []Message{Bank(), Newsletter()}
}
