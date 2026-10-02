//go:build integration

package accountload_test

import (
	"bytes"
	"context"
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
// no row in oauth_clients loads without a client, and one that connects through a client loads with
// it (ADR-0080, ADR-0090).
func TestAnOAuthClientIsLoadedOnlyForAProviderThatHasOne(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "client-id", sealed(t, keys, "client-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, keys, "gmail-token", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	account(t, conn, "work", "fastmail", sealed(t, keys, "fastmail-token", seal.AccountCredential("work")))
	var log logBuffer

	s := load(t, keyring(t, keys), &log).Snapshot()

	want := map[string]string{"personal": "gmail-token", "work": "fastmail-token"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
	wantClients := map[string]string{"personal": "household gmail client-id client-secret", "work": "no client"}
	if diff := cmp.Diff(wantClients, clients(s), compare.Options); diff != "" {
		t.Errorf("clients (-want +got):\n%s", diff)
	}
	if got := log.errors(t); len(got) != 0 {
		t.Errorf("error records %v", got)
	}
}

// F6's part of VERIFICATIONS' row for an account's token source built from its own client. Two
// accounts of one provider that connect through two different clients each load with their own, a
// third that shares one of them loads with it, and an account of that provider naming no client
// loads with none, though the provider has clients. A third client carries its provider's name, as
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
	var log logBuffer

	s := load(t, keyring(t, keys), &log).Snapshot()

	want := map[string]string{
		"personal": "household gmail household-id household-secret",
		"family":   "household gmail household-id household-secret",
		"work":     "employer gmail employer-id employer-secret",
		"unpaired": "no client",
	}
	if diff := cmp.Diff(want, clients(s), compare.Options); diff != "" {
		t.Errorf("clients (-want +got):\n%s", diff)
	}
	if got := log.errors(t); len(got) != 0 {
		t.Errorf("error records %v", got)
	}
}

// VERIFICATIONS' row for an account of a provider that authenticates through an OAuth client served
// without its client. Told that Gmail authenticates through a client, the loader connects a Gmail
// account through the client it names, and connects neither a Gmail account naming no client nor one
// whose client's secret does not open, though another Gmail client opened. A Fastmail account needs
// no client and is connected. Told no provider authenticates through a client, the same accounts are
// all connected, so the refusal comes from what the deployable names (ADR-0106, ADR-0090).
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

	var log logBuffer
	s := loadWith(t, keyring(t, keys), &log, []string{"gmail"}).Snapshot()

	want := map[string]string{"paired": "paired-token", "unpaired": "not connected", "unopened": "not connected", "mail": "mail-token"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials with Gmail named (-want +got):\n%s", diff)
	}
	if !strings.Contains(log.String(), `"account":"unpaired"`) || !strings.Contains(log.String(), `"account":"unopened"`) {
		t.Errorf("the accounts left not connected are not logged: %s", log.String())
	}

	s = load(t, keyring(t, keys), &log).Snapshot()

	want = map[string]string{"paired": "paired-token", "unpaired": "unpaired-token", "unopened": "unopened-token", "mail": "mail-token"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials with no provider named (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' row for an account's own client, for a credential read again. An account
// moved to another client has its new client and its new credential written in one transaction
// (ADR-0106), and a re-read after a refusal returns the account paired with the client it now names,
// a client stored after the last load included, and publishes that pair. Moved to a client whose
// secret does not open, the account is returned and published not connected (ADR-0090).
func TestARereadPairsTheCredentialWithTheClientTheAccountMovedTo(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, keys, "household-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	var log logBuffer
	l := loadWith(t, keyring(t, keys), &log, []string{"gmail"})
	client(t, conn, "employer", "gmail", "employer-id", sealed(t, keys, "employer-secret", seal.ClientSecret("employer")))
	move := func(to, token string) {
		t.Helper()
		tx, err := conn.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(t.Context(), "UPDATE accounts SET oauth_client = $1 WHERE account_id = 'personal'", to); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(t.Context(), "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'",
			sealed(t, keys, token, seal.AccountCredential("personal"))); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}

	move("employer", "moved-token")
	got, err := l.Reread(t.Context(), "personal")
	if err != nil {
		t.Fatal(err)
	}

	c, ok := got.Client()
	if !got.Connected() || string(got.Credential()) != "moved-token" || !ok || c.Name() != "employer" || c.ID() != "employer-id" || string(c.Secret()) != "employer-secret" {
		t.Errorf("the re-read returned %q through %v %q %q %q, want moved-token through employer", got.Credential(), ok, c.Name(), c.ID(), c.Secret())
	}
	if diff := cmp.Diff(map[string]string{"personal": "employer gmail employer-id employer-secret"}, clients(l.Snapshot()), compare.Options); diff != "" {
		t.Errorf("the published client (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]string{"personal": "moved-token"}, credentials(l.Snapshot()), compare.Options); diff != "" {
		t.Errorf("the published credential (-want +got):\n%s", diff)
	}

	client(t, conn, "broken", "gmail", "broken-id", sealed(t, keys, "broken-secret", seal.ClientSecret("another-row")))
	move("broken", "broken-token")
	got, err = l.Reread(t.Context(), "personal")
	if err != nil {
		t.Fatal(err)
	}

	if got.Connected() || got.Credential() != nil {
		t.Errorf("an account moved to a client that does not open is connected with %q", got.Credential())
	}
	if diff := cmp.Diff(map[string]string{"personal": "not connected"}, credentials(l.Snapshot()), compare.Options); diff != "" {
		t.Errorf("the published account after a move to a client that does not open (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' row for an account served without its client, for a client read again.
// When delta sync's re-seal loses to a client secret someone replaced and the replacement does not
// open, the accounts naming the client are published not connected rather than connected with no
// client (ADR-0106, ADR-0089).
func TestAClientReadAgainThatDoesNotOpenLeavesItsAccountsNotConnected(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old := generate(t), generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, old, "household-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, current, "personal-token", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	var log logBuffer
	l := loadWith(t, keyring(t, current, old), &log, []string{"gmail"})
	lost := func(context.Context, string, []byte, []byte) (bool, error) { return false, nil }

	must(t, conn, "UPDATE oauth_clients SET client_secret = $1 WHERE name = 'household'", sealed(t, old, "replaced-secret", seal.ClientSecret("another-row")))
	l.ResealClients(t.Context(), lost)

	if diff := cmp.Diff(map[string]string{"personal": "not connected"}, credentials(l.Snapshot()), compare.Options); diff != "" {
		t.Errorf("after a replacement that does not open (-want +got):\n%s", diff)
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
	restoreOnCleanup(t, "GRANT SELECT (name, provider, client_id, client_secret) ON oauth_clients TO "+role)
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
	must(t, conn, "GRANT SELECT (name, provider, client_id, client_secret) ON oauth_clients TO "+role)

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

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for a loader that adopted the
// replacement before the unit of work ended. A unit started at the first credential's adoption, a
// reload then adopted the operator's, and the unit hands over what it holds. The loader discards the
// hand-over, hands back the operator's credential and leaves it stored, while a unit that started at
// the adoption the loader holds still has its rotation written (ADR-0089).
func TestAHandOverFromAReplacedCredentialIsDiscarded(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "gmail", sealed(t, keys, "first-token", c))
	var log logBuffer
	l := load(t, keyring(t, keys), &log)
	first := adoption(t, l, "personal")

	reauthorized := sealed(t, keys, "reauthorized-token", c)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, held := range []string{"first-token", "rotated-from-first"} {
		use, err := l.HandOver(t.Context(), "personal", first, []byte(held))
		if err != nil || string(use) != "reauthorized-token" {
			t.Errorf("handing over %q from the replaced credential returned %q, %v, want the operator's credential", held, use, err)
		}
		if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
			t.Errorf("handing over %q from the replaced credential replaced the operator's", held)
		}
	}

	use, err := l.HandOver(t.Context(), "personal", adoption(t, l, "personal"), []byte("rotated-token"))
	if err != nil || string(use) != "rotated-token" {
		t.Errorf("a hand-over from the adoption the loader holds returned %q, %v, want the rotation", use, err)
	}
	if got, err := keyring(t, keys).Open(storedCredential(t, conn, "personal"), c); err != nil || string(got) != "rotated-token" {
		t.Errorf("the stored credential opens as %q, %v, want the rotated one", got, err)
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for the process's own earlier
// write-back. Two units start at one adoption and each sees its own rotation. The first's lands, and
// the second's lands after it by compare-and-set against the first's bytes, since no one else stored
// a value meanwhile (ADR-0082, ADR-0089). A reload after them keeps the second rotation.
func TestOverlappingRotationsEachLand(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "gmail", sealed(t, keys, "first-token", c))
	var log logBuffer
	l := load(t, keyring(t, keys), &log)
	start := adoption(t, l, "personal")

	for _, rotated := range []string{"rotated-in-one", "rotated-in-two"} {
		use, err := l.HandOver(t.Context(), "personal", start, []byte(rotated))
		if err != nil || string(use) != rotated {
			t.Errorf("handing over %q returned %q, %v, want it", rotated, use, err)
		}
		if got, err := keyring(t, keys).Open(storedCredential(t, conn, "personal"), c); err != nil || string(got) != rotated {
			t.Errorf("after handing over %q the stored credential opens as %q, %v", rotated, got, err)
		}
	}
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := credentials(l.Snapshot())["personal"]; got != "rotated-in-two" {
		t.Errorf("after a reload the account holds %q, want the later rotation", got)
	}
	if got := adoption(t, l, "personal"); got != start {
		t.Errorf("the loader's own write-backs moved the adoption from %d to %d", start, got)
	}
}

// adoption returns the account's adoption in the loader's active snapshot.
func adoption(t *testing.T, l *accountload.Loader, id string) uint64 {
	t.Helper()
	a, ok := l.Snapshot().Account(id)
	if !ok {
		t.Fatalf("the snapshot does not list %s", id)
	}
	return a.Adoption()
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
	before := adoption(t, l, "personal")
	got, err := l.Reread(t.Context(), "personal")
	if err != nil || string(got.Credential()) != "again-token" {
		t.Errorf("reading a refused credential again: %q, %v, want the stored one", got.Credential(), err)
	}
	if got.Adoption() == before || got.Adoption() != adoption(t, l, "personal") {
		t.Errorf("reading a replaced credential again returned adoption %d, from %d, want a new one the snapshot holds", got.Adoption(), before)
	}
	if got := credentials(l.Snapshot())["personal"]; got != "again-token" {
		t.Errorf("after reading again the snapshot holds %q", got)
	}
}

// F6's part of VERIFICATIONS' row for re-sealing to the current key. A credential sealed to an old key
// is sealed again to the current one and written by compare-and-set, and the scan reads true for it
// before and false after. An account whose credential cannot be opened reads true, and one with no
// state row reads false. Every row of oauth_clients has an entry keyed on the client's name, two
// clients of one provider included, and a provider with none has no entry (ADR-0092, ADR-0106).
func TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old, lost := generate(t), generate(t), generate(t)
	account(t, conn, "on-old", "gmail", sealed(t, old, "old-token", seal.AccountCredential("on-old")))
	account(t, conn, "on-current", "gmail", sealed(t, current, "current-token", seal.AccountCredential("on-current")))
	account(t, conn, "unopenable", "gmail", sealed(t, lost, "lost-token", seal.AccountCredential("unopenable")))
	listedOnly(t, conn, "no-state", "fastmail")
	client(t, conn, "household", "gmail", "household-id", sealed(t, old, "household-secret", seal.ClientSecret("household")))
	client(t, conn, "employer", "gmail", "employer-id", sealed(t, current, "employer-secret", seal.ClientSecret("employer")))
	var log logBuffer
	l := load(t, keyring(t, current, old), &log)

	before := accountload.Scan{
		Accounts: map[string]bool{"on-old": true, "on-current": false, "unopenable": true, "no-state": false},
		Clients:  map[string]bool{"household": true, "employer": false},
	}
	if diff := cmp.Diff(before, l.Scan(), compare.Options); diff != "" {
		t.Errorf("the scan after loading (-want +got):\n%s", diff)
	}
	onCurrent := storedCredential(t, conn, "on-current")

	after := l.Reseal(t.Context())

	want := accountload.Scan{
		Accounts: map[string]bool{"on-old": false, "on-current": false, "unopenable": true, "no-state": false},
		Clients:  map[string]bool{"household": true, "employer": false},
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
// either, so the account that connects through the client has none. Each refusal is logged without
// the value (ADR-0088, ADR-0091).
func TestAValueCopiedToAnotherRowOrPurposeDoesNotOpen(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	personal := sealed(t, keys, "personal-token", seal.AccountCredential("personal"))
	account(t, conn, "personal", "gmail", personal)
	account(t, conn, "copied", "gmail", personal)
	account(t, conn, "household", "gmail", sealed(t, keys, "client-secret", seal.ClientSecret("household")))
	client(t, conn, "household", "gmail", "client-id", sealed(t, keys, "account-token", seal.AccountCredential("household")))
	through(t, conn, "personal", "household")
	var log logBuffer

	s := load(t, keyring(t, keys), &log).Snapshot()

	want := map[string]string{"personal": "personal-token", "copied": "not connected", "household": "not connected"}
	if diff := cmp.Diff(want, credentials(s), compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
	if c, ok := s.Account("personal"); !ok {
		t.Error("personal is not served")
	} else if _, ok := c.Client(); ok {
		t.Error("an account connecting through a client whose secret is an account credential has a client")
	}
	var refused []string
	for _, r := range log.errors(t) {
		refused = append(refused, r["account"]+r["client"])
	}
	if diff := cmp.Diff([]string{"household", "copied", "household"}, refused, compare.Options); diff != "" {
		t.Errorf("clients and accounts logged as not opening (-want +got):\n%s", diff)
	}
	for _, value := range []string{"personal-token", "client-secret", "account-token"} {
		if strings.Contains(log.String(), value) {
			t.Errorf("the log holds %q: %s", value, log.String())
		}
	}
}
