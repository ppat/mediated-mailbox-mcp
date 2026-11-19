//go:build integration

package connections_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// F6's part of VERIFICATIONS' row for re-sealing to the current key, for the connections' Reseal. A
// credential and a client secret sealed to an old key are sealed again to the current one and written
// by compare-and-set, and the scan reads false for them after. An account whose credential cannot be
// opened reads true, and one with no state row reads false. Every row of oauth_clients has an entry
// keyed on the client's name, two clients of one provider included. A value already on the current
// key is not written, and a process holding the current key alone opens what was re-sealed (ADR-0092,
// ADR-0106).
func TestReSealingMovesCredentialsToTheCurrentKeyAndTheScanFollows(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old, lost := generate(t), generate(t), generate(t)
	account(t, conn, "on-old", "fastmail", sealed(t, old, "old-token", seal.AccountCredential("on-old")))
	account(t, conn, "on-current", "fastmail", sealed(t, current, "current-token", seal.AccountCredential("on-current")))
	account(t, conn, "unopenable", "fastmail", sealed(t, lost, "lost-token", seal.AccountCredential("unopenable")))
	listedOnly(t, conn, "no-state", "fastmail")
	client(t, conn, "household", "gmail", "household-id", sealed(t, old, "household-secret", seal.ClientSecret("household")))
	client(t, conn, "employer", "gmail", "employer-id", sealed(t, current, "employer-secret", seal.ClientSecret("employer")))
	r := start(t, current, old)
	onCurrent := storedCredential(t, conn, "on-current")
	employer := storedSecret(t, conn, "employer")

	after := r.connections.Reseal(t.Context())

	want := connections.Scan{
		Accounts: map[string]bool{"on-old": false, "on-current": false, "unopenable": true, "no-state": false},
		Clients:  map[string]bool{"household": false, "employer": false},
	}
	if diff := cmp.Diff(want, after, compare.Options); diff != "" {
		t.Errorf("the scan after re-sealing (-want +got):\n%s", diff)
	}
	if got := opens(t, conn, current, "on-old"); got != "old-token" {
		t.Errorf("the re-sealed credential opens with the current key alone as %q", got)
	}
	if got, err := keyring(t, current).Open(storedSecret(t, conn, "household"), seal.ClientSecret("household")); err != nil || string(got) != "household-secret" {
		t.Errorf("the re-sealed client secret opens with the current key alone as %q, %v", got, err)
	}
	if !bytes.Equal(storedCredential(t, conn, "on-current"), onCurrent) || !bytes.Equal(storedSecret(t, conn, "employer"), employer) {
		t.Error("a value already on the current key was written again")
	}
	if got := start(t, current).token(t, "on-old"); got != "old-token" {
		t.Errorf("a process holding only the current key reads the re-sealed credential as %q", got)
	}
}

// storedSecret reads a client's stored secret as the superuser.
func storedSecret(t *testing.T, conn *pgx.Conn, name string) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT client_secret FROM oauth_clients WHERE name = $1", name).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// F6's part of VERIFICATIONS' stale-write row for a re-seal. A re-seal based on a credential the
// operator has since replaced changes nothing, the connections read the stored credential again and
// call with it, and the scan reads true while it stays on the old key (ADR-0089).
func TestAStaleReSealIsRefused(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old := generate(t), generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "fastmail", sealed(t, old, "old-token", c))
	r := start(t, current, old)

	reauthorized := sealed(t, old, "reauthorized-token", c)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
	scan := r.connections.Reseal(t.Context())

	if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
		t.Error("the stale re-seal replaced the operator's credential")
	}
	if got := r.token(t, "personal"); got != "reauthorized-token" {
		t.Errorf("the account's token holds %q, want the operator's credential", got)
	}
	if !scan.Accounts["personal"] {
		t.Error("the scan reads false for a credential still on the old key")
	}
}

// F6's part of VERIFICATIONS' row for a re-seal that never puts back a replaced secret, and for an
// account served without its client. A client secret the operator replaced after the connections
// loaded it stays the operator's. The connections read it again and make the token of each account
// naming it from it, while an account naming another client keeps its own, and when the replacement does not open, the accounts naming the client are left not connected
// rather than connected with no client (ADR-0106, ADR-0089).
func TestAReSealNeverPutsBackAReplacedSecret(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old := generate(t), generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, old, "household-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, current, "personal-token", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	client(t, conn, "employer", "gmail", "employer-id", sealed(t, current, "employer-secret", seal.ClientSecret("employer")))
	account(t, conn, "work", "gmail", sealed(t, current, "work-token", seal.AccountCredential("work")))
	through(t, conn, "work", "employer")
	r := start(t, current, old)

	replaced := sealed(t, old, "replaced-secret", seal.ClientSecret("household"))
	must(t, conn, "UPDATE oauth_clients SET client_secret = $1 WHERE name = 'household'", replaced)
	scan := r.connections.Reseal(t.Context())
	if !bytes.Equal(storedSecret(t, conn, "household"), replaced) {
		t.Error("the stale re-seal replaced the operator's secret")
	}
	if !scan.Clients["household"] {
		t.Error("the scan reads false for a secret still on the old key")
	}
	if got := r.token(t, "personal"); got != "personal-token through household-id replaced-secret" {
		t.Errorf("the account's token is %q, want it made from the secret read again", got)
	}
	if got := r.token(t, "work"); got != "work-token through employer-id employer-secret" {
		t.Errorf("the token of an account naming another client is %q, want it made from its own client", got)
	}

	must(t, conn, "UPDATE oauth_clients SET client_secret = $1 WHERE name = 'household'", sealed(t, old, "unopenable", seal.ClientSecret("another-row")))
	r.connections.Reseal(t.Context())
	if got := r.token(t, "personal"); got != "not connected" {
		t.Errorf("after a replacement that does not open the account's token is %q, want it not connected", got)
	}
}

// F9's part of VERIFICATIONS' row for a public key matching none of the private keys. The connections
// load the public key and every private key while the process is built, so a public key matching none
// of them stops the start before anything serves (ADR-0088, ADR-0092).
func TestAPublicKeyMatchingNoPrivateKeyStopsTheStart(t *testing.T) {
	keys, other := generate(t), generate(t)
	args := append(applicationFlags(t),
		"--credential.public_key_file="+other.publicFile, "--credential.private_key_files=["+keys.privateFile+"]")
	spec := process.Spec[program]{Name: "test", Jobs: []process.Job[program]{{
		Name: "work",
		Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			s, err := sessions.Get(ctx, p, c)
			if err != nil {
				return schedule.Plan{}, err
			}
			_, err = connections.Of(ctx, p, c, s)
			return idle(), err
		},
	}}}
	// A start the keyring does not stop serves until its context ends, so the run is bounded.
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	err := process.RunContext(ctx, args, nil, io.Discard, io.Discard, spec)
	if err == nil || !strings.Contains(err.Error(), "loading the keyring") {
		t.Errorf("a start whose public key matches no private key ended with %v, want the keyring refused", err)
	}
}

// F9's part of VERIFICATIONS' row for attaching the connections. The connections attach to the
// Session once, when they are built, and a second attach is refused (ADR-0142).
func TestASecondAttachIsRefused(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	r := start(t, generate(t))
	if err := r.session.Attach(r.connections); err == nil {
		t.Error("a second attach of the connections was accepted")
	}
}

// F6's part of VERIFICATIONS' stale-write row for a re-seal, against a unit's end. The re-seal of an
// account's credential to the current key and a unit's write-back of a rotation take turns under the
// connections' lock, so with the account's row held the re-seal lands first and the end's write
// lands after it, against the re-sealed bytes, rather than reading them back as someone else's and
// losing the rotation (ADR-0089, ADR-0092).
func TestAReSealAndAnEndWhoseWritesInterleaveBothLand(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	current, old := generate(t), generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, old, "old-token", seal.AccountCredential("personal")))
	r := start(t, current, old)
	w := r.begin(t, "personal")
	token := r.provider.latest("personal")
	token.Rotate([]byte("rotated-token"))

	release := lockRow(t, "personal")
	resealed := make(chan connections.Scan, 1)
	go func() { resealed <- r.connections.Reseal(t.Context()) }()
	time.Sleep(300 * time.Millisecond)
	done := ended(t, w)
	time.Sleep(300 * time.Millisecond)
	release()
	scan := <-resealed
	if err := <-done; err != nil {
		t.Fatalf("the end returned %v", err)
	}
	if got := opens(t, conn, current, "personal"); got != "rotated-token" {
		t.Errorf("after a re-seal and an end whose writes interleaved the stored credential opens as %q, want the rotation", got)
	}
	if r.provider.latest("personal") != token {
		t.Error("the re-seal made the account's token again")
	}
	if scan.Accounts["personal"] {
		t.Error("the scan reads true for a credential the re-seal moved to the current key")
	}
}
