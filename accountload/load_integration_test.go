//go:build integration

package accountload_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// VERIFICATIONS' row for an OAuth client being optional per provider. An account whose provider has
// no row in oauth_clients loads without a client, and one whose provider has a row loads with it
// (ADR-0080, ADR-0090).
func TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "gmail", sealed(t, keys, "gmail-token", seal.AccountCredential("personal")))
	account(t, conn, "work", "fastmail", sealed(t, keys, "fastmail-token", seal.AccountCredential("work")))
	client(t, conn, "gmail", "client-id", sealed(t, keys, "client-secret", seal.ClientSecret("gmail")))
	var log logBuffer

	s := load(t, keyring(t, keys), &log).Snapshot()

	want := map[string]string{"personal": "gmail-token", "work": "fastmail-token"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
	c, ok := s.Client("gmail")
	if !ok || c.ID() != "client-id" || string(c.Secret()) != "client-secret" {
		t.Errorf("gmail's client: %v, %q, %q, want client-id and its secret", ok, c.ID(), c.Secret())
	}
	if _, ok := s.Client("fastmail"); ok {
		t.Error("fastmail, which has no row in oauth_clients, has a client")
	}
	if got := log.errors(t); len(got) != 0 {
		t.Errorf("error records %v", got)
	}
}

// F6's part of VERIFICATIONS' rows for an account with no state row and for a credential supplied
// outside its state row. An account with no state row, and one whose state row holds no credential,
// load as not connected, whatever the environment and the mounted files hold, since the loader reads
// the database alone (ADR-0080, ADR-0091).
func TestAnAccountWithoutAStoredCredentialIsNotConnected(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	listedOnly(t, conn, "no-state", "gmail")
	account(t, conn, "no-credential", "gmail", nil)
	t.Setenv("GMAIL_TEST_REFRESH_TOKEN", "environment-token")
	if err := os.WriteFile(filepath.Join(t.TempDir(), "refresh-token"), []byte("mounted-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	var log logBuffer

	s := load(t, keyring(t, keys), &log).Snapshot()

	want := map[string]string{"no-state": "not connected", "no-credential": "not connected"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' row for reloading the account snapshot. An account that appears is
// served from the next load and one that disappears is dropped. A read that fails keeps the previous
// snapshot and is logged, and a read that lists no account serves none. A unit of work keeps the
// snapshot it took (ADR-0090).
func TestAReloadServesWhatItReadAndKeepsTheLastGoodSnapshotOnAFailedRead(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "gmail", sealed(t, keys, "personal-token", seal.AccountCredential("personal")))
	var log logBuffer
	l := load(t, keyring(t, keys), &log)
	taken := l.Snapshot()

	account(t, conn, "work", "gmail", sealed(t, keys, "work-token", seal.AccountCredential("work")))
	must(t, conn, "DELETE FROM account_state WHERE account_id = 'personal'")
	must(t, conn, "DELETE FROM accounts WHERE account_id = 'personal'")
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]string{"work": "work-token"}, credentials(l.Snapshot()), compare.Options); diff != "" {
		t.Errorf("after one account appeared and one disappeared (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]string{"personal": "personal-token"}, credentials(taken), compare.Options); diff != "" {
		t.Errorf("the snapshot a unit of work took changed (-want +got):\n%s", diff)
	}

	must(t, conn, "REVOKE SELECT ON oauth_clients FROM "+role)
	restoreOnCleanup(t, "GRANT SELECT (provider, client_id, client_secret) ON oauth_clients TO "+role)
	account(t, conn, "later", "gmail", sealed(t, keys, "later-token", seal.AccountCredential("later")))
	if err := l.Load(t.Context()); !errors.Is(err, accountload.ErrUntrustedRead) {
		t.Errorf("a load whose read failed: %v, want ErrUntrustedRead", err)
	}
	if diff := cmp.Diff(map[string]string{"work": "work-token"}, credentials(l.Snapshot()), compare.Options); diff != "" {
		t.Errorf("after a failed read (-want +got):\n%s", diff)
	}
	if got := log.errors(t); len(got) != 1 || !strings.Contains(got[0]["msg"], "previous one stays") {
		t.Errorf("error records %v, want one naming the failed reload", got)
	}
	must(t, conn, "GRANT SELECT (provider, client_id, client_secret) ON oauth_clients TO "+role)

	reset(t, conn)
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := l.Snapshot().Accounts(); len(got) != 0 {
		t.Errorf("a read listing no account serves %d", len(got))
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set. After the operator replaces an
// account's credential, a write-back based on the credential the loader read before changes nothing,
// and the loader reads the stored credential again and hands it back (ADR-0089).
func TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "gmail", sealed(t, keys, "first-token", c))
	var log logBuffer
	l := load(t, keyring(t, keys), &log)
	taken := l.Snapshot()

	reauthorized := sealed(t, keys, "reauthorized-token", c)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
	use, err := l.Persist(t.Context(), "personal", []byte("rotated-token"))
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if string(use) != "reauthorized-token" {
		t.Errorf("the write-back hands back %q, want the credential the operator stored", use)
	}
	if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
		t.Error("the stale write-back replaced the operator's credential")
	}
	if got := credentials(l.Snapshot())["personal"]; got != "reauthorized-token" {
		t.Errorf("the snapshot holds %q, want the operator's credential", got)
	}
	if got := credentials(taken)["personal"]; got != "first-token" {
		t.Errorf("the snapshot a unit of work took holds %q, want the credential it was taken with", got)
	}

	use, err = l.Persist(t.Context(), "personal", []byte("next-rotated-token"))
	if err != nil || string(use) != "next-rotated-token" {
		t.Errorf("a write-back on the bytes the loader now knows: %q, %v, want it to land", use, err)
	}
	if got, err := keyring(t, keys).Open(storedCredential(t, conn, "personal"), c); err != nil || string(got) != "next-rotated-token" {
		t.Errorf("the stored credential opens as %q, %v, want the rotated one", got, err)
	}
}

// F6's part of VERIFICATIONS' row for forcing a rotation and restarting. At the end of a unit of work
// the deployable hands over the credential its adapter holds. One equal to the credential last read
// or written writes nothing, and a rotated one is written back once, and is what a new loader, as
// after a restart, loads (ADR-0082).
func TestARotatedCredentialSurvivesARestart(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	original := sealed(t, keys, "original-token", seal.AccountCredential("personal"))
	account(t, conn, "personal", "gmail", original)
	var log logBuffer
	l := load(t, keyring(t, keys), &log)

	if _, err := l.Persist(t.Context(), "personal", []byte("original-token")); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if !bytes.Equal(storedCredential(t, conn, "personal"), original) {
		t.Error("an unchanged credential was written")
	}
	if _, err := l.Persist(t.Context(), "personal", []byte("rotated-token")); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	written := storedCredential(t, conn, "personal")
	if _, err := l.Persist(t.Context(), "personal", []byte("rotated-token")); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if !bytes.Equal(storedCredential(t, conn, "personal"), written) {
		t.Error("a rotated credential already written was written again")
	}
	restarted := load(t, keyring(t, keys), &log).Snapshot()
	if got := credentials(restarted)["personal"]; got != "rotated-token" {
		t.Errorf("after a restart the account holds %q, want the rotated credential", got)
	}
}

// VERIFICATIONS' row for a failed write-back, and F6's part of the reload row. A write-back that fails
// keeps the rotated credential in memory, is logged without the credential, and a reload keeps it
// while the stored bytes stay the ones the loader knew. A later write-back lands it (ADR-0082,
// ADR-0090).
func TestAFailedWriteBackIsHeldLoggedAndKeptByAReload(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "gmail", sealed(t, keys, "original-token", seal.AccountCredential("personal")))
	var log logBuffer
	l := load(t, keyring(t, keys), &log)

	must(t, conn, "REVOKE UPDATE ON account_state FROM "+role)
	restoreOnCleanup(t, "GRANT UPDATE (credential) ON account_state TO "+role)
	use, err := l.Persist(t.Context(), "personal", []byte("rotated-token"))
	if err == nil {
		t.Fatal("the write-back succeeded without its grant")
	}
	if string(use) != "rotated-token" {
		t.Errorf("a failed write-back hands back %q, want the rotated credential", use)
	}
	want := []map[string]string{{"msg": "a rotated credential was not written back, so a restart before a later write-back lands loses the account's access", "account": "personal"}}
	if diff := cmp.Diff(want, log.errors(t), compare.Options); diff != "" {
		t.Errorf("error records (-want +got):\n%s", diff)
	}
	if strings.Contains(log.String(), "rotated-token") {
		t.Errorf("the log holds the credential: %s", log.String())
	}
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := credentials(l.Snapshot())["personal"]; got != "rotated-token" {
		t.Errorf("after a reload the account holds %q, want the rotated credential whose write-back failed", got)
	}

	must(t, conn, "GRANT UPDATE (credential) ON account_state TO "+role)
	if _, err := l.Persist(t.Context(), "personal", []byte("rotated-token")); err != nil {
		t.Fatalf("the write-back tried again: %v", err)
	}
	if got, err := keyring(t, keys).Open(storedCredential(t, conn, "personal"), seal.AccountCredential("personal")); err != nil || string(got) != "rotated-token" {
		t.Errorf("the stored credential opens as %q, %v, want the rotated one", got, err)
	}
}

// F6's part of VERIFICATIONS' reload row. A reload after the operator replaced a credential the loader
// held a failed write-back for takes the stored one, and a credential the provider refused is read
// again from its row (ADR-0089, ADR-0090).
func TestAStoredCredentialSomeoneElseReplacedWins(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "gmail", sealed(t, keys, "original-token", c))
	var log logBuffer
	l := load(t, keyring(t, keys), &log)

	must(t, conn, "REVOKE UPDATE ON account_state FROM "+role)
	restoreOnCleanup(t, "GRANT UPDATE (credential) ON account_state TO "+role)
	if _, err := l.Persist(t.Context(), "personal", []byte("rotated-token")); err == nil {
		t.Fatal("the write-back succeeded without its grant")
	}
	must(t, conn, "GRANT UPDATE (credential) ON account_state TO "+role)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, "reauthorized-token", c))
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := credentials(l.Snapshot())["personal"]; got != "reauthorized-token" {
		t.Errorf("after a reload the account holds %q, want the operator's credential", got)
	}

	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, "again-token", c))
	got, err := l.Reread(t.Context(), "personal")
	if err != nil || string(got) != "again-token" {
		t.Errorf("reading a refused credential again: %q, %v, want the stored one", got, err)
	}
	if got := credentials(l.Snapshot())["personal"]; got != "again-token" {
		t.Errorf("after reading again the snapshot holds %q", got)
	}
}

// F6's part of VERIFICATIONS' row for re-sealing to the current key. A credential sealed to an old key
// is sealed again to the current one and written by compare-and-set, and the scan reads true for it
// before and false after. An account whose credential cannot be opened reads true, and one with no
// state row reads false. Every row of oauth_clients has an entry, and a provider with none has no
// entry (ADR-0092).
func TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old, lost := generate(t), generate(t), generate(t)
	account(t, conn, "on-old", "gmail", sealed(t, old, "old-token", seal.AccountCredential("on-old")))
	account(t, conn, "on-current", "gmail", sealed(t, current, "current-token", seal.AccountCredential("on-current")))
	account(t, conn, "unopenable", "gmail", sealed(t, lost, "lost-token", seal.AccountCredential("unopenable")))
	listedOnly(t, conn, "no-state", "fastmail")
	client(t, conn, "gmail", "client-id", sealed(t, old, "client-secret", seal.ClientSecret("gmail")))
	var log logBuffer
	l := load(t, keyring(t, current, old), &log)

	before := accountload.Scan{
		Accounts: map[string]bool{"on-old": true, "on-current": false, "unopenable": true, "no-state": false},
		Clients:  map[string]bool{"gmail": true},
	}
	if diff := cmp.Diff(before, l.Scan(), compare.Options); diff != "" {
		t.Errorf("the scan after loading (-want +got):\n%s", diff)
	}
	onCurrent := storedCredential(t, conn, "on-current")

	after := l.Reseal(t.Context())

	want := accountload.Scan{
		Accounts: map[string]bool{"on-old": false, "on-current": false, "unopenable": true, "no-state": false},
		Clients:  map[string]bool{"gmail": true},
	}
	if diff := cmp.Diff(want, after, compare.Options); diff != "" {
		t.Errorf("the scan after re-sealing (-want +got):\n%s", diff)
	}
	if got, err := keyring(t, current).Open(storedCredential(t, conn, "on-old"), seal.AccountCredential("on-old")); err != nil || string(got) != "old-token" {
		t.Errorf("the re-sealed credential opens with the current key alone as %q, %v", got, err)
	}
	if !bytes.Equal(storedCredential(t, conn, "on-current"), onCurrent) {
		t.Error("a credential already on the current key was written again")
	}
	if got := credentials(load(t, keyring(t, current), &log).Snapshot())["on-old"]; got != "old-token" {
		t.Errorf("a loader holding only the current key reads the re-sealed credential as %q", got)
	}
}

// F6's part of VERIFICATIONS' stale-write row for a re-seal. A re-seal based on a credential the
// operator has since replaced changes nothing, and the loader reads the stored credential again
// (ADR-0089).
func TestAStaleReSealIsRefused(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old := generate(t), generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "gmail", sealed(t, old, "old-token", c))
	var log logBuffer
	l := load(t, keyring(t, current, old), &log)

	reauthorized := sealed(t, old, "reauthorized-token", c)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
	scan := l.Reseal(t.Context())

	if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
		t.Error("the stale re-seal replaced the operator's credential")
	}
	if got := credentials(l.Snapshot())["personal"]; got != "reauthorized-token" {
		t.Errorf("the snapshot holds %q, want the operator's credential", got)
	}
	if !scan.Accounts["personal"] {
		t.Error("the scan reads false for a credential still on the old key")
	}
}

// VERIFICATIONS' row for copying a sealed credential. A credential copied from one account's state row
// to another's, and a client secret stored as an account's credential, do not open there, so the
// account loads as not connected. An account credential stored as a client's secret does not open
// either, so the provider has no client. Each refusal is logged without the value (ADR-0088,
// ADR-0091).
func TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	personal := sealed(t, keys, "personal-token", seal.AccountCredential("personal"))
	account(t, conn, "personal", "gmail", personal)
	account(t, conn, "copied", "gmail", personal)
	account(t, conn, "gmail", "gmail", sealed(t, keys, "client-secret", seal.ClientSecret("gmail")))
	client(t, conn, "gmail", "client-id", sealed(t, keys, "account-token", seal.AccountCredential("gmail")))
	var log logBuffer

	s := load(t, keyring(t, keys), &log).Snapshot()

	want := map[string]string{"personal": "personal-token", "copied": "not connected", "gmail": "not connected"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
	if _, ok := s.Client("gmail"); ok {
		t.Error("a client whose secret is an account credential has a client")
	}
	var refused []string
	for _, r := range log.errors(t) {
		refused = append(refused, r["account"]+r["provider"])
	}
	if diff := cmp.Diff([]string{"copied", "gmail", "gmail"}, refused, compare.Options); diff != "" {
		t.Errorf("accounts and providers logged as not opening (-want +got):\n%s", diff)
	}
	for _, value := range []string{"personal-token", "client-secret", "account-token"} {
		if strings.Contains(log.String(), value) {
			t.Errorf("the log holds %q: %s", value, log.String())
		}
	}
}
