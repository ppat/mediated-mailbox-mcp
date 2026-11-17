//go:build integration

package connections_test

import (
	"bytes"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/accounts"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// noBound is the zero time, a unit of work whose leases wait with no bound.
var noBound time.Time

// opens returns what the account's stored credential opens as with keys.
func opens(t *testing.T, conn *pgx.Conn, keys keyPair, account string) string {
	t.Helper()
	got, err := keyring(t, keys).Open(storedCredential(t, conn, account), seal.AccountCredential(account))
	if err != nil {
		t.Fatalf("the stored credential of %s does not open: %v", account, err)
	}
	return string(got)
}

// call makes one call through the unit's mailbox and returns its error.
func call(t *testing.T, w *connections.Work) error {
	t.Helper()
	_, err := w.Mailbox().ListLabels(t.Context())
	return err
}

// end ends the unit of work and fails the test on an error of either kind.
func end(t *testing.T, w *connections.Work) {
	t.Helper()
	if writeBack, record := w.End(t.Context()); writeBack != nil || record != nil {
		t.Fatalf("ending the unit of work: %v, %v", writeBack, record)
	}
}

// F6's part of VERIFICATIONS' row for forcing a rotation and restarting. At the end of a unit of work
// the token's credential is written back. One equal to the credential last read or written writes
// nothing, a rotated one is written back once, and it is what a new process, as after a restart,
// loads (ADR-0082).
func TestARotatedCredentialSurvivesARestart(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	original := sealed(t, keys, "original-token", seal.AccountCredential("personal"))
	account(t, conn, "personal", "fastmail", original)
	r := start(t, keys)

	w := r.begin(t, "personal")
	if err := call(t, w); err != nil {
		t.Fatal(err)
	}
	end(t, w)
	if !bytes.Equal(storedCredential(t, conn, "personal"), original) {
		t.Error("an unchanged credential was written")
	}
	w = r.begin(t, "personal")
	if err := call(t, w); err != nil {
		t.Fatal(err)
	}
	r.provider.latest("personal").Rotate([]byte("rotated-token"))
	end(t, w)
	written := storedCredential(t, conn, "personal")
	if got := opens(t, conn, keys, "personal"); got != "rotated-token" {
		t.Errorf("the stored credential opens as %q, want the rotated one", got)
	}
	end(t, r.begin(t, "personal"))
	if !bytes.Equal(storedCredential(t, conn, "personal"), written) {
		t.Error("a rotated credential already written was written again")
	}

	restarted := start(t, keys)
	if got := restarted.token(t, "personal"); got != "rotated-token" {
		t.Errorf("after a restart the account's token holds %q, want the rotated credential", got)
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set. After the operator replaces an
// account's credential, the write-back of a rotation of the credential the connections read before
// changes nothing, and the connections read the stored credential again, so the next unit of work
// calls with the operator's. A rotation of that one is written back by compare-and-set against the
// bytes the operator stored (ADR-0089).
func TestAStaleWriteBackIsRefusedAndTheStoredCredentialUsed(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", c))
	r := start(t, keys)

	w := r.begin(t, "personal")
	reauthorized := sealed(t, keys, "reauthorized-token", c)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
	r.provider.latest("personal").Rotate([]byte("rotated-token"))
	end(t, w)
	if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
		t.Error("the stale write-back replaced the operator's credential")
	}
	if got := r.token(t, "personal"); got != "reauthorized-token" {
		t.Errorf("the next unit of work calls with %q, want the operator's credential", got)
	}

	w = r.begin(t, "personal")
	if err := call(t, w); err != nil {
		t.Fatal(err)
	}
	r.provider.latest("personal").Rotate([]byte("next-rotated-token"))
	end(t, w)
	if got := opens(t, conn, keys, "personal"); got != "next-rotated-token" {
		t.Errorf("the stored credential opens as %q, want the rotation written on the bytes the operator stored", got)
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for connections that adopted
// the replacement before the unit of work ended. A unit began over the first credential's token, a
// reload then adopted the operator's, and the unit's own token rotated. The unit's end reads the
// record's token, made from the operator's credential, with the record's adoption stamp, so it
// writes nothing and the operator's credential stays stored, while a unit that began after the
// adoption still has its rotation written (ADR-0089).
func TestAnEndAfterAnAdoptionLeavesTheOperatorsCredential(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", c))
	r := start(t, keys)

	before := r.begin(t, "personal")
	first := r.provider.latest("personal")
	reauthorized := sealed(t, keys, "reauthorized-token", c)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
	r.reload(t)
	first.Rotate([]byte("rotated-from-first"))
	end(t, before)
	if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
		t.Error("the end of a unit of work begun before the adoption replaced the operator's credential")
	}

	after := r.begin(t, "personal")
	if err := call(t, after); err != nil {
		t.Fatal(err)
	}
	if got := string(r.provider.latest("personal").Credential()); got != "reauthorized-token" {
		t.Fatalf("a unit of work after the adoption calls with %q", got)
	}
	r.provider.latest("personal").Rotate([]byte("rotated-token"))
	end(t, after)
	if got := opens(t, conn, keys, "personal"); got != "rotated-token" {
		t.Errorf("the stored credential opens as %q, want the rotation of the adopted credential", got)
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for the process's own earlier
// write-back. Two units of work begin on one account and share its token, which rotates in each. The
// first's end lands its rotation, and the second's lands after it by compare-and-set against the
// first's bytes, since no one else stored a value meanwhile (ADR-0082, ADR-0089). A reload after them
// keeps the token, which holds the second rotation.
func TestOverlappingRotationsEachLand(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	r := start(t, keys)

	one, two := r.begin(t, "personal"), r.begin(t, "personal")
	token := r.provider.latest("personal")
	for _, u := range []struct {
		w       *connections.Work
		rotated string
	}{{one, "rotated-in-one"}, {two, "rotated-in-two"}} {
		token.Rotate([]byte(u.rotated))
		end(t, u.w)
		if got := opens(t, conn, keys, "personal"); got != u.rotated {
			t.Errorf("after ending the unit that saw %q the stored credential opens as %q", u.rotated, got)
		}
	}
	r.reload(t)
	if got := r.token(t, "personal"); got != "rotated-in-two" {
		t.Errorf("after a reload the account's token holds %q, want the later rotation", got)
	}
	if r.provider.latest("personal") != token {
		t.Error("a reload made the account's token again though the stored credential is the one it pairs with")
	}

	// One unit's end runs whole between another's read of the credential and its write. The
	// process's own write-back leaves the stamp as it is, so the later write lands on its bytes.
	three, four := r.begin(t, "personal"), r.begin(t, "personal")
	token.Rotate([]byte("rotated-in-three"))
	token.AtAttemptRead(func() {
		token.AtAttemptRead(nil)
		token.Rotate([]byte("rotated-in-four"))
		end(t, four)
		if got := opens(t, conn, keys, "personal"); got != "rotated-in-four" {
			t.Errorf("after the inner end the stored credential opens as %q, want its rotation", got)
		}
	})
	end(t, three)
	if got := opens(t, conn, keys, "personal"); got != "rotated-in-three" {
		t.Errorf("after an end around another's the stored credential opens as %q, want the outer end's rotation landed too", got)
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for an adoption between a
// unit's read and its write. A unit's end reads its token's credential and the record's adoption
// stamp, the connections then adopt the operator's credential by a reload, and the end writes
// nothing, since the stamp it read has moved, so the operator's credential stays stored (ADR-0089).
func TestAnAdoptionBetweenAnEndsReadAndItsWriteDiscardsTheWriteBack(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", c))
	r := start(t, keys)

	w := r.begin(t, "personal")
	first := r.provider.latest("personal")
	first.Rotate([]byte("rotated-from-first"))
	reauthorized := sealed(t, keys, "reauthorized-token", c)
	first.AtAttemptRead(func() {
		first.AtAttemptRead(nil)
		must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", reauthorized)
		r.reload(t)
	})
	end(t, w)
	if !bytes.Equal(storedCredential(t, conn, "personal"), reauthorized) {
		t.Errorf("an end whose stamp moved before its write replaced the operator's credential with %q", opens(t, conn, keys, "personal"))
	}
	if got := r.token(t, "personal"); got != "reauthorized-token" {
		t.Errorf("after the adoption the account's token holds %q, want the operator's credential", got)
	}
}

// VERIFICATIONS' row for a failed write-back, and F6's part of the reload row. A write-back that fails
// is logged without the credential and returned, and the record keeps the rotated credential in its
// token, which a reload keeps while the stored bytes stay the ones the connections knew. The next
// unit's end writes it (ADR-0082, ADR-0090).
func TestAFailedWriteBackIsHeldLoggedAndKeptByAReload(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "original-token", seal.AccountCredential("personal")))
	r := start(t, keys)

	must(t, conn, "REVOKE UPDATE ON account_state FROM "+role)
	restoreOnCleanup(t, "GRANT UPDATE (credential, last_auth_at, last_auth_outcome) ON account_state TO "+role)
	w := r.begin(t, "personal")
	r.provider.latest("personal").Rotate([]byte("rotated-token"))
	if writeBack, record := w.End(t.Context()); writeBack == nil || record != nil {
		t.Fatalf("ending with the write-back refused returned %v, %v, want the write-back's failure alone", writeBack, record)
	}
	want := []map[string]string{{"msg": "a rotated credential was not written back, so a restart before a later write-back lands loses the account's access", "account": "personal"}}
	if diff := cmp.Diff(want, r.logs.records(t, "ERROR"), compare.Options); diff != "" {
		t.Errorf("error records (-want +got):\n%s", diff)
	}
	if strings.Contains(r.logs.String(), "rotated-token") {
		t.Error("the log holds the credential")
	}
	r.reload(t)
	if got := r.token(t, "personal"); got != "rotated-token" {
		t.Errorf("after a reload the account's token holds %q, want the rotated credential whose write-back failed", got)
	}

	must(t, conn, "GRANT UPDATE (credential, last_auth_at, last_auth_outcome) ON account_state TO "+role)
	end(t, r.begin(t, "personal"))
	if got := opens(t, conn, keys, "personal"); got != "rotated-token" {
		t.Errorf("the stored credential opens as %q, want the rotated one", got)
	}
}

// F6's part of VERIFICATIONS' reload row. A reload after the operator replaced a credential whose
// rotation's write-back failed takes the stored one, and a credential the provider refuses is read
// again from its row, the call made once more over it, and the units of work after it call with it
// (ADR-0089, ADR-0090).
func TestAStoredCredentialSomeoneElseReplacedWins(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "fastmail", sealed(t, keys, "original-token", c))
	r := start(t, keys)

	must(t, conn, "REVOKE UPDATE ON account_state FROM "+role)
	restoreOnCleanup(t, "GRANT UPDATE (credential, last_auth_at, last_auth_outcome) ON account_state TO "+role)
	w := r.begin(t, "personal")
	r.provider.latest("personal").Rotate([]byte("rotated-token"))
	if writeBack, record := w.End(t.Context()); writeBack == nil || record != nil {
		t.Fatalf("ending with the write-back refused returned %v, %v, want the write-back's failure alone", writeBack, record)
	}
	must(t, conn, "GRANT UPDATE (credential, last_auth_at, last_auth_outcome) ON account_state TO "+role)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, refusedPrefix+"-reauthorized", c))
	r.reload(t)
	if got := string(r.provider.tokenOf(t, r, "personal")); got != refusedPrefix+"-reauthorized" {
		t.Errorf("after a reload the account's token holds %q, want the operator's credential", got)
	}

	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, "again-token", c))
	w = r.begin(t, "personal")
	if err := call(t, w); err != nil {
		t.Errorf("a call refused over a credential the operator replaced answered %v, want it made once more over the stored one", err)
	}
	if got := string(r.provider.latest("personal").Credential()); got != "again-token" {
		t.Errorf("the call was made once more over %q, want the stored credential", got)
	}
	if got := string(r.provider.tokenOf(t, r, "personal")); got != "again-token" {
		t.Errorf("the unit of work after the re-read begins over %q, want the credential read again", got)
	}
}

// tokenOf begins a unit of work on the account and returns the credential its token holds.
func (p *provided) tokenOf(t *testing.T, r *running, account string) []byte {
	t.Helper()
	r.begin(t, account)
	return p.latest(account).Credential()
}

// F6's part of VERIFICATIONS' row for an account's own client, for a credential read again. An
// account moved to another client has its new client and its new credential written in one
// transaction (ADR-0106). A credential the provider refuses is read again from the account's row
// with the client it now names, a client stored after the last reload included, and the call is made
// once more over a token made from that pair, which the units after it call with. A re-read that
// finds the refused credential through the same client leaves the refusal standing after one call
// more, and one that finds the account moved to a client whose secret does not open leaves it
// standing and the account not connected (ADR-0090).
func TestAReadAgainMakesTheTokenFromTheClientTheAccountMovedTo(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, keys, "household-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, keys, refusedPrefix+"-first", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	r := start(t, keys)
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

	w := r.begin(t, "personal")
	before := r.provider.built("personal")
	if err := call(t, w); !errors.Is(err, coremail.ErrAuthentication) {
		t.Fatalf("a call over the refused credential with nothing replaced answered %v, want the refusal", err)
	}
	if got := r.provider.built("personal") - before; got != 0 {
		t.Errorf("a re-read finding the refused credential made %d tokens again, want none", got)
	}

	move("employer", "moved-token")
	if err := call(t, w); err != nil {
		t.Fatalf("a call refused after the account moved answered %v, want it made once more", err)
	}
	r.begin(t, "personal")
	if tok := r.provider.latest("personal"); string(tok.Credential()) != "moved-token" || tok.Client().ID != "employer-id" {
		t.Errorf("the unit of work after the re-read begins over %q through %q, want the moved credential through its new client", tok.Credential(), tok.Client().ID)
	}

	client(t, conn, "broken", "gmail", "broken-id", sealed(t, keys, "broken-secret", seal.ClientSecret("another-row")))
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, refusedPrefix+"-moved", seal.AccountCredential("personal")))
	r.reload(t)
	w = r.begin(t, "personal")
	move("broken", "broken-token")
	if err := call(t, w); !errors.Is(err, coremail.ErrAuthentication) {
		t.Errorf("a call refused after a move to a client that does not open answered %v, want the refusal", err)
	}
	if got := r.token(t, "personal"); got != "not connected" {
		t.Errorf("after a move to a client that does not open the account's token is %q, want it not connected", got)
	}
}

// F10's part of VERIFICATIONS' row for a held token, now the record's. An account's token serves its
// units of work across reloads while the stored credential is the one it pairs with, so a token the
// provider refreshed serves the units after it. It is made again when the stored credential changes,
// and when the account's client changes, its identifier or its secret, with the credential the token
// held (ADR-0089, ADR-0106).
func TestATokenServesItsAccountUntilItsCredentialOrClientChanges(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	client(t, conn, "household", "gmail", "household-id", sealed(t, keys, "household-secret", seal.ClientSecret("household")))
	account(t, conn, "personal", "gmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	through(t, conn, "personal", "household")
	r := start(t, keys)

	r.begin(t, "personal")
	first := r.provider.latest("personal")
	r.reload(t)
	r.begin(t, "personal")
	if r.provider.latest("personal") != first {
		t.Error("a reload with nothing changed made the account's token again")
	}

	must(t, conn, "UPDATE oauth_clients SET client_secret = $1 WHERE name = 'household'", sealed(t, keys, "rotated-secret", seal.ClientSecret("household")))
	r.reload(t)
	if got := r.token(t, "personal"); got != "first-token through household-id rotated-secret" {
		t.Errorf("after the client's secret changed the token is %q, want it made again from the new secret", got)
	}
	second := r.provider.latest("personal")

	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, "replaced-token", seal.AccountCredential("personal")))
	r.reload(t)
	if got := r.token(t, "personal"); got != "replaced-token through household-id rotated-secret" || r.provider.latest("personal") == second {
		t.Errorf("after the credential was replaced the token is %q, want it made again from the stored credential", got)
	}
}

// F9's part of VERIFICATIONS' row for every caller sharing the connections' records. Units of work
// begun through two callers, as two job kinds of a process each build one, call with the account's
// one token, so a rotation one sees is the one the other writes back (ADR-0089, ADR-0117).
func TestUnitsOfWorkThroughTwoCallersShareTheAccountsToken(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	r := start(t, keys)

	r.begin(t, "personal")
	first := r.provider.latest("personal")
	w, err := r.connections.Begin("personal", r.other, noBound)
	if err != nil {
		t.Fatal(err)
	}
	if r.provider.latest("personal") != first {
		t.Error("a unit of work through another caller calls with a token of its own")
	}
	first.Rotate([]byte("rotated-token"))
	end(t, w)
	if got := opens(t, conn, keys, "personal"); got != "rotated-token" {
		t.Errorf("the stored credential opens as %q, want the rotation the first caller's token saw", got)
	}
}

// VERIFICATIONS' rows for the attempt recorded at the end of a unit of work. A token that made no
// attempt records nothing, each outcome is recorded as the token reported it with the instant it
// started, and a recording that fails is returned apart from the write-back (ADR-0097).
func TestTheEndRecordsTheTokensLatestAttempt(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	r := start(t, keys)
	recorded := func() (time.Time, string) {
		t.Helper()
		var at *time.Time
		var outcome *string
		if err := conn.QueryRow(t.Context(), "SELECT last_auth_at, last_auth_outcome FROM account_state WHERE account_id = 'personal'").Scan(&at, &outcome); err != nil {
			t.Fatal(err)
		}
		if at == nil || outcome == nil {
			return time.Time{}, ""
		}
		return at.UTC(), *outcome
	}

	end(t, r.begin(t, "personal"))
	if at, outcome := recorded(); !at.IsZero() || outcome != "" {
		t.Errorf("a token that made no attempt recorded %v %q", at, outcome)
	}
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	for i, outcome := range []coremail.AuthOutcome{coremail.AuthSucceeded, coremail.AuthRefused, coremail.AuthFailed} {
		w := r.begin(t, "personal")
		when := at.Add(time.Duration(i) * time.Minute)
		r.provider.latest("personal").Attempted(coremail.AuthAttempt{At: coremail.UnixMilli(when.UnixMilli()), Outcome: outcome})
		end(t, w)
		if gotAt, gotOutcome := recorded(); !gotAt.Equal(when) || gotOutcome != string(outcome) {
			t.Errorf("recorded %v %q, want %v %q", gotAt, gotOutcome, when, outcome)
		}
	}
	// An attempt older than the one recorded, as a unit of work that began before another's ends after
	// it, changes nothing, and a later one replaces it (ADR-0097).
	latest := at.Add(2 * time.Minute)
	w := r.begin(t, "personal")
	r.provider.latest("personal").Attempted(coremail.AuthAttempt{At: coremail.UnixMilli(at.Add(-time.Minute).UnixMilli()), Outcome: coremail.AuthSucceeded})
	end(t, w)
	if gotAt, gotOutcome := recorded(); !gotAt.Equal(latest) || gotOutcome != string(coremail.AuthFailed) {
		t.Errorf("after an older attempt recorded %v %q, want the later %v %q kept", gotAt, gotOutcome, latest, coremail.AuthFailed)
	}
	later := at.Add(3 * time.Minute)
	w = r.begin(t, "personal")
	r.provider.latest("personal").Attempted(coremail.AuthAttempt{At: coremail.UnixMilli(later.UnixMilli()), Outcome: coremail.AuthRefused})
	end(t, w)
	if gotAt, gotOutcome := recorded(); !gotAt.Equal(later) || gotOutcome != string(coremail.AuthRefused) {
		t.Errorf("after a later attempt recorded %v %q, want %v %q", gotAt, gotOutcome, later, coremail.AuthRefused)
	}

	must(t, conn, "REVOKE UPDATE ON account_state FROM "+role)
	restoreOnCleanup(t, "GRANT UPDATE (credential, last_auth_at, last_auth_outcome) ON account_state TO "+role)
	w = r.begin(t, "personal")
	r.provider.latest("personal").Attempted(coremail.AuthAttempt{At: coremail.UnixMilli(at.Add(time.Hour).UnixMilli()), Outcome: coremail.AuthSucceeded})
	if writeBack, record := w.End(t.Context()); writeBack != nil || record == nil {
		t.Errorf("ending with the recording refused returned %v, %v, want the recording's failure alone", writeBack, record)
	}
}

// targetRate returns the target an account's rate state was last written at, in the fake's units a
// second, by a limiter that spent for it, to a thousandth of a unit since the fraction is stored as a
// real.
func targetRate(t *testing.T, conn *pgx.Conn, account string) float64 {
	t.Helper()
	var target float64
	if err := conn.QueryRow(t.Context(), "SELECT target_rate FROM rate_state WHERE account_id = $1", account).Scan(&target); err != nil {
		t.Fatal(err)
	}
	return math.Round(target*1000) / 1000
}

// VERIFICATIONS' row for an account's lowered target, and the Session's failure table for a target
// that cannot be read. Each account's calls lease under the lowered target its state row held at the
// latest reload, a target changed between reloads applying from the next, and every other account's
// under the default target. A stored target above half the ceiling refuses the account's unit of work,
// naming the account, and one that cannot be read refuses it too, while every other account's units
// begin (ADR-0024, ADR-0090).
func TestEachAccountLeasesUnderItsLoweredTarget(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	must(t, conn, "DELETE FROM rate_grants")
	must(t, conn, "DELETE FROM rate_state")
	keys := generate(t)
	for _, id := range []string{"lowered", "default", "raised"} {
		account(t, conn, id, "fastmail", sealed(t, keys, id+"-token", seal.AccountCredential(id)))
	}
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.2 WHERE account_id = 'lowered'")
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.6 WHERE account_id = 'raised'")
	r := start(t, keys)
	spend := func(account string) float64 {
		t.Helper()
		if err := call(t, r.begin(t, account)); err != nil {
			t.Fatal(err)
		}
		return targetRate(t, conn, account)
	}

	got := map[string]float64{"default": spend("default"), "lowered": spend("lowered")}
	if diff := cmp.Diff(map[string]float64{"default": 50, "lowered": 20}, got, compare.Options); diff != "" {
		t.Errorf("the targets each account leases under, in the fake's units a second (-want +got):\n%s", diff)
	}
	if _, err := r.connections.Begin("raised", r.caller, noBound); err == nil || !strings.Contains(err.Error(), `"raised"`) {
		t.Errorf("a unit of work on the raised account began with %v, want the target refused for the account", err)
	}

	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.3 WHERE account_id = 'lowered'")
	if got := spend("lowered"); got != 20 {
		t.Errorf("before the next reload the account leases under %v, want the target read at the last", got)
	}
	r.reload(t)
	if got := spend("lowered"); got != 30 {
		t.Errorf("after the target was raised to 0.3 the next reload's units lease under %v, want 30", got)
	}

	must(t, conn, "REVOKE SELECT ON account_state FROM "+role)
	restoreOnCleanup(t, "GRANT SELECT ON account_state TO "+role)
	must(t, conn, "GRANT SELECT (account_id, credential) ON account_state TO "+role)
	r.reload(t)
	if _, err := r.connections.Begin("lowered", r.caller, noBound); err == nil || !strings.Contains(err.Error(), "lowered target") {
		t.Errorf("a unit of work on an account whose target could not be read began with %v, want it refused", err)
	}
}

// The row the Session's member list keys a unit of work to. No unit of work begins on an account the
// list in force does not hold, never listed, or listed and dropped by a later reload, and the Session
// no longer holds it workable (ADR-0090).
func TestNoUnitOfWorkBeginsOnAnAccountNotInForce(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "personal-token", seal.AccountCredential("personal")))
	r := start(t, keys)
	if _, err := r.connections.Begin("never-listed", r.caller, noBound); !errors.Is(err, connections.ErrNotConnected) {
		t.Errorf("a unit of work on an account never listed began with %v, want ErrNotConnected", err)
	}
	w := r.begin(t, "personal")

	// The connections load ahead of the list's being put in force, so they can hold an account the
	// list in force does not yet hold.
	account(t, conn, "ahead", "fastmail", sealed(t, keys, "ahead-token", seal.AccountCredential("ahead")))
	if err := r.connections.Load(t.Context(), append(r.session.Accounts(), accounts.Account{ID: "ahead", Provider: "fastmail"})); err != nil {
		t.Fatal(err)
	}
	if !r.connections.Connected("ahead") {
		t.Fatal("the connections did not load the account ahead of the list in force")
	}
	if _, err := r.connections.Begin("ahead", r.caller, noBound); !errors.Is(err, connections.ErrNotConnected) {
		t.Errorf("a unit of work on an account the list in force does not yet hold began with %v, want ErrNotConnected", err)
	}

	forget(t, conn, "personal")
	r.reload(t)
	if _, err := r.connections.Begin("personal", r.caller, noBound); !errors.Is(err, connections.ErrNotConnected) {
		t.Errorf("a unit of work on an account dropped began with %v, want ErrNotConnected", err)
	}
	if got, err := r.session.Workable(); err != nil || slices.Contains(got, "personal") {
		t.Errorf("the Session holds %v workable (%v), the account dropped among them", got, err)
	}
	if writeBack, record := w.End(t.Context()); writeBack != nil && !strings.Contains(writeBack.Error(), "personal") || record != nil {
		t.Errorf("ending a unit of work on an account dropped returned %v, %v, want no error or one naming the account", writeBack, record)
	}
}

// lockRow holds the account's state row in a transaction of its own, so every compare-and-set on it
// waits, and returns the function that releases it.
func lockRow(t *testing.T, account string) func() {
	t.Helper()
	conn := superuser(t)
	blocker, err := conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocker.Exec(t.Context(), "SELECT 1 FROM account_state WHERE account_id = $1 FOR UPDATE", account); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := blocker.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
}

// ended runs the unit's end on a goroutine of its own and returns where its errors arrive.
func ended(t *testing.T, w *connections.Work) <-chan error {
	t.Helper()
	out := make(chan error, 1)
	go func() {
		writeBack, record := w.End(t.Context())
		out <- errors.Join(writeBack, record)
	}()
	return out
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for the process's own
// write-backs whose compare-and-sets interleave. Two units of one account end at once, each having
// read its own rotation, while the account's row is held, so the first write waits at the database.
// The write-backs take turns under the connections' lock, so the later compares against the earlier's
// bytes and both land, and the account keeps its token (ADR-0089).
func TestTwoEndsWhoseWritesInterleaveBothLand(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	r := start(t, keys)
	one, two := r.begin(t, "personal"), r.begin(t, "personal")
	token := r.provider.latest("personal")

	release := lockRow(t, "personal")
	token.Rotate([]byte("rotated-in-one"))
	first := ended(t, one)
	time.Sleep(300 * time.Millisecond)
	token.Rotate([]byte("rotated-in-two"))
	second := ended(t, two)
	time.Sleep(300 * time.Millisecond)
	release()
	for _, done := range []<-chan error{first, second} {
		if err := <-done; err != nil {
			t.Fatalf("an end returned %v", err)
		}
	}
	if got := opens(t, conn, keys, "personal"); got != "rotated-in-two" {
		t.Errorf("after two ends whose writes interleaved the stored credential opens as %q, want the later rotation landed", got)
	}
	if got := string(r.provider.tokenOf(t, r, "personal")); got != "rotated-in-two" || r.provider.latest("personal") != token {
		t.Errorf("after the two ends the account's token holds %q and was made again: %v, want the token kept", got, r.provider.latest("personal") != token)
	}
}

// VERIFICATIONS' row for a held token, for a credential stored again under new bytes. A reload
// that reads the credential the account's token holds now, sealed again, as a re-seal or another
// process's write-back of the same rotation leaves it, keeps the token, as a token is kept while its
// own current credential equals the stored one (ADR-0089, ADR-0090).
func TestTheSameCredentialStoredUnderNewBytesKeepsTheToken(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	c := seal.AccountCredential("personal")
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", c))
	r := start(t, keys)
	end(t, r.begin(t, "personal"))
	first := r.provider.latest("personal")

	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, "first-token", c))
	r.reload(t)
	end(t, r.begin(t, "personal"))
	if r.provider.latest("personal") != first {
		t.Error("a reload that read the same credential under new bytes made the token again")
	}

	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, keys, "other-token", c))
	r.reload(t)
	if got := string(r.provider.tokenOf(t, r, "personal")); got != "other-token" || r.provider.latest("personal") == first {
		t.Errorf("after another credential was stored the token holds %q, want it made again from the stored one", got)
	}
}

// VERIFICATIONS' row for a stale write refused by the compare-and-set, for a reload during a
// write-back. A reload asked for while a unit's write-back waits at the database, the account's row
// held, waits for the write-back, so it reads the bytes the write-back landed as the process's own,
// not as someone else's, and a second unit's write-back, queued behind the first, still lands. The
// reload, the write-backs and the re-reads take turns under the connections' lock (ADR-0089).
func TestAReloadDuringAWriteBackWaitsForIt(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	keys := generate(t)
	account(t, conn, "personal", "fastmail", sealed(t, keys, "first-token", seal.AccountCredential("personal")))
	r := start(t, keys)
	one, two := r.begin(t, "personal"), r.begin(t, "personal")
	token := r.provider.latest("personal")

	release := lockRow(t, "personal")
	token.Rotate([]byte("rotated-in-one"))
	first := ended(t, one)
	time.Sleep(300 * time.Millisecond)
	reloaded := make(chan error, 1)
	go func() { reloaded <- r.session.Reload(t.Context()) }()
	time.Sleep(300 * time.Millisecond)
	select {
	case err := <-reloaded:
		t.Errorf("a reload finished while a write-back waited at the database, with %v", err)
	default:
	}
	token.Rotate([]byte("rotated-in-two"))
	second := ended(t, two)
	time.Sleep(300 * time.Millisecond)
	release()
	for _, done := range []<-chan error{first, reloaded, second} {
		if err := <-done; err != nil {
			t.Fatalf("an end or the reload returned %v", err)
		}
	}
	if got := opens(t, conn, keys, "personal"); got != "rotated-in-two" {
		t.Errorf("after a reload during two write-backs the stored credential opens as %q, want the later rotation landed", got)
	}
	if got := string(r.provider.tokenOf(t, r, "personal")); got != "rotated-in-two" || r.provider.latest("personal") != token {
		t.Errorf("after the reload the account's token holds %q and was made again: %v, want the token kept", got, r.provider.latest("personal") != token)
	}
}
