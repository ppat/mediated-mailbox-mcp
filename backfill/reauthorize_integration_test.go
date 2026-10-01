//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
)

// provider plays Google for one account over the provider fake: it honours one refresh token, and
// refuses every call made over a port built on a source holding any other as a refused credential,
// counting the calls it refuses. Google's token endpoint is out of reach without a stand-in for it
// (ADR-0043), so the refusal is the fake's schedule, the same contract plus a schedule. When operator
// is set, the provider runs it at the first call of the operation at, before deciding that call, as
// an operator who re-authorizes or disconnects the account while the run is under way, after it took
// its account snapshot.
type provider struct {
	f        *fake.Fake
	honoured string
	at       mail.Operation
	operator func()
	refused  int
}

// connect is the connector the run builds each account's port with.
func (p *provider) connect(_ string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
	held := source.RefreshToken()
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

// refusedRun sets up the account personal, connected with the refresh token first-token through the
// installation's Gmail client, and returns what the run needs to run backfill's passes over p, which
// holds bodyMailbox's messages and honours nothing until the test says what.
func refusedRun(t *testing.T) (*provider, func(t *testing.T) error, seal.PublicKey) {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	must(t, conn, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ($1, $2, $3)",
		gmailProvider, "client-id", sealed(t, public, "client-secret", seal.ClientSecret(gmailProvider)))
	p := &provider{f: bodyMailbox(t, "personal")}
	run := func(t *testing.T) error {
		t.Helper()
		pool := backfillPool(t)
		registry := prometheus.NewRegistry()
		s, err := scan.New(scan.DefaultConfig(), "a-revision")
		if err != nil {
			t.Fatal(err)
		}
		work, err := firstPass(pool, s, registry, slog.New(slog.DiscardHandler), p.connect)
		if err != nil {
			t.Fatal(err)
		}
		return backfill(t.Context(), pool, ring, slog.New(slog.DiscardHandler), registry, work)
	}
	return p, run, public
}

// passesEnded reads whether the account's first and second passes have ended.
func passesEnded(t *testing.T, account string) (bool, bool) {
	t.Helper()
	var first, second bool
	err := superuser(t).QueryRow(t.Context(), "SELECT backfill_pass1_complete, backfill_pass2_complete FROM account_state WHERE account_id = $1", account).
		Scan(&first, &second)
	if err != nil {
		t.Fatal(err)
	}
	return first, second
}

// D1's part of VERIFICATIONS' row for picking up a replaced credential. The operator replaces the
// account's credential while a run is under way, and the provider refuses the one the run's snapshot
// opened from then on. The refused call reads the credential again from the account's row and is made
// once more with the new one, so both passes end in the run that was refused, at a call of the first
// pass and at a call of the second. The hand-over then reads the new credential's source, so the
// operator's credential stays stored as the operator wrote it (ADR-0090, ADR-0089).
func TestARefusedCallUsesTheCredentialTheOperatorStored(t *testing.T) {
	for name, at := range map[string]mail.Operation{"the first pass": mail.OpEnumerateAll, "the second pass": mail.OpGetMessageBody} {
		t.Run(name, func(t *testing.T) {
			p, run, public := refusedRun(t)
			p.honoured, p.at = "first-token", at
			conn := superuser(t)
			replaced := sealed(t, public, "second-token", seal.AccountCredential("personal"))
			p.operator = func() {
				must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", replaced)
				p.honoured = "second-token"
			}

			if err := run(t); err != nil {
				t.Fatalf("backfill returned %v, want both passes to end with the replaced credential", err)
			}

			if first, second := passesEnded(t, "personal"); !first || !second {
				t.Errorf("the first pass ended %v and the second %v, want both ended in the refused run", first, second)
			}
			if p.refused != 1 {
				t.Errorf("the provider refused %d calls, want the one call made with the replaced credential", p.refused)
			}
			if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, replaced) {
				t.Error("the run wrote a credential over the one the operator stored")
			}
		})
	}
}

// D1's part of VERIFICATIONS' row for picking up a replaced credential. A refused credential that is
// still the one the account's row holds, or an account whose row no longer holds one, ends the pass
// in the refusal after one refused call, never a second call with the refused credential (ADR-0090).
func TestARefusalOfTheStoredCredentialIsReported(t *testing.T) {
	cases := map[string]func(t *testing.T, p *provider){
		"the stored credential refused": func(*testing.T, *provider) {},
		"the account disconnected": func(t *testing.T, p *provider) {
			conn := superuser(t)
			p.honoured, p.at = "first-token", mail.OpEnumerateAll
			p.operator = func() {
				must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
				p.honoured = "nothing-honoured"
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			p, run, _ := refusedRun(t)
			p.honoured = "nothing-honoured"
			setup(t, p)

			err := run(t)

			if !errors.Is(err, mail.ErrAuthentication) {
				t.Errorf("backfill returned %v, want the refusal", err)
			}
			if p.refused != 1 {
				t.Errorf("the provider refused %d calls, want one", p.refused)
			}
			if first, _ := passesEnded(t, "personal"); first {
				t.Error("the first pass ended though every call was refused")
			}
		})
	}
}

// A refused credential whose row cannot be read again ends the pass in the refusal joined with the
// failed read, so neither is lost (ADR-0090).
func TestAFailedReadOfTheRefusedCredentialIsReported(t *testing.T) {
	p, run, _ := refusedRun(t)
	conn := superuser(t)
	p.honoured, p.at = "nothing-honoured", mail.OpEnumerateAll
	p.operator = func() { revoke(t, conn, "SELECT (credential) ON account_state") }

	err := run(t)

	if !errors.Is(err, mail.ErrAuthentication) || !strings.Contains(err.Error(), "reading the refused credential again") {
		t.Errorf("backfill returned %v, want the refusal joined with the failed read", err)
	}
	if p.refused != 1 {
		t.Errorf("the provider refused %d calls, want one", p.refused)
	}
}
