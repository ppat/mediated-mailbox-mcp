package provider

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/google"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// VERIFICATIONS' rows for a token made with its client's identifier and secret swapped, and for an
// account's token made from its own client, for the provider library's part. A Gmail account's
// token is made from the identifier and secret of the client it connects through, each in its own
// place, and from the account's refresh token, so a mix-up is caught here rather than surfacing as a
// refusal at Google's token endpoint (ADR-0085, ADR-0106).
func TestAGmailTokenIsMadeFromItsClientAndItsCredential(t *testing.T) {
	got := gmailCredentials(coremail.OAuthClient{ID: "client-id", Secret: "client-secret"}, []byte("refresh-token"))
	want := google.Credentials{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "refresh-token"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the credentials (-want +got):\n%s", diff)
	}
}
