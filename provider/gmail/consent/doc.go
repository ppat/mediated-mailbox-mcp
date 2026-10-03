// Package consent is Google's installed-app OAuth consent for Gmail, apart from the adapter, so the UI
// links it without linking any code that reads a mailbox (ADR-0107). It holds no Provider Port code.
//
// It builds the consent page's address with its PKCE challenge and state, exchanges the code for the
// grant and refuses any grant but the modify scope alone, reads the profile that names the mailbox a
// grant belongs to, compares that mailbox with the one named, reads the loopback address the operator
// pastes back, and checks a client's identifier and secret against the token endpoint. Google
// implements the consent interface of core/mail, which the UI runs. Interactive runs a consent that
// listens on the loopback address itself, for the consent command.
//
// It also holds the requests to Google's token endpoint and the reading of their answers, which the
// adapter's token source sends its refreshes through, and the outcome a request's answer records
// (ADR-0097).
//
// What it does with Google's own answers is tested on bodies shaped as Google documents them, since a
// stand-in for Google's endpoints would be a mock (ADR-0043). The contract suite's run against real
// Gmail and the live check of the client setup cover the rest.
package consent
