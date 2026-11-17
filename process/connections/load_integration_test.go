//go:build integration

package connections_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// VERIFICATIONS' row for an OAuth client being optional per provider. An account whose provider has
// no OAuth client loads without one, and one that connects through a client loads with it, its token
// made from the client's identifier and secret and the account's credential (ADR-0080, ADR-0090,
// ADR-0106).
func TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "client-id", sealed(t, keys, "client-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, keys, "gmail-token", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	account(t, conn, "work", "fastmail", sealed(t, keys, "fastmail-token", seal.AccountCredential("work")))

	r := start(t, keys)

	want := map[string]string{"personal": "gmail-token through client-id client-secret", "work": "fastmail-token"}
	if diff := cmp.Diff(want, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("each account's token (-want +got):\n%s", diff)
	}
	if got := r.logs.records(t, "ERROR"); len(got) != 0 {
		t.Errorf("error records %v", got)
	}
}

// F6's part of VERIFICATIONS' row for an account's token made from its own client. Two accounts of
// one provider that connect through two different clients each get a token made from their own, a
// third that shares one of them gets one made from it, and an account of that provider naming no
// client has none, though the provider has clients. A third client carries its provider's name, as
// the client the migration to named clients kept does, so a fallback to a client found by the
// provider would hand it over (ADR-0106, ADR-0090).
func TestEachAccountIsLoadedWithTheClientItNamesAndNoOther(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, keys, "household-secret", seal.ClientSecret("household")))
	client(t, conn, "employer", "gmail", "employer-id", sealed(t, keys, "employer-secret", seal.ClientSecret("employer")))
	client(t, conn, "gmail", "gmail", "migrated-id", sealed(t, keys, "migrated-secret", seal.ClientSecret("gmail")))
	for _, id := range []string{"personal", "family", "work", "unpaired"} {
		account(t, conn, id, "gmail", sealed(t, keys, id+"-token", seal.AccountCredential(id)))
	}
	through(t, conn, "personal", "household")
	through(t, conn, "family", "household")
	through(t, conn, "work", "employer")

	r := start(t, keys)

	want := map[string]string{
		"personal": "personal-token through household-id household-secret",
		"family":   "family-token through household-id household-secret",
		"work":     "work-token through employer-id employer-secret",
		"unpaired": "not connected",
	}
	if diff := cmp.Diff(want, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("each account's token (-want +got):\n%s", diff)
	}
	if got := r.logs.records(t, "ERROR"); len(got) != 0 {
		t.Errorf("error records %v", got)
	}
}

// VERIFICATIONS' row for an account of a provider that authenticates through an OAuth client served
// without its client. A Gmail account connects through the client it names, and neither a Gmail
// account naming no client nor one whose client's secret does not open is connected, though another
// Gmail client opened, and each is logged. A Fastmail account needs no client and is connected, so
// the refusal comes from the facts the list of adapters holds about each provider (ADR-0106,
// ADR-0090).
func TestAnAccountOfAClientProviderWithoutItsClientIsNotConnected(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, keys, "household-secret", seal.ClientSecret("household")))
	client(t, conn, "broken", "gmail", "broken-id", sealed(t, keys, "broken-secret", seal.ClientSecret("another-row")))
	for _, id := range []string{"paired", "unpaired", "unopened"} {
		account(t, conn, id, "gmail", sealed(t, keys, id+"-token", seal.AccountCredential(id)))
	}
	account(t, conn, "mail", "fastmail", sealed(t, keys, "mail-token", seal.AccountCredential("mail")))
	through(t, conn, "paired", "household")
	through(t, conn, "unopened", "broken")

	r := start(t, keys)

	want := map[string]string{
		"paired": "paired-token through household-id household-secret", "unpaired": "not connected",
		"unopened": "not connected", "mail": "mail-token",
	}
	if diff := cmp.Diff(want, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("each account's token (-want +got):\n%s", diff)
	}
	logs := r.logs.String()
	if !strings.Contains(logs, `"account":"unpaired"`) || !strings.Contains(logs, `"account":"unopened"`) {
		t.Errorf("the accounts left not connected are not logged: %s", logs)
	}
	if diff := cmp.Diff([]string{"mail", "paired"}, r.session.Workable(), compare.Options); diff != "" {
		t.Errorf("the workable accounts (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' rows for an account with no state row and for a credential supplied
// outside its state row. An account with no state row, and one whose state row holds no credential,
// load as not connected, whatever the environment and the mounted files hold, since the connections
// read the database alone (ADR-0080, ADR-0091).
func TestAnAccountWithoutAStoredCredentialIsNotConnected(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	listedOnly(t, conn, "no-state", "fastmail")
	account(t, conn, "no-credential", "fastmail", nil)
	t.Setenv("GMAIL_TEST_REFRESH_TOKEN", "environment-token")
	if err := os.WriteFile(filepath.Join(t.TempDir(), "refresh-token"), []byte("mounted-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := start(t, keys)

	want := map[string]string{"no-state": "not connected", "no-credential": "not connected"}
	if diff := cmp.Diff(want, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("each account's token (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for copying a sealed credential. A credential copied from one account's state row
// to another's, and a client secret stored as an account's credential, do not open there, so the
// account loads as not connected. An account credential stored as a client's secret does not open
// either, so the account that connects through the client has none and is not connected. Each refusal
// is logged without the value (ADR-0088, ADR-0091).
func TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	personal := sealed(t, keys, "personal-token", seal.AccountCredential("personal"))
	account(t, conn, "personal", "gmail", personal)
	account(t, conn, "copied", "fastmail", personal)
	account(t, conn, "household", "fastmail", sealed(t, keys, "client-secret", seal.ClientSecret("household")))
	client(t, conn, "household", "gmail", "client-id", sealed(t, keys, "account-token", seal.AccountCredential("household")))
	through(t, conn, "personal", "household")

	r := start(t, keys)

	want := map[string]string{"personal": "not connected", "copied": "not connected", "household": "not connected"}
	if diff := cmp.Diff(want, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("each account's token (-want +got):\n%s", diff)
	}
	var refused []string
	for _, rec := range r.logs.records(t, "ERROR") {
		if strings.Contains(rec["msg"], "did not open") {
			refused = append(refused, rec["account"]+rec["client"])
		}
	}
	slices.Sort(refused)
	refused = slices.Compact(refused)
	if diff := cmp.Diff([]string{"copied", "household"}, refused, compare.Options); diff != "" {
		t.Errorf("clients and accounts logged as not opening (-want +got):\n%s", diff)
	}
	for _, value := range []string{"personal-token", "client-secret", "account-token"} {
		if strings.Contains(r.logs.String(), value) {
			t.Errorf("the log holds %q", value)
		}
	}
}

// F6's part of VERIFICATIONS' row for reloading the accounts, and the Session's failure table for the
// connections. An account that appears is connected from the next reload and one that disappears is
// dropped, and no unit of work begins on it. A reload whose read of the credentials or the clients
// fails keeps in force what was, the list of accounts included, is logged, and returns an error
// wrapping ErrUntrustedRead, and a read that lists no account connects none (ADR-0090).
func TestAReloadConnectsWhatItReadAndKeepsWhatWasOnAFailedRead(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "personal-token", seal.AccountCredential("personal")))
	r := start(t, keys)

	account(t, conn, "work", "fastmail", sealed(t, keys, "work-token", seal.AccountCredential("work")))
	forget(t, conn, "personal")
	r.reload(t)
	if diff := cmp.Diff(map[string]string{"work": "work-token"}, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("after one account appeared and one disappeared (-want +got):\n%s", diff)
	}
	if _, err := r.connections.Begin("personal", r.caller, noBound); !errors.Is(err, connections.ErrNotConnected) {
		t.Errorf("a unit of work on an account no longer listed began with %v, want ErrNotConnected", err)
	}

	must(t, conn, "REVOKE SELECT ON oauth_clients FROM "+role)
	restoreOnCleanup(t, "GRANT SELECT (name, provider, client_id, client_secret) ON oauth_clients TO "+role)
	account(t, conn, "later", "fastmail", sealed(t, keys, "later-token", seal.AccountCredential("later")))
	err := r.session.Reload(t.Context())
	if !errors.Is(err, connections.ErrUntrustedRead) || errors.Is(err, sessions.ErrPolicyNotLoaded) {
		t.Errorf("a reload whose read of the connections failed returned %v, want ErrUntrustedRead", err)
	}
	if diff := cmp.Diff([]string{"work"}, ids(r), compare.Options); diff != "" {
		t.Errorf("the accounts in force after a failed read of the connections (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]string{"work": "work-token"}, r.tokens(t), compare.Options); diff != "" {
		t.Errorf("after a failed read (-want +got):\n%s", diff)
	}
	if got := r.logs.records(t, "ERROR"); !slices.ContainsFunc(got, func(rec map[string]string) bool {
		return strings.Contains(rec["msg"], "previous ones stay")
	}) {
		t.Errorf("error records %v, want one naming the failed reload", got)
	}
	must(t, conn, "GRANT SELECT (name, provider, client_id, client_secret) ON oauth_clients TO "+role)

	reset(t, conn)
	r.reload(t)
	if got := r.session.Accounts(); len(got) != 0 {
		t.Errorf("a read listing no account holds %v", got)
	}
	if r.connections.Connected("work") {
		t.Error("a read listing no account left an account connected")
	}
}

// ids returns the identifiers of the accounts the Session has in force.
func ids(r *running) []string {
	var out []string
	for _, a := range r.session.Accounts() {
		out = append(out, a.ID)
	}
	return out
}
