//go:build integration

package pass1_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// mixed is a mailbox of seven messages in pages of three, from every kind of sender, some with a code
// in the subject.
var mixed = setup{PageSize: 3, Messages: []drawn{
	{Sender: 0, Code: true, Inbox: true},
	{Sender: 1, ListID: true, Inbox: true},
	{Sender: 1, ListID: true},
	{Sender: 2, Code: true},
	{Sender: 3},
	{Sender: 0},
	{Sender: 2, Inbox: true},
}}

// finish drives the world's pass until it is done, failing the test after limit steps.
func (w *world) finish(t *testing.T, limit int) {
	t.Helper()
	for range limit {
		if w.step(t) {
			return
		}
	}
	t.Fatalf("the pass did not end within %d steps", limit)
}

// timeline returns the kinds of events a run recorded, in order, with their pages.
func timeline(t *testing.T, account, runID string) []string {
	t.Helper()
	conn := superuser(t)
	rows, err := conn.Query(t.Context(), `SELECT kind || coalesce(' ' || page::text, '') FROM job_run_events
		WHERE account_id = $1 AND run_id = $2 ORDER BY seq`, account, runID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return events
}

// failures returns the failed items a run recorded, as class, page, attempts and disposition.
func failures(t *testing.T, account string) []string {
	t.Helper()
	conn := superuser(t)
	rows, err := conn.Query(t.Context(), `SELECT run_id || ' ' || item_kind || ' ' || item_id || ' ' || page::text || ' ' || error_class
		|| ' ' || attempts::text || ' ' || disposition FROM job_run_failures WHERE account_id = $1 ORDER BY seq`, account)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A whole pass over PostgreSQL indexes every message once with its subject masked and its sender
// classified, rebuilds each sender's statistics, records each mask as an event naming its rule and
// tier, records the run with a progress event at every checkpoint, and sets the account's completion
// flag when it ends (ADR-0003, ADR-0017, ADR-0022).
func TestAPassIndexesTheWholeMailbox(t *testing.T) {
	w := realWorld(t, mixed)
	w.finish(t, 10)
	w.progress(t)

	conn := superuser(t)
	var rule string
	var tier int
	if err := conn.QueryRow(t.Context(), "SELECT rule_id, tier FROM masking_events WHERE account_id = $1 AND message_id = 'm1'", w.account).Scan(&rule, &tier); err != nil {
		t.Fatal(err)
	}
	if rule != "mfa.trigger_window" || tier != 1 {
		t.Errorf("the masking event names rule %q at tier %d, want mfa.trigger_window at tier 1", rule, tier)
	}
	type stats struct {
		Count     int
		First     string
		Last      string
		ListRatio float32
		Labels    string
	}
	var got stats
	err := conn.QueryRow(t.Context(), `SELECT message_count, first_seen::text, last_seen::text, has_list_id_ratio, label_distribution::text
		FROM senders WHERE account_id = $1 AND domain = 'news.example'`, w.account).Scan(&got.Count, &got.First, &got.Last, &got.ListRatio, &got.Labels)
	if err != nil {
		t.Fatal(err)
	}
	want := stats{Count: 2, First: "2023-11-14 22:14:20+00", Last: "2023-11-14 22:15:20+00", ListRatio: 1, Labels: `{"INBOX": 1}`}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the statistics of news.example (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"start 0", "progress 1", "progress 2", "progress 3", "finish"}, timeline(t, w.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the run's timeline (-want +got):\n%s", diff)
	}
	s := w.inspect(t)
	wantProgress := core.Progress{Checkpoint: core.Checkpoint{Page: 3, Stamp: core.Stamp{Version: 1, Revision: "revision-0"}}, Counters: core.Counters{Pages: 3, Messages: 7}}
	latest, _ := s.latest()
	if diff := cmp.Diff(wantProgress, latest.Progress, compare.Options); diff != "" {
		t.Errorf("the run's last checkpoint and counters (-want +got):\n%s", diff)
	}
}

// A mailbox with no message ends the pass on its first, empty page.
func TestAnEmptyMailboxEndsThePass(t *testing.T) {
	w := realWorld(t, setup{PageSize: 3})
	w.finish(t, 2)
	w.progress(t)
}

// A pass that has ended opens done, records no run and asks the provider for nothing.
func TestAPassThatEndedIsSkipped(t *testing.T) {
	w := realWorld(t, mixed)
	w.finish(t, 10)
	fetched := w.fetched
	w.open(t)
	if w.pass.Run() != "" {
		t.Errorf("a pass that ended opened the run %q", w.pass.Run())
	}
	if !w.step(t) || w.fetched != fetched {
		t.Errorf("a pass that ended was not done at once, or asked the provider for a page")
	}
	if n := len(w.inspect(t).Runs); n != 1 {
		t.Errorf("%d runs are recorded, want the one that ended the pass", n)
	}
}

// A run killed after two pages leaves them durable, and the next run records itself as resuming it,
// records the killed run as failed, and asks the provider next for the third page, so the kill cost no
// page (ADR-0017).
func TestAKilledRunIsResumedByTheNextRun(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	w.step(t)
	w.crash(t, betweenSteps)
	w.open(t)
	w.persistence(t)
	w.finish(t, 10)
	w.progress(t)
	s := w.inspect(t)
	var got []run
	for _, r := range s.Runs {
		r.Progress = core.Progress{}
		got = append(got, r)
	}
	want := []run{{ID: "run1", State: "failed"}, {ID: "run2", State: "succeeded", ResumedFrom: "run1"}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the runs (-want +got):\n%s", diff)
	}
	if w.fetched != 3 {
		t.Errorf("the provider was asked for %d pages, want 3", w.fetched)
	}
	if diff := cmp.Diff([]string{"resume 2", "progress 3", "finish"}, timeline(t, w.account, "run2"), compare.Options); diff != "" {
		t.Errorf("the resuming run's timeline (-want +got):\n%s", diff)
	}
}

// A run killed after the commit of the page that ended the enumeration, before it recorded its end, is
// resumed by a run that finishes the pass and asks the provider for no page again (ADR-0017).
func TestARunKilledBeforeItsEndFinishesWithoutEnumeratingAgain(t *testing.T) {
	w := realWorld(t, setup{PageSize: 1, Messages: []drawn{{Sender: 1}, {Sender: 2}}})
	w.step(t)
	w.crash(t, afterCommit)
	w.open(t)
	w.persistence(t)
	if !w.step(t) {
		t.Fatalf("the resuming run did not end the pass on its first step")
	}
	if w.fetched != 2 {
		t.Errorf("the provider was asked for %d pages, want 2, the mailbox once", w.fetched)
	}
}

// A page the provider throttles is asked for again, and once it arrives the failure is recorded as
// recovered, with how many attempts it took, beside a backoff event for each throttle. The throttles
// reach the rate limiter's controller (ADR-0022, ADR-0024).
func TestAThrottledPageIsRetriedAndRecorded(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	w.throttle = 2
	w.finish(t, 10)
	w.progress(t)
	if diff := cmp.Diff([]string{"run1 page 2 2 throttled 3 recovered"}, failures(t, w.account), compare.Options); diff != "" {
		t.Errorf("the failed items (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"start 0", "progress 1", "backoff 2", "backoff 2", "progress 2", "progress 3", "finish"}, timeline(t, w.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the timeline (-want +got):\n%s", diff)
	}
	var cut float64
	if err := superuser(t).QueryRow(t.Context(), "SELECT current_rate FROM rate_state WHERE account_id = $1", w.account).Scan(&cut); err != nil {
		t.Fatal(err)
	}
	if cut >= 5000 {
		t.Errorf("the rate is %v after two throttles, want it cut below the target of 5000", cut)
	}
}

// A page the provider throttles on every attempt fails the run after the last attempt, recording the
// failure as abandoned and the run as failed with its error. The next run resumes from the page before
// it (ADR-0022).
func TestAPageFailingEveryAttemptFailsTheRun(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	w.throttle = core.MaxAttempts
	if w.step(t) || w.pass != nil {
		t.Fatal("a page throttled on every attempt did not fail the run")
	}
	s := w.inspect(t)
	if latest, _ := s.latest(); latest.State != "failed" {
		t.Errorf("the run is %q, want failed", latest.State)
	}
	var lastError string
	if err := superuser(t).QueryRow(t.Context(), "SELECT last_error FROM job_runs WHERE account_id = $1 AND run_id = 'run1'", w.account).Scan(&lastError); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(lastError, "asking for page 2: ") || !strings.Contains(lastError, "throttled") {
		t.Errorf("the run's last error is %q, want the throttle on page 2", lastError)
	}
	if diff := cmp.Diff([]string{"run1 page 2 2 throttled 5 abandoned"}, failures(t, w.account), compare.Options); diff != "" {
		t.Errorf("the failed items (-want +got):\n%s", diff)
	}
	w.finish(t, 10)
	w.progress(t)
}

// A credential the provider refuses fails the run on its first attempt, since asking again cannot
// change the answer.
func TestARefusedCredentialFailsTheRunAtOnce(t *testing.T) {
	w := realWorld(t, mixed)
	refusing := w.deps
	refusing.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, mail.ErrAuthentication
	}
	p, err := pass1.Open(t.Context(), refusing, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Next(t.Context()); !errors.Is(err, mail.ErrAuthentication) {
		t.Fatalf("Next returned %v, want the refused credential", err)
	}
	if diff := cmp.Diff([]string{"run1 page 1 1 authentication 1 abandoned"}, failures(t, w.account), compare.Options); diff != "" {
		t.Errorf("the failed items (-want +got):\n%s", diff)
	}
}

// The fallback when the provider refuses the page token a run resumed from. The pass starts over from
// the first page and takes every page again, and a page taken twice counts nothing twice. Each message
// is indexed once with one masking event per mask, each sender's statistics count each message once,
// and the counters count each message once (ADR-0017).
func TestAPassStartedOverCountsNothingTwice(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	w.step(t)
	w.crash(t, betweenSteps)
	conn := superuser(t)
	if _, err := conn.Exec(t.Context(), `UPDATE job_runs SET checkpoint = jsonb_set(checkpoint, '{token}', '"mnot-issued"')
		WHERE account_id = $1`, w.account); err != nil {
		t.Fatal(err)
	}
	w.reported.Checkpoint.Token = "mnot-issued"
	w.finish(t, 10)
	// Starting over takes the two pages made durable before the refusal again, beside the one page a
	// crash may cost, so the bound on rework is the crash's plus those two.
	w.crashes += 2
	w.progress(t)
	if diff := cmp.Diff([]string{"resume 2", "retry 3", "progress 1", "progress 2", "progress 3", "finish"}, timeline(t, w.account, "run2"), compare.Options); diff != "" {
		t.Errorf("the timeline of the run that started over (-want +got):\n%s", diff)
	}
	latest, _ := w.inspect(t).latest()
	if diff := cmp.Diff(core.Counters{Pages: 5, Messages: 7}, latest.Progress.Counters, compare.Options); diff != "" {
		t.Errorf("the counters, five pages made durable and seven messages added (-want +got):\n%s", diff)
	}
}

// storedCheckpoint returns the checkpoint the run's record holds, as its keys and values.
func storedCheckpoint(t *testing.T, account, runID string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := superuser(t).QueryRow(t.Context(), "SELECT checkpoint FROM job_runs WHERE account_id = $1 AND run_id = $2",
		account, runID).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// The run record's checkpoint carries the pages the enumeration takes, from the total the provider
// reports with each page, which the backfill card reads as page of pages. Seven messages in pages of
// three take three pages. A provider reporting no total leaves the checkpoint without them, so the
// card shows no estimate (ADR-0095).
func TestTheCheckpointCarriesThePagesTheTotalImplies(t *testing.T) {
	counted := realWorld(t, setup{PageSize: 3, Messages: mixed.Messages, Totals: true})
	counted.step(t)
	if diff := cmp.Diff(map[string]any{"page": 1.0, "token": "m4", "of": 3.0, "version": 1.0, "revision": "revision-0"}, storedCheckpoint(t, counted.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the checkpoint after the first page (-want +got):\n%s", diff)
	}
	counted.finish(t, 10)
	if diff := cmp.Diff(map[string]any{"page": 3.0, "token": "", "of": 3.0, "version": 1.0, "revision": "revision-0"}, storedCheckpoint(t, counted.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the checkpoint after the last page (-want +got):\n%s", diff)
	}
	uncounted := realWorld(t, mixed)
	uncounted.step(t)
	if diff := cmp.Diff(map[string]any{"page": 1.0, "token": "m4", "version": 1.0, "revision": "revision-0"}, storedCheckpoint(t, uncounted.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the checkpoint after the first page of an enumeration with no total (-want +got):\n%s", diff)
	}
}

// A run stopped with a checkpoint that carries no page count, as one stored before the provider
// reported a total, is resumed from it like any other, and the next page it makes durable records the
// count (ADR-0095).
func TestACheckpointWithoutAPageCountResumes(t *testing.T) {
	w := realWorld(t, setup{PageSize: 3, Messages: mixed.Messages, Totals: true})
	w.step(t)
	w.crash(t, betweenSteps)
	if _, err := superuser(t).Exec(t.Context(), "UPDATE job_runs SET checkpoint = checkpoint - 'of' WHERE account_id = $1", w.account); err != nil {
		t.Fatal(err)
	}
	w.reported.Checkpoint.Of = 0
	w.expect.Checkpoint.Of = 0
	w.open(t)
	w.persistence(t)
	if diff := cmp.Diff(map[string]any{"page": 1.0, "token": "m4", "version": 1.0, "revision": "revision-0"}, storedCheckpoint(t, w.account, "run2"), compare.Options); diff != "" {
		t.Errorf("the resumed run's first checkpoint (-want +got):\n%s", diff)
	}
	w.step(t)
	if diff := cmp.Diff(map[string]any{"page": 2.0, "token": "m7", "of": 3.0, "version": 1.0, "revision": "revision-0"}, storedCheckpoint(t, w.account, "run2"), compare.Options); diff != "" {
		t.Errorf("the checkpoint after the resumed run's first page (-want +got):\n%s", diff)
	}
	w.progress(t)
}

// A run resuming an enumeration made under another scanner starts it over, and the page count the
// old enumeration's checkpoint carried goes with its token, so the restarted run's checkpoint carries
// no count until a page of the new enumeration reports one, and then carries that page's (ADR-0095,
// ADR-0096).
func TestAnEnumerationStartedOverUnderAnotherScannerDropsItsPageCount(t *testing.T) {
	w := realWorld(t, setup{PageSize: 3, Messages: mixed.Messages, Totals: true})
	w.step(t)
	if got := storedCheckpoint(t, w.account, "run1")["of"]; got != 3.0 {
		t.Fatalf("the first run's checkpoint carries of %v, want 3", got)
	}
	w.rescan(t)
	w.crash(t, betweenSteps)
	w.open(t)
	w.persistence(t)
	if diff := cmp.Diff(map[string]any{"page": 0.0, "token": "", "version": 1.0, "revision": "revision-1"}, storedCheckpoint(t, w.account, "run2"), compare.Options); diff != "" {
		t.Errorf("the restarted run's first checkpoint (-want +got):\n%s", diff)
	}
	w.step(t)
	if diff := cmp.Diff(map[string]any{"page": 1.0, "token": "m4", "of": 3.0, "version": 1.0, "revision": "revision-1"}, storedCheckpoint(t, w.account, "run2"), compare.Options); diff != "" {
		t.Errorf("the checkpoint after the restarted run's first page (-want +got):\n%s", diff)
	}
	w.progress(t)
}

// A page's commit is one transaction. When one of its statements fails, none of the page's messages,
// masking events or sender statistics is left, and the checkpoint stays where it was, so the page is
// taken again whole by the next run.
func TestAPageThatFailsToCommitLeavesNothing(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	before := w.inspect(t)
	revoke(t, "UPDATE (checkpoint) ON job_runs")
	if w.step(t) {
		t.Fatal("the pass ended")
	}
	after := w.inspect(t)
	if diff := cmp.Diff(before.Messages, after.Messages, compare.Options); diff != "" {
		t.Errorf("a commit that failed left messages (-before +after):\n%s", diff)
	}
	if diff := cmp.Diff(before.Events, after.Events, compare.Options); diff != "" {
		t.Errorf("a commit that failed left masking events (-before +after):\n%s", diff)
	}
	if diff := cmp.Diff(before.Senders, after.Senders, compare.Options); diff != "" {
		t.Errorf("a commit that failed left sender statistics (-before +after):\n%s", diff)
	}
	latest, _ := after.latest()
	if diff := cmp.Diff(run{ID: "run1", State: "failed", Progress: w.reported}, latest, compare.Options); diff != "" {
		t.Errorf("after the failed commit the run is not failed at the progress last reported (-want +got):\n%s", diff)
	}
}

// revoke revokes a grant of backfill's role for the rest of the test, and grants it again when the test
// ends, so the tests after it see the role as the migration chain made it.
func revoke(t *testing.T, privilege string) {
	t.Helper()
	if _, err := superuser(t).Exec(t.Context(), "REVOKE "+privilege+" FROM "+role); err != nil {
		t.Fatal(err)
	}
	url := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO "+role); err != nil {
			t.Error(err)
		}
	})
}

// The model the crash harness reduces against agrees with PostgreSQL on a sequence with a crash inside
// a page and a run failed by throttles, so its checks carry over (ADR-0069).
func TestTheModelAgreesWithPostgreSQL(t *testing.T) {
	drive := func(w *world) state {
		w.step(t)
		w.crash(t, insidePage)
		w.open(t)
		w.throttle = core.MaxAttempts
		w.step(t)
		w.finish(t, 10)
		return w.inspect(t)
	}
	model, database := drive(modelWorld(t, mixed)), drive(realWorld(t, mixed))
	if diff := cmp.Diff(model, database, compare.Options); diff != "" {
		t.Errorf("the model and PostgreSQL disagree (-model +database):\n%s", diff)
	}
	if !slices.ContainsFunc(database.Runs, func(r run) bool { return r.State == "failed" }) {
		t.Error("no run failed, so the sequence did not reach what it meant to")
	}
}

// A page whose earlier attempts failed is recorded as recovered only once it is durable, in the
// transaction that makes it durable. When that commit fails, nothing says the page recovered, and the
// run that takes it again records what became of it.
func TestAPageIsRecordedRecoveredOnlyOnceDurable(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	w.throttle = 2
	revoke(t, "UPDATE (checkpoint) ON job_runs")
	if w.step(t) || w.pass != nil {
		t.Fatal("a page whose commit failed did not fail the run")
	}
	if got := failures(t, w.account); len(got) != 0 {
		t.Errorf("a page that never became durable has the failed items %v, want none", got)
	}
}

// Refusing a page token the run got from the provider in this run is a failure, not a reason to start
// over. Only the token a run resumed from, which the provider may no longer honour, starts the
// enumeration over.
func TestARefusedTokenTheRunGotFailsTheRun(t *testing.T) {
	w := realWorld(t, mixed)
	w.step(t)
	w.refuse = 1
	if w.step(t) || w.pass != nil {
		t.Fatal("a refused token the run got in this run did not fail the run")
	}
	if diff := cmp.Diff([]string{"run1 page 2 2 validation 1 abandoned"}, failures(t, w.account), compare.Options); diff != "" {
		t.Errorf("the failed items (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"start 0", "progress 1", "failure"}, timeline(t, w.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the timeline, with no start over (-want +got):\n%s", diff)
	}
}

// A failure that is not the provider's, such as the rate limiter failing to record a call, fails the
// run and records no failed page, since the page did nothing wrong. A later run resumes from the
// checkpoint.
func TestAFailureNotTheProvidersFailsTheRunWithNoFailedPage(t *testing.T) {
	w := realWorld(t, mixed)
	broken := w.deps
	broken.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, errors.New("telling the rate limiter how the call went failed")
	}
	p, err := pass1.Open(t.Context(), broken, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Next(t.Context()); err == nil {
		t.Fatal("Next succeeded")
	}
	if got := failures(t, w.account); len(got) != 0 {
		t.Errorf("the failed items are %v, want none", got)
	}
	if diff := cmp.Diff([]string{"start 0", "failure"}, timeline(t, w.account, "run1"), compare.Options); diff != "" {
		t.Errorf("the timeline (-want +got):\n%s", diff)
	}
}
