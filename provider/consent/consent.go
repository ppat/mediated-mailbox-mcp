// Package consent returns each provider's consent by the provider's name, for setting an account up,
// so the UI's setup reaches the provider library through it and links neither an adapter nor the
// rate limiter (ADR-0107).
package consent

import (
	"fmt"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/provider/google"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
)

// For returns the consent of the provider name, calling the provider through client, whose consent
// page redirects the browser to redirect. It returns an error for a provider whose accounts connect
// through no consent.
func For(name string, client *http.Client, redirect string) (mail.Consent, error) {
	switch name {
	case "gmail":
		return google.New(client, redirect), nil
	default:
		return nil, fmt.Errorf("provider %q has no consent", name)
	}
}
