//go:build integration

package backfill

import (
	"context"
	"errors"
	"log/slog"
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
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// still is a source holding the credential it was built from.
type still struct{ credential []byte }

func (s still) Credential() []byte            { return s.credential }
func (s still) LastAttempt() mail.AuthAttempt { return mail.AuthAttempt{} }

// held returns a connector whose ports are provider fakes holding messages.
func held(messages ...fake.Message) session.Connector {
	return session.Connect(func(c session.Credentials) still { return still{credential: c.Credential} },
		func(account string, _ still) (mail.Port[context.Context], error) {
			return fake.New(fake.Config{Account: account, PageSize: 2}, messages...)
		})
}

// shopMail returns n messages from a shop, each with a body.
func shopMail(n int) []fake.Message {
	var out []fake.Message
	for i := range n {
		id := "m" + string(rune('a'+i))
		out = append(out, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: id, ThreadID: "t" + id, From: mail.Address{Email: "orders@shop.example"}, Subject: marker.Field("order"),
				Date: mail.UnixMilli(1_700_000_000_000 + int64(i)*60_000),
			},
			Body: mail.MessageBody{Text: marker.Body("order") + " Thanks for your order."},
		})
	}
	return out
}

// worker returns backfill's job kind with jobs added to a scheduler of its own, reloading every few
// milliseconds, and the scheduler, which the test stops when it ends.
func worker(t *testing.T, ring *open.Keyring, registry prometheus.Registerer, connect session.Connector) (*Backfill, *schedule.Scheduler) {
	t.Helper()
	metrics, err := schedule.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	s := schedule.New(t.Context(), metrics)
	t.Cleanup(s.Stop)
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	b, err := build(Config{
		Pool: backfillPool(t), Keys: ring, Scanner: scanner, Registry: registry, Logger: slog.New(slog.DiscardHandler),
		Jobs: s, Concurrency: 1, ReloadInterval: 50 * time.Millisecond,
	}, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		t.Fatal(err)
	}
	return b, s
}

// eventually waits until done reports true, failing the test after twenty seconds.
func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen within twenty seconds", what)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// ended reports whether both of the account's passes have ended.
func ended(t *testing.T, conn *pgx.Conn, account string) bool {
	t.Helper()
	var first, second bool
	err := conn.QueryRow(t.Context(), "SELECT backfill_pass1_complete, backfill_pass2_complete FROM account_state WHERE account_id = $1", account).Scan(&first, &second)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return first && second
}

// revisions returns the scanner revision each of the account's messages' verdicts names, by message.
func revisions(t *testing.T, conn *pgx.Conn, account string) map[string]string {
	t.Helper()
	rows, err := conn.Query(t.Context(), "SELECT message_id, coalesce(scanner_revision, '') FROM messages WHERE account_id = $1", account)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var id, revision string
	if _, err := pgx.ForEachRow(rows, []any{&id, &revision}, func() error {
		out[id] = revision
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// VERIFICATIONS' row for a newly connected account. With the worker's scheduler running backfill's
// reload and no account connected, an account connected in the database is backfilled within one
// reload, both passes, with no request, signal or restart. The run-start step is made once for each
// job's life, so the account no longer served and then connected again, after its verdicts were made
// under another scanner, has them returned to pending and scanned again under the worker's scanner
// (ADR-0119, ADR-0121, ADR-0120).
func TestANewAccountIsBackfilledWithNoManualStep(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	client(t, conn, public)
	b, s := worker(t, ring, prometheus.NewRegistry(), held(shopMail(3)...))
	if err := s.Ensure(b.Reload()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	must(t, conn, "UPDATE accounts SET oauth_client = 'household' WHERE account_id = 'personal'")
	eventually(t, "the new account's backfill", func() bool { return ended(t, conn, "personal") })
	want := map[string]string{"ma": "a-revision", "mb": "a-revision", "mc": "a-revision"}
	if diff := cmp.Diff(want, revisions(t, conn, "personal"), compare.Options); diff != "" {
		t.Fatalf("the messages the new account's backfill indexed and scanned (-want +got):\n%s", diff)
	}

	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	time.Sleep(300 * time.Millisecond)
	must(t, conn, "UPDATE messages SET scanner_revision = 'another-revision' WHERE account_id = 'personal'")
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, public, "personal-token", seal.AccountCredential("personal")))
	eventually(t, "the run-start step of the account served again", func() bool {
		return ended(t, conn, "personal") && revisions(t, conn, "personal")["ma"] == "a-revision"
	})
	if diff := cmp.Diff(want, revisions(t, conn, "personal"), compare.Options); diff != "" {
		t.Errorf("the messages once the account is served again (-want +got):\n%s", diff)
	}
}

// waiting is a Provider Port whose enumeration waits until its context ends, as a call the provider is
// slow to answer does. It reports each call's start on started and its end on stopped.
type waiting struct {
	mail.Port[context.Context]
	started, stopped chan string
	account          string
}

func (w waiting) EnumerateAll(ctx context.Context, _ mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	w.started <- w.account
	<-ctx.Done()
	w.stopped <- w.account
	return mail.Page[mail.MessageMetadata]{}, ctx.Err()
}

// VERIFICATIONS' row for an account removed while its runs are running, backfill's part. The account's
// first pass waits on the provider. Once the account is no longer served, the next reload drops its
// job, whose run's context is cancelled, so the run stops at once with its checkpoint as it was,
// recording nothing more, and no run starts for the account after it (ADR-0119, ADR-0090).
func TestADroppedAccountsRunIsCancelled(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
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
	b, s := worker(t, ring, prometheus.NewRegistry(), connect)
	if err := s.Ensure(b.Reload()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("no run started for the account")
	}

	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	select {
	case <-stopped:
	case <-time.After(20 * time.Second):
		t.Fatal("the run of the account no longer served kept running")
	}
	time.Sleep(300 * time.Millisecond)
	var states []string
	rows, err := conn.Query(t.Context(), "SELECT state FROM job_runs WHERE account_id = 'personal' AND workload = 'backfill'")
	if err != nil {
		t.Fatal(err)
	}
	var state string
	if _, err := pgx.ForEachRow(rows, []any{&state}, func() error {
		states = append(states, state)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"running"}, states, compare.Options); diff != "" {
		t.Errorf("the account's runs, the cancelled one left running for the next run to record (-want +got):\n%s", diff)
	}
	select {
	case account := <-started:
		t.Errorf("a run started for %s after its job was dropped", account)
	default:
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
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), quiet, j)
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}

	revoke(t, conn, "SELECT ON policy_rules")
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "UPDATE accounts SET oauth_client = 'household' WHERE account_id = 'work'")
	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	if err := b.reload(t.Context(), schedule.Ask{}); err == nil {
		t.Error("a reload whose policy load failed reported no error")
	}
	if diff := cmp.Diff([]string{"personal"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the jobs added while the policy load fails (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal"}, j.removed, compare.Options); diff != "" {
		t.Errorf("the jobs dropped while the policy load fails (-want +got):\n%s", diff)
	}

	must(t, conn, "GRANT SELECT ON policy_rules TO "+role)
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"personal", "work"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the jobs added once the policy loads (-want +got):\n%s", diff)
	}
}

// panicking is a Provider Port whose enumeration panics, as a bug in the code a page runs would.
type panicking struct{ mail.Port[context.Context] }

func (panicking) EnumerateAll(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	panic("a bug in the page")
}

// VERIFICATIONS' row for a job's run that panics, its recorded part. A panic raised inside a page of
// the first pass is recovered in the run that raised it and recorded as that run's failure with its
// stack, so the run's record says what happened, and the run returns it as its error (ADR-0119,
// ADR-0117).
func TestAPanicInARunIsRecordedAsTheRunsFailure(t *testing.T) {
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
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), connect, j)

	err := runAll(t, b, j)
	if !errors.Is(err, schedule.ErrPanicked) {
		t.Fatalf("the run returned %v, want the panic as its error", err)
	}
	var state, last string
	if err := conn.QueryRow(t.Context(), "SELECT state, coalesce(last_error, '') FROM job_runs WHERE account_id = 'personal' AND pass = 'pass1'").Scan(&state, &last); err != nil {
		t.Fatal(err)
	}
	if state != "failed" || !strings.Contains(last, "a bug in the page") || !strings.Contains(last, "goroutine") {
		t.Errorf("the run is recorded %q with %q, want failed with the panic and its stack", state, last)
	}
}

// Every series backfill's job kind emits, its own and the shared libraries' it registers, the provider
// adapter's, the rate limiter's and the policy loader's among them, carries the job kind, because the
// kind registers each on the registerer it is given, which labels them (ADR-0117, ADR-0076).
func TestEverySeriesTheKindEmitsCarriesItsJobKind(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	registry := prometheus.NewRegistry()
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	j := newJobs()
	b, err := New(Config{
		Pool: backfillPool(t), Keys: ring, Scanner: scanner, Client: unreachable(), Logger: slog.New(slog.DiscardHandler), Jobs: j,
		Registry:    prometheus.WrapRegistererWith(prometheus.Labels{"job_kind": Kind}, registry),
		Concurrency: 1, ReloadInterval: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runAll(t, b, j); err == nil {
		t.Fatal("a run whose token refresh nothing answers succeeded")
	}
	port, err := b.port("personal", accessToken("an-access-token"))
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
		"mediated_mailbox_unclassified_senders_total",
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

// editing is a provider fake whose enumeration runs edit before it answers the page after the first,
// as an operator's policy edit landing while a run is between its pages.
type editing struct {
	*fake.Fake
	calls int
	edit  func()
}

func (e *editing) EnumerateAll(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	e.calls++
	if e.calls == 2 {
		e.edit()
	}
	return e.Fake.EnumerateAll(ctx, token)
}

// VERIFICATIONS' row for a policy edit during a running pass, the part a backfill run's pages take.
// Each page of a run takes the policy the kind's loader holds at its entry, so a rule restricting the
// shop, added and reloaded while the first pass is between its pages, classifies the shop's message on
// the next page as restricted, and the second pass's comparison restricts the one stored before it, in
// the same run (ADR-0119, ADR-0041, ADR-0113).
func TestEachPageTakesTheActivePolicy(t *testing.T) {
	want := map[string]string{"m1": "restricted", "m2": "normal", "m3": "restricted", "m4": "normal"}
	if diff := cmp.Diff(want, classesAfterAnEdit(t, false), compare.Options); diff != "" {
		t.Errorf("the classes the run stored under the rule added between its pages (-want +got):\n%s", diff)
	}
}

// The first pass's part of the same row. With the second pass found ended, so it makes no comparison,
// the stored classes are the first pass's own, and the shop's message on the page after the edit is
// stored restricted while the one stored before it stays normal (ADR-0119, ADR-0041).
func TestEachPageOfTheFirstPassTakesTheActivePolicy(t *testing.T) {
	want := map[string]string{"m1": "normal", "m2": "normal", "m3": "restricted", "m4": "normal"}
	if diff := cmp.Diff(want, classesAfterAnEdit(t, true), compare.Options); diff != "" {
		t.Errorf("the classes the first pass stored under the rule added between its pages (-want +got):\n%s", diff)
	}
}

// classesAfterAnEdit runs backfill over four messages on two pages, with a rule restricting the shop
// added and reloaded before the second page, and returns the classes stored. secondEnded records the
// second pass as ended before the run, so it opens done.
func classesAfterAnEdit(t *testing.T, secondEnded bool) map[string]string {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	messages := []fake.Message{
		{Metadata: mail.MessageMetadata{ID: "m1", ThreadID: "t1", From: mail.Address{Email: "a@shop.example"}, Subject: marker.Field("first"), Date: 1_700_000_000_000}},
		{Metadata: mail.MessageMetadata{ID: "m2", ThreadID: "t2", From: mail.Address{Email: "b@news.example"}, Subject: marker.Field("second"), Date: 1_700_000_060_000}},
		{Metadata: mail.MessageMetadata{ID: "m3", ThreadID: "t3", From: mail.Address{Email: "c@shop.example"}, Subject: marker.Field("third"), Date: 1_700_000_120_000}},
		{Metadata: mail.MessageMetadata{ID: "m4", ThreadID: "t4", From: mail.Address{Email: "d@news.example"}, Subject: marker.Field("fourth"), Date: 1_700_000_180_000}},
	}
	f, err := fake.New(fake.Config{Account: "personal", PageSize: 2}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	url := postgres.URL(t)
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), url)
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := c.Exec(context.Background(), "DELETE FROM policy_rules"); err != nil {
			t.Error(err)
		}
		if err := c.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if secondEnded {
		must(t, conn, "UPDATE account_state SET backfill_pass2_complete = true WHERE account_id = 'personal'")
	}
	j := newJobs()
	var b *Backfill
	port := &editing{Fake: f, edit: func() {
		must(t, conn, "INSERT INTO policy_rules (rule_id, class, domain_suffix, source, created_by) VALUES ('rule.shop', 'restricted', '{shop.example}', 'operator', 'operator')")
		if err := b.policies.Reload(t.Context()); err != nil {
			t.Fatal(err)
		}
	}}
	connect := session.Connect(func(c session.Credentials) still { return still{credential: c.Credential} },
		func(string, still) (mail.Port[context.Context], error) { return port, nil })
	b = kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), connect, j)

	if err := runAll(t, b, j); err != nil {
		t.Fatal(err)
	}

	rows, err := conn.Query(t.Context(), "SELECT message_id, sender_class FROM messages WHERE account_id = 'personal'")
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	var id, class string
	if _, err := pgx.ForEachRow(rows, []any{&id, &class}, func() error {
		classes[id] = class
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return classes
}

// bodyPanicking is a provider fake whose body read panics, as a bug in the code a page of the second
// pass runs would.
type bodyPanicking struct{ *fake.Fake }

func (bodyPanicking) GetMessageBody(context.Context, string) (mail.MessageBody, error) {
	panic("a bug in the second pass")
}

// VERIFICATIONS' row for a job's run that panics, the second pass's recorded part. A panic raised
// inside a page of the second pass is recovered in the run that raised it and recorded as the second
// pass run's failure with its stack, after the first pass ended (ADR-0119, ADR-0117).
func TestAPanicInTheSecondPassIsRecordedAsItsRunsFailure(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	connect := session.Connect(func(c session.Credentials) still { return still{credential: c.Credential} },
		func(account string, _ still) (mail.Port[context.Context], error) {
			f, err := fake.New(fake.Config{Account: account, PageSize: 2}, shopMail(1)...)
			return bodyPanicking{Fake: f}, err
		})
	j := newJobs()
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), connect, j)

	err := runAll(t, b, j)
	if !errors.Is(err, schedule.ErrPanicked) {
		t.Fatalf("the run returned %v, want the panic as its error", err)
	}
	var state, last string
	if err := conn.QueryRow(t.Context(), "SELECT state, coalesce(last_error, '') FROM job_runs WHERE account_id = 'personal' AND pass = 'pass2'").Scan(&state, &last); err != nil {
		t.Fatal(err)
	}
	if state != "failed" || !strings.Contains(last, "a bug in the second pass") || !strings.Contains(last, "goroutine") {
		t.Errorf("the second pass run is recorded %q with %q, want failed with the panic and its stack", state, last)
	}
}

// The run-start step is made once for each job's life, and not again at the job's later asks. Once an
// account is backfilled, its verdicts made under another scanner stay as they are through the job's
// next ask, the reconciliation ask every job gets on its interval, since the step that returns them to
// pending was made at the job's first ask (ADR-0121).
func TestTheRunStartStepIsNotMadeAgainAtAJobsLaterAsks(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	j := newJobs()
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), held(shopMail(3)...), j)
	if err := runAll(t, b, j); err != nil {
		t.Fatal(err)
	}

	must(t, conn, "UPDATE messages SET scanner_revision = 'another-revision' WHERE account_id = 'personal'")
	if err := runAll(t, b, j); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"ma": "another-revision", "mb": "another-revision", "mc": "another-revision"}
	if diff := cmp.Diff(want, revisions(t, conn, "personal"), compare.Options); diff != "" {
		t.Errorf("the verdicts after the job's later ask, which makes no run-start step (-want +got):\n%s", diff)
	}
}

// A run-start step that fails is made again at the job's next ask, since the step counts as made only
// once it has succeeded. A process whose step for the account fails once, its role briefly lacking a
// grant the step needs, returns the verdicts made under another scanner to pending at the next ask,
// and they are scanned again under its scanner (ADR-0121).
func TestAFailedRunStartStepIsMadeAgainAtTheNextAsk(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	earlier := newJobs()
	if err := runAll(t, kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), held(shopMail(3)...), earlier), earlier); err != nil {
		t.Fatal(err)
	}
	must(t, conn, "UPDATE messages SET scanner_revision = 'another-revision' WHERE account_id = 'personal'")

	j := newJobs()
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), held(shopMail(3)...), j)
	revoke(t, conn, "UPDATE (scanned_at) ON messages")
	if err := runAll(t, b, j); err == nil {
		t.Fatal("the run whose run-start step lacked a grant reported no error")
	}
	must(t, conn, "GRANT UPDATE (scanned_at) ON messages TO "+role)
	if err := runAll(t, b, j); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"ma": "a-revision", "mb": "a-revision", "mc": "a-revision"}
	if diff := cmp.Diff(want, revisions(t, conn, "personal"), compare.Options); diff != "" {
		t.Errorf("the verdicts after the next ask makes the failed step again (-want +got):\n%s", diff)
	}
}

// accountsIn returns the accounts the series name carries on reg, by the account label.
func accountsIn(t *testing.T, reg *prometheus.Registry, name string) []string {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" {
					out = append(out, l.GetValue())
				}
			}
		}
	}
	return out
}

// An account's scan backlog series is removed once its second pass ends, when delta sync's tick
// emits the account's backlog in its place, and its unclassified series once its job is dropped, so
// the worker, which outlives both, serves no series for an account it no longer backfills (ADR-0104,
// ADR-0076, ADR-0119).
func TestTheKindsSeriesOfAnAccountGoWhenItsWorkEnds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	reg := prometheus.NewRegistry()
	j := newJobs()
	b, err := build(Config{
		Pool: backfillPool(t), Keys: ring, Scanner: scanner, Registry: reg, Logger: slog.New(slog.DiscardHandler), Jobs: j,
		Concurrency: 1, ReloadInterval: time.Minute,
	}, map[string]session.Connector{gmailProvider: held(shopMail(3)...)})
	if err != nil {
		t.Fatal(err)
	}
	if err := runAll(t, b, j); err != nil {
		t.Fatal(err)
	}
	if !ended(t, conn, "personal") {
		t.Fatal("the account's backfill did not end")
	}
	if got := accountsIn(t, reg, "mediated_mailbox_scan_backlog"); len(got) != 0 {
		t.Errorf("the backlog series names %v once the second pass ended, want none", got)
	}
	if got := accountsIn(t, reg, "mediated_mailbox_unclassified_senders_total"); !slices.Equal(got, []string{"personal"}) {
		t.Errorf("the unclassified series names %v while the account is served, want personal", got)
	}

	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	if got := accountsIn(t, reg, "mediated_mailbox_unclassified_senders_total"); len(got) != 0 {
		t.Errorf("the unclassified series names %v once the account's job was dropped, want none", got)
	}
}

// The backlog series of an account whose job is dropped before its second pass ends is removed with
// the job (ADR-0076, ADR-0119).
func TestTheBacklogSeriesOfADroppedAccountGoesWithItsJob(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	reg := prometheus.NewRegistry()
	j := newJobs()
	connect := session.Connect(func(c session.Credentials) still { return still{credential: c.Credential} },
		func(account string, _ still) (mail.Port[context.Context], error) {
			f, err := fake.New(fake.Config{Account: account, PageSize: 2}, shopMail(3)...)
			return bodyPanicking{Fake: f}, err
		})
	b, err := build(Config{
		Pool: backfillPool(t), Keys: ring, Scanner: scanner, Registry: reg, Logger: slog.New(slog.DiscardHandler), Jobs: j,
		Concurrency: 1, ReloadInterval: time.Minute,
	}, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		t.Fatal(err)
	}
	if err := runAll(t, b, j); err == nil {
		t.Fatal("the second pass whose body read panics succeeded")
	}
	if got := accountsIn(t, reg, "mediated_mailbox_scan_backlog"); !slices.Equal(got, []string{"personal"}) {
		t.Fatalf("the backlog series names %v while the second pass has not ended, want personal", got)
	}

	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	if got := accountsIn(t, reg, "mediated_mailbox_scan_backlog"); len(got) != 0 {
		t.Errorf("the backlog series names %v once the account's job was dropped, want none", got)
	}
}
