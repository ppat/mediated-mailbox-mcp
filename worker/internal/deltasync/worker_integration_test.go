//go:build integration

package deltasync

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// Delta sync's reload re-seals an OAuth client's secret stored under an old key, and sets its scan
// series, in an installation that lists no account, so key replacement can finish before any account
// is connected (ADR-0092, ADR-0103).
func TestAReloadWithNoAccountReSealsAClientSecretOnAnOldKey(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	oldPrivate, oldPublic := keyPair(t)
	newPrivate, newPublic := keyPair(t)
	ring, err := open.NewKeyring(newPublic, oldPrivate, newPrivate)
	if err != nil {
		t.Fatal(err)
	}
	client(t, conn, oldPublic)
	reg := prometheus.NewRegistry()
	j := newJobs()
	d := kind(t, syncPool(t), ring, reg, slog.New(slog.DiscardHandler), fakes(gmailSource(http.DefaultClient), nil), j)

	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}

	current := keyIdentifier(t, sealed(t, newPublic, "x", seal.ClientSecret("x")))
	if got := keyIdentifier(t, storedSecret(t, conn)); got != current {
		t.Errorf("with no account listed the client's secret is sealed to %s, want the current key %s", got, current)
	}
	want := map[string]float64{"mediated_mailbox_client_secret_on_old_key household": 0}
	if diff := cmp.Diff(want, keyScan(t, reg), compare.Options); diff != "" {
		t.Errorf("the scan series with no account listed (-want +got):\n%s", diff)
	}
	if len(j.ensured) != 0 {
		t.Errorf("a reload listing no account added the jobs %v", j.ensured)
	}
}

// A reload whose policy load fails adds no job, since an account whose rules were never read would be
// decided as if every sender were restricted, and still drops the job of an account it no longer
// serves. Once the policy loads again, the account waiting for it gets its job (ADR-0119, ADR-0041).
func TestNoAccountIsAddedWhileThePolicyReloadFails(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler), fakes(gmailSource(http.DefaultClient), nil), j)
	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}

	revoke(t, conn, "SELECT ON policy_rules")
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "UPDATE accounts SET oauth_client = 'household' WHERE account_id = 'work'")
	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	if err := d.reload(t.Context(), schedule.Ask{}); err == nil {
		t.Error("a reload whose policy load failed reported no error")
	}
	if diff := cmp.Diff([]string{"personal"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the jobs added while the policy load fails (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal"}, j.removed, compare.Options); diff != "" {
		t.Errorf("the jobs dropped while the policy load fails (-want +got):\n%s", diff)
	}

	must(t, conn, "GRANT SELECT ON policy_rules TO "+role)
	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"personal", "work"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the jobs added once the policy loads (-want +got):\n%s", diff)
	}
}

// still is a source holding the credential it was built from.
type still struct{ credential []byte }

func (s still) Credential() []byte            { return s.credential }
func (s still) LastAttempt() mail.AuthAttempt { return mail.AuthAttempt{} }

// waiting is a Provider Port whose cursor reads wait until their context ends, as a call the provider
// is slow to answer does. It reports each read's start on started and its end on stopped.
type waiting struct {
	mail.Port[context.Context]
	started, stopped chan string
	account          string
}

func (w waiting) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	w.started <- w.account
	<-ctx.Done()
	w.stopped <- w.account
	return "", ctx.Err()
}

// VERIFICATIONS' row for an account removed while its runs are running, delta sync's part. The
// worker's scheduler runs the kind's reload, which adds the account's job, whose first tick waits on
// the provider. Once the account is no longer served, the next reload drops the job, whose tick's
// context is cancelled, so the tick stops at once, recording nothing more, and no tick starts for the
// account after it (ADR-0119, ADR-0090).
func TestADroppedAccountsTickIsCancelled(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	f, err := fake.New(fake.Config{Account: "personal"})
	if err != nil {
		t.Fatal(err)
	}
	started, stopped := make(chan string, 4), make(chan string, 4)
	connect := session.Connect(func(c session.Credentials) still { return still{credential: c.Credential} },
		func(account string, _ still) (mail.Port[context.Context], error) {
			return waiting{Port: f, started: started, stopped: stopped, account: account}, nil
		})
	metrics, err := schedule.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	s := schedule.New(t.Context(), metrics)
	defer s.Stop()
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	d, err := build(Config{
		Pool: syncPool(t), Keys: ring, Scanner: scanner, Registry: prometheus.NewRegistry(), Logger: slog.New(slog.DiscardHandler),
		Jobs: s, Concurrency: 1, ReloadInterval: 50 * time.Millisecond, SyncInterval: time.Hour, FirstWindow: time.Hour, DecisionsPerTick: 10,
	}, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure(d.Reload()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("no tick started for the account")
	}

	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	select {
	case <-stopped:
	case <-time.After(20 * time.Second):
		t.Fatal("the tick of the account no longer served kept running")
	}
	time.Sleep(300 * time.Millisecond)
	runs := ticked(t, conn)
	if !slices.Equal(runs, []string{"personal"}) {
		t.Fatalf("the accounts with a tick recorded are %v, want the one whose tick was cancelled", runs)
	}
	var state string
	if err := conn.QueryRow(t.Context(), "SELECT state FROM job_runs WHERE account_id = 'personal' AND workload = 'sync'").Scan(&state); err != nil {
		t.Fatalf("reading the tick, the only one the account has: %v", err)
	}
	if state != "running" {
		t.Errorf("the cancelled tick is recorded %q, want running, as a stopped process leaves it for the next tick to record", state)
	}
	select {
	case account := <-started:
		t.Errorf("a tick started for %s after its job was dropped", account)
	default:
	}
}

// Every series delta sync's job kind emits, its own and the shared libraries' it registers, the
// provider adapter's, the rate limiter's and the policy loader's among them, carries the job kind,
// because the kind registers each on the registerer it is given, which labels them (ADR-0117,
// ADR-0076).
func TestEverySeriesTheKindEmitsCarriesItsJobKind(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	registry := prometheus.NewRegistry()
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	j := newJobs()
	d, err := New(Config{
		Pool: syncPool(t), Keys: ring, Scanner: scanner, Client: unreachable(t), Logger: slog.New(slog.DiscardHandler), Jobs: j,
		Registry:    prometheus.WrapRegistererWith(prometheus.Labels{"job_kind": Kind}, registry),
		Concurrency: 1, ReloadInterval: time.Minute, SyncInterval: time.Hour, FirstWindow: time.Hour, DecisionsPerTick: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cycle(t, d, j); err == nil {
		t.Fatal("a tick whose token refresh nothing answers succeeded")
	}
	port, err := d.port("personal", accessToken("an-access-token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := port.CurrentCursor(t.Context()); err == nil {
		t.Fatal("a request through a proxy nothing listens on succeeded")
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var emitted []string
	for _, f := range families {
		emitted = append(emitted, f.GetName())
		for _, m := range f.GetMetric() {
			kind := ""
			for _, l := range m.GetLabel() {
				if l.GetName() == "job_kind" {
					kind = l.GetValue()
				}
			}
			if kind != Kind {
				t.Errorf("%s %v carries the job kind %q, want %q", f.GetName(), m.GetLabel(), kind, Kind)
			}
		}
	}
	for _, name := range []string{
		"mediated_mailbox_provider_request_cost_total", "mediated_mailbox_provider_hard_cap",
		"mediated_mailbox_ratelimit_granted_total", "mediated_mailbox_policyload_reload_failed",
		"mediated_mailbox_credential_on_old_key", "mediated_mailbox_client_secret_on_old_key",
		"mediated_mailbox_sync_cursor_gaps_total", "mediated_mailbox_unclassified_senders_total",
	} {
		if !slices.Contains(emitted, name) {
			t.Errorf("the kind emits no %s, so the test cannot tell its job kind", name)
		}
	}
}

// accessToken is a token source holding an access token, so the adapter builds and counts a request
// without asking for a token.
type accessToken string

func (a accessToken) AccessToken(context.Context) (string, error) { return string(a), nil }

// panicking is a Provider Port whose cursor read panics, as a bug in the code a tick runs would.
type panicking struct{ mail.Port[context.Context] }

func (panicking) CurrentCursor(context.Context) (mail.Cursor, error) { panic("a bug in the tick") }

// VERIFICATIONS' row for a job's run that panics, delta sync's recorded part. A panic raised inside a
// tick is recovered in the run that raised it and recorded as the tick's failure with its stack, and
// the run returns it as its error (ADR-0119, ADR-0117).
func TestAPanicInATickIsRecordedAsTheTicksFailure(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	connect := session.Connect(func(c session.Credentials) still { return still{credential: c.Credential} },
		func(account string, _ still) (mail.Port[context.Context], error) {
			f, err := fake.New(fake.Config{Account: account})
			return panicking{Port: f}, err
		})
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler), connect, j)

	err := cycle(t, d, j)
	if !errors.Is(err, schedule.ErrPanicked) {
		t.Fatalf("the tick returned %v, want the panic as its error", err)
	}
	var state, last string
	if err := conn.QueryRow(t.Context(), "SELECT state, coalesce(last_error, '') FROM job_runs WHERE account_id = 'personal' AND pass = 'tick'").Scan(&state, &last); err != nil {
		t.Fatal(err)
	}
	if state != "failed" || !strings.Contains(last, "a bug in the tick") || !strings.Contains(last, "goroutine") {
		t.Errorf("the tick is recorded %q with %q, want failed with the panic and its stack", state, last)
	}
}

// D4's part, in the worker, of VERIFICATIONS' row for failing the write-back of a rotated credential.
// A tick whose source rotated the account's refresh token hands it over when the tick ends, and a
// write-back that fails is logged by the loader and fails the tick with an error naming the account,
// so the failure is loud (ADR-0082).
func TestAFailedWriteBackFailsTheTick(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	rotated := func(c session.Credentials) *gmail.TokenSource {
		if string(c.Credential) == "first-token" {
			c.Credential = []byte("second-token")
		}
		return gmailSource(http.DefaultClient)(c)
	}
	var log logBuffer
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), log.logger(), fakes(rotated, map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)
	revoke(t, conn, "UPDATE (credential) ON account_state")

	err := cycle(t, d, j)

	if err == nil || !strings.Contains(err.Error(), "account personal: writing back the rotated credential") {
		t.Fatalf("the tick returned %v, want the failed write-back", err)
	}
	var failures []record
	for _, r := range log.records(t) {
		if r.Level == "ERROR" && strings.Contains(r.Msg, "rotated credential") {
			failures = append(failures, r)
		}
	}
	want := []record{{Level: "ERROR", Msg: "a rotated credential was not written back, so a restart before a later write-back lands loses the account's access", Account: "personal"}}
	if diff := cmp.Diff(want, failures, compare.Options); diff != "" {
		t.Errorf("error records (-want +got):\n%s", diff)
	}
}

// ticks returns how many ticks the account has recorded.
func ticks(t *testing.T, conn *pgx.Conn, account string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM job_runs WHERE account_id = $1 AND pass = 'tick'", account).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A delta sync job asked before its sync interval has passed since its last tick answers that it is
// not due and ticks nothing, so a wake that comes early, such as the end of a backoff, never ticks
// twice inside one interval, and the job asked once the interval has passed ticks again (ADR-0103,
// ADR-0119).
func TestAJobAskedBeforeItsIntervalDoesNotTick(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler),
		fakes(gmailSource(http.DefaultClient), map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)
	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	job := j.held[schedule.Key{Kind: Kind, ID: "personal"}]
	t0 := time.UnixMilli(1_790_000_000_000)

	if err := job.Run(t.Context(), schedule.Ask{At: t0, Phase: t0}); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(t.Context(), schedule.Ask{At: t0.Add(time.Minute), Phase: t0}); !errors.Is(err, schedule.ErrNotDue) {
		t.Errorf("the job asked a minute after its tick returned %v, want that it is not due", err)
	}
	if n := ticks(t, conn, "personal"); n != 1 {
		t.Errorf("the account has %d ticks after an early ask, want 1", n)
	}
	if err := job.Run(t.Context(), schedule.Ask{At: t0.Add(5 * time.Minute), Phase: t0.Add(5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if n := ticks(t, conn, "personal"); n != 2 {
		t.Errorf("the account has %d ticks once the interval passed, want 2", n)
	}
}

// A tick asked off the phase, at the end of a backoff, is recorded at the ticker's tick before it, so
// the ticker's next tick is due and the ticks keep the phase the scheduler asks them on. Recorded at
// its own time, the ticker's next tick would come less than an interval after it and read not due,
// and the account would wait one interval more (ADR-0119, ADR-0103).
func TestATickAtTheEndOfABackoffKeepsThePhase(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler),
		fakes(gmailSource(http.DefaultClient), map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)
	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	job := j.held[schedule.Key{Kind: Kind, ID: "personal"}]
	t0 := time.UnixMilli(1_790_000_000_000)

	if err := job.Run(t.Context(), schedule.Ask{At: t0, Phase: t0}); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(t.Context(), schedule.Ask{At: t0.Add(11 * time.Minute), Phase: t0.Add(10 * time.Minute)}); err != nil {
		t.Fatalf("the ask at the end of a backoff returned %v, want a tick", err)
	}
	if err := job.Run(t.Context(), schedule.Ask{At: t0.Add(15 * time.Minute), Phase: t0.Add(15 * time.Minute)}); err != nil {
		t.Errorf("the ask at the ticker's next tick returned %v, want a tick", err)
	}
	if n := ticks(t, conn, "personal"); n != 3 {
		t.Errorf("the account has %d ticks, want 3, one at the backoff's end and one at the ticker's next tick", n)
	}
}

// Each tick takes the account snapshot the kind's latest reload holds, so a credential the operator
// stored for an account already served reaches the account's next tick after the reload that reads
// it, not the snapshot of the reload that added the account's job (ADR-0090, ADR-0119).
func TestATickTakesTheActiveAccountSnapshot(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")
	client(t, conn, public)
	var built []string
	source := func(c session.Credentials) *gmail.TokenSource {
		built = append(built, string(c.Credential))
		return gmailSource(http.DefaultClient)(c)
	}
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler),
		fakes(source, map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)
	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, public, "second-token", seal.AccountCredential("personal")))
	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"first-token", "second-token"}, built, compare.Options); diff != "" {
		t.Errorf("the credentials each tick's source was built from (-want +got):\n%s", diff)
	}
}
