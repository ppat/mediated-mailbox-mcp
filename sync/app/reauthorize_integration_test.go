//go:build integration

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// provider plays Google for one account over the provider fake: it honours one refresh token, and
// refuses every call made over a port built on a source holding any other as a refused credential,
// counting the calls it refuses. Google's token endpoint is out of reach without a stand-in for it
// (ADR-0043), so the refusal is the fake's schedule, the same contract plus a schedule. When operator
// is set, the provider runs it at the first call of the operation at, before deciding that call, as
// an operator who re-authorizes or disconnects the account while a tick is under way, after it took
// its account snapshot.
type provider struct {
	f        *fake.Fake
	honoured string
	at       mail.Operation
	operator func()
	refused  int
	// unbuildable is a refresh token connect refuses to build a port over.
	unbuildable string
	// rotations maps a refresh token to the one a source built over it holds once Google rotated it,
	// and ring opens what the tick stored.
	rotations map[string]string
	ring      *open.Keyring
	// built records the credentials of every source the tick built, in order.
	built []gmail.Credentials
}

// connect is the ports the tick builds each account's port with.
func (p *provider) connect(_ string, tokens gmail.Tokens) (mail.Port[context.Context], error) {
	source, ok := tokens.(*gmail.TokenSource)
	if !ok {
		return nil, fmt.Errorf("the tick built a port over %T, want a Gmail token source", tokens)
	}
	held := source.RefreshToken()
	if held != "" && held == p.unbuildable {
		return nil, errors.New("the port could not be built")
	}
	return fake.Throttle(p.f, func(c fake.Call) error {
		if p.operator != nil && c.Op.Operation == p.at {
			operator := p.operator
			p.operator = nil
			operator()
		}
		if held != p.honoured {
			p.refused++
			return mail.ErrAuthentication
		}
		return nil
	}, time.Now), nil
}

// refusedTick sets up the account personal, connected with the refresh token first-token through the
// household Gmail client, its second pass of backfill ended, and returns what a tick needs over
// p, which holds one waiting message and honours nothing until the test says what.
func refusedTick(t *testing.T) (*provider, func(t *testing.T) error, seal.PublicKey) {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	p := &provider{ring: ring, f: mailbox(t, "personal", fake.Message{
		Metadata: mail.MessageMetadata{ID: "m1", ThreadID: "t1", From: mail.Address{Email: "orders@shop.example"}, Subject: "Your order", Date: mail.UnixMilli(time.Now().UnixMilli())},
		Body:     mail.MessageBody{Text: "Thanks for your order."},
	})}
	run := func(t *testing.T) error {
		t.Helper()
		s := newTestSyncer(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler), nil)
		s.ports = p.connect
		s.source = func(c gmail.Credentials) *gmail.TokenSource {
			p.built = append(p.built, c)
			if rotated, ok := p.rotations[c.RefreshToken]; ok {
				c.RefreshToken = rotated
			}
			return gmail.NewTokenSource(http.DefaultClient, c)
		}
		return s.tick(t.Context())
	}
	return p, run, public
}

// scanState reads the state of the account personal's message m1, empty when it is not stored.
func scanState(t *testing.T) string {
	t.Helper()
	var state string
	err := superuser(t).QueryRow(t.Context(), "SELECT coalesce((SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'm1'), '')").Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// D4's part of VERIFICATIONS' row for picking up a replaced credential. The operator replaces the
// account's credential while a tick is under way, and the provider refuses the one the tick's snapshot
// opened from then on. The refused call reads the credential again from the account's row and is made
// once more with the new one, so the tick ends having indexed and scanned the message, whether the
// refusal came at its first call or at a body fetch. The hand-over then reads the new credential's
// source, so the operator's credential stays stored as the operator wrote it (ADR-0090, ADR-0089).
func TestARefusedCallUsesTheCredentialTheOperatorStored(t *testing.T) {
	for name, at := range map[string]mail.Operation{"the first call": mail.OpCurrentCursor, "a body fetch": mail.OpGetMessageBody} {
		t.Run(name, func(t *testing.T) {
			p, run, public := refusedTick(t)
			p.honoured, p.at = "first-token", at
			conn := superuser(t)
			replaced := sealed(t, public, "second-token", seal.AccountCredential("personal"))
			p.operator = func() {
				must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", replaced)
				p.honoured = "second-token"
			}

			if err := run(t); err != nil {
				t.Fatalf("the tick returned %v, want it to end with the replaced credential", err)
			}

			if state := scanState(t); state != "scanned" {
				t.Errorf("the message is %q, want scanned in the refused tick", state)
			}
			if p.refused != 1 {
				t.Errorf("the provider refused %d calls, want the one call made with the replaced credential", p.refused)
			}
			if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, replaced) {
				t.Error("the tick wrote a credential over the one the operator stored")
			}
		})
	}
}

// D4's part of VERIFICATIONS' row for picking up a replaced credential. A refused credential that is
// still the one the account's row holds, or an account whose row no longer holds one, ends the tick in
// the refusal after one refused call, never a second call with the refused credential (ADR-0090).
func TestARefusalOfTheStoredCredentialIsReported(t *testing.T) {
	cases := map[string]func(t *testing.T, p *provider){
		"the stored credential refused": func(*testing.T, *provider) {},
		"the account disconnected": func(t *testing.T, p *provider) {
			conn := superuser(t)
			p.honoured, p.at = "first-token", mail.OpCurrentCursor
			p.operator = func() {
				must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
				p.honoured = "nothing-honoured"
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			p, run, _ := refusedTick(t)
			p.honoured = "nothing-honoured"
			setup(t, p)

			err := run(t)

			if !errors.Is(err, mail.ErrAuthentication) {
				t.Errorf("the tick returned %v, want the refusal", err)
			}
			if p.refused != 1 {
				t.Errorf("the provider refused %d calls, want one", p.refused)
			}
			if state := scanState(t); state != "" {
				t.Errorf("the message is %q though every call was refused, want it not stored", state)
			}
		})
	}
}

// A refused credential whose row cannot be read again ends the tick in the refusal joined with the
// failed read, so neither is lost (ADR-0090).
func TestAFailedReadOfTheRefusedCredentialIsReported(t *testing.T) {
	p, run, _ := refusedTick(t)
	conn := superuser(t)
	p.honoured, p.at = "nothing-honoured", mail.OpCurrentCursor
	p.operator = func() { revoke(t, conn, "SELECT (credential) ON account_state") }

	err := run(t)

	if !errors.Is(err, mail.ErrAuthentication) || !strings.Contains(err.Error(), "reading the refused credential again") {
		t.Errorf("the tick returned %v, want the refusal joined with the failed read", err)
	}
	if p.refused != 1 {
		t.Errorf("the provider refused %d calls, want one", p.refused)
	}
}

// D4's part of VERIFICATIONS' row for a write-back based on a credential the operator has since
// replaced. A tick hands over with the adoption stamp of the credential it holds. When the tick reads
// the operator's credential again after a refusal and then fails to build its port over it, it still
// holds the refused one, whose stamp the re-read moved, so the loader discards the hand-over and the
// operator's credential stays stored, rather than the refused one being written back over it
// (ADR-0089, ADR-0090).
func TestATickWhoseRebuildFailsLeavesTheReauthorization(t *testing.T) {
	p, run, public := refusedTick(t)
	p.honoured, p.at, p.unbuildable = "first-token", mail.OpCurrentCursor, "second-token"
	conn := superuser(t)
	replaced := sealed(t, public, "second-token", seal.AccountCredential("personal"))
	p.operator = func() {
		must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", replaced)
		p.honoured = "second-token"
	}

	if err := run(t); err == nil {
		t.Fatal("the tick succeeded though it could not build its port over the replaced credential")
	}

	if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, replaced) {
		t.Error("the tick wrote the refused credential back over the one the operator stored")
	}
}

// D4's part of VERIFICATIONS' row for a write-back based on a credential the operator has since
// replaced, the re-read's half. A tick that reads the operator's credential again after a refusal and
// builds its source over it hands over with the re-read's adoption stamp, so a rotation the rebuilt
// source received during the rest of the tick is written to the account's row. Google's rotation is
// out of reach without a stand-in for its endpoint (ADR-0043), so the rebuilt source is built holding
// the token a rotation would leave (ADR-0082, ADR-0089, ADR-0090).
func TestARotationAfterARereadIsWrittenBack(t *testing.T) {
	p, run, public := refusedTick(t)
	p.honoured, p.at = "first-token", mail.OpCurrentCursor
	p.rotations = map[string]string{"second-token": "second-token-rotated"}
	conn := superuser(t)
	p.operator = func() {
		must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'",
			sealed(t, public, "second-token", seal.AccountCredential("personal")))
		p.honoured = "second-token-rotated"
	}

	if err := run(t); err != nil {
		t.Fatalf("the tick returned %v, want it to end with the rotated credential", err)
	}

	restarted := accountload.New(syncPool(t), p.ring, slog.New(slog.DiscardHandler), []string{gmailProvider})
	if err := restarted.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if a, _ := restarted.Snapshot().Account("personal"); string(a.Credential()) != "second-token-rotated" {
		t.Errorf("after a restart the account holds %q, want the rotation the rebuilt source received", a.Credential())
	}
}

// F6's part of VERIFICATIONS' row for an account's own client, for delta sync's re-read. The account
// moves to another client while a tick is under way, its new client and new credential written in
// one transaction (ADR-0106). The provider refuses the old credential, and the call made again is
// built from the new client and the new credential (ADR-0090).
func TestARereadAfterAMoveBuildsTheSourceFromTheNewClient(t *testing.T) {
	p, run, public := refusedTick(t)
	p.honoured, p.at = "first-token", mail.OpCurrentCursor
	conn := superuser(t)
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('employer', $1, 'employer-id', $2)",
		gmailProvider, sealed(t, public, "employer-secret", seal.ClientSecret("employer")))
	moved := sealed(t, public, "moved-token", seal.AccountCredential("personal"))
	p.operator = func() {
		tx, err := conn.Begin(context.Background())
		if err != nil {
			t.Error(err)
			return
		}
		for _, sql := range []string{"UPDATE accounts SET oauth_client = 'employer' WHERE account_id = 'personal'", "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'"} {
			var args []any
			if strings.Contains(sql, "$1") {
				args = append(args, moved)
			}
			if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
				t.Error(err)
			}
		}
		if err := tx.Commit(context.Background()); err != nil {
			t.Error(err)
		}
		p.honoured = "moved-token"
	}

	if err := run(t); err != nil {
		t.Fatalf("the tick returned %v, want it to end through the new client", err)
	}

	want := []gmail.Credentials{
		{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "first-token"},
		{ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: "moved-token"},
	}
	if diff := cmp.Diff(want, p.built, compare.Options); diff != "" {
		t.Errorf("the credentials the tick's sources were built from (-want +got):\n%s", diff)
	}
}
