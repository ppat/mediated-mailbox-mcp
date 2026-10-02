//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
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
// household Gmail client, and returns what the run needs to run backfill's passes over p, which
// holds bodyMailbox's messages and honours nothing until the test says what.
func refusedRun(t *testing.T) (*provider, func(t *testing.T) error, seal.PublicKey) {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
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

// operatorReplaced sets up the account personal, connected with the refresh token first-token through
// the household Gmail client, and returns a run of backfill over work and a function that stores
// second-token as the operator's re-authorization would, returning the sealed bytes it stored.
func operatorReplaced(t *testing.T) (func(t *testing.T, work unitOfWork) error, func(t *testing.T) []byte, *open.Keyring) {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	run := func(t *testing.T, work unitOfWork) error {
		t.Helper()
		return backfill(t.Context(), backfillPool(t), ring, slog.New(slog.DiscardHandler), prometheus.NewRegistry(), work)
	}
	replace := func(t *testing.T) []byte {
		t.Helper()
		replaced := sealed(t, public, "second-token", seal.AccountCredential("personal"))
		must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", replaced)
		return replaced
	}
	return run, replace, ring
}

// reauthorized reads the account's refused credential again through the run's reauthorize, failing
// the test unless it built a source over the credential the operator stored.
func reauthorized(ctx context.Context, t *testing.T, s served, account string) {
	t.Helper()
	source, err := s.reauthorize(ctx, account)
	if err != nil {
		t.Fatalf("reading the refused credential again: %v", err)
	}
	if source == nil || source.RefreshToken() != "second-token" {
		t.Fatal("reading the refused credential again built no source over the one the operator stored")
	}
}

// D1's part of VERIFICATIONS' row for a write-back based on a credential the operator has since
// replaced, the re-read's half. A run that reads the operator's credential again after a refusal and
// builds its source over it hands over with the re-read's adoption stamp, so a rotation the new
// source receives afterwards is written to the account's row. Google's rotation is out of reach
// without a stand-in for its endpoint (ADR-0043), so the unit of work replaces the new source with
// one holding the token a rotation would leave (ADR-0082, ADR-0089, ADR-0090).
func TestARotationAfterARereadIsWrittenBack(t *testing.T) {
	run, replace, ring := operatorReplaced(t)
	work := func(ctx context.Context, s served) error {
		replace(t)
		reauthorized(ctx, t, s, "personal")
		s.sources["personal"] = called(s.sources["personal"], gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{
			ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "second-token-rotated",
		}))
		return s.handOver(ctx, "personal")
	}

	if err := run(t, work); err != nil {
		t.Fatalf("backfill returned %v", err)
	}

	if a, _ := loaded(t, ring, slog.New(slog.DiscardHandler)).Snapshot().Account("personal"); string(a.Credential()) != "second-token-rotated" {
		t.Errorf("after a restart the account holds %q, want the rotation the new source received", a.Credential())
	}
}

// D1's part of VERIFICATIONS' row for a write-back based on a credential the operator has since
// replaced, the stale hand-over's half. A unit of work that took its source before the loader
// adopted the operator's credential hands over that source's stamp, which the adoption moved, so the
// loader discards the hand-over and the operator's credential stays stored, even though the loader
// last knew the operator's bytes and a compare-and-set against them would land. Backfill's own
// re-read replaces the source and its stamp together, so the unit of work hands the source it took
// before the re-read back, as a run whose loader adopted a value while that unit ran would hold it,
// rotated, since an unchanged credential writes nothing whatever its stamp (ADR-0089).
func TestAHandOverFromBeforeAnAdoptionLeavesTheOperatorsCredential(t *testing.T) {
	run, replace, _ := operatorReplaced(t)
	conn := superuser(t)
	var replaced []byte
	work := func(ctx context.Context, s served) error {
		before := s.sources["personal"]
		replaced = replace(t)
		reauthorized(ctx, t, s, "personal")
		s.sources["personal"] = called(before, gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{
			ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "first-token-rotated",
		}))
		return s.handOver(ctx, "personal")
	}

	if err := run(t, work); err != nil {
		t.Fatalf("backfill returned %v, want the stale hand-over discarded without an error", err)
	}

	if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, replaced) {
		t.Error("a hand-over from before the adoption wrote its credential over the one the operator stored")
	}
}

// F6's part of VERIFICATIONS' row for an account's own client, for backfill's re-read. The account
// moves to another client while a run is under way, its new client and new credential written in one
// transaction (ADR-0106). The re-read after a refusal makes the account's source from the new client
// and the new credential, and the run holds that pair from then on (ADR-0090).
func TestARereadAfterAMoveBuildsTheSourceFromTheNewClient(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('employer', $1, 'employer-id', $2)",
		gmailProvider, sealed(t, public, "employer-secret", seal.ClientSecret("employer")))
	var source *gmail.TokenSource
	var built gmail.Credentials
	work := func(ctx context.Context, s served) error {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE accounts SET oauth_client = 'employer' WHERE account_id = 'personal'"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'",
			sealed(t, public, "moved-token", seal.AccountCredential("personal"))); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		if source, err = s.reauthorize(ctx, "personal"); err != nil {
			return err
		}
		built = s.sources["personal"].built
		return nil
	}

	if err := backfill(t.Context(), backfillPool(t), ring, slog.New(slog.DiscardHandler), prometheus.NewRegistry(), work); err != nil {
		t.Fatal(err)
	}

	if source == nil || source.RefreshToken() != "moved-token" {
		t.Fatalf("the re-read made no source holding moved-token")
	}
	want := gmail.Credentials{ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: "moved-token"}
	if diff := cmp.Diff(want, built, compare.Options); diff != "" {
		t.Errorf("the credentials the re-read's source was built from (-want +got):\n%s", diff)
	}
}
