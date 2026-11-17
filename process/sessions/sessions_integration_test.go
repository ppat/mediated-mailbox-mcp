//go:build integration

package sessions_test

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/process/accounts"
	"github.com/ppat/mediated-mailbox-mcp/process/policies"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// ids returns the identifiers of the accounts the Session has in force.
func ids(s *sessions.Session) []string {
	var out []string
	for _, a := range s.Accounts() {
		out = append(out, a.ID)
	}
	return out
}

// told records the accounts in force each time the Session tells it of a reload.
type told struct {
	mu   sync.Mutex
	seen []string
}

func (o *told) observe(s *sessions.Session) func() {
	return func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.seen = append(o.seen, strings.Join(ids(s), ","))
	}
}

func (o *told) reloads() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.seen...)
}

// F9's part of VERIFICATIONS' rows for the Session's failure table. A reload whose read of the
// accounts fails keeps in force what the reload before it read, tells no one and returns the error. One
// whose policy load fails puts its accounts in force all the same, so an account newly listed is held
// and one no longer listed is dropped, tells those who asked, keeps the policy it had, and returns an
// error wrapping ErrPolicyNotLoaded, and an account newly listed is not worked before its policy has
// loaded (ADR-0090, ADR-0041).
func TestAFailedReloadKeepsWhatItCouldNotRead(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	listed(t, conn, "personal")
	baseRule(t, conn)
	r := start(t)
	s := r.session()
	r.waitReady(t, 10*time.Second)
	var o told
	s.AfterReload(o.observe(s))
	if err := s.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}

	listed(t, conn, "work")
	revoke(t, conn, "SELECT ON accounts")
	err := s.Reload(t.Context())
	if !errors.Is(err, accounts.ErrUntrustedRead) || errors.Is(err, sessions.ErrPolicyNotLoaded) {
		t.Errorf("a reload whose read failed returned %v, want the failed read", err)
	}
	if diff := cmp.Diff([]string{"personal"}, ids(s), compare.Options); diff != "" {
		t.Errorf("after the failed read the accounts in force (-want +got):\n%s", diff)
	}

	must(t, conn, "GRANT SELECT ON accounts TO "+role)
	revoke(t, conn, "SELECT ON policy_rules")
	err = s.Reload(t.Context())
	if !errors.Is(err, sessions.ErrPolicyNotLoaded) || !errors.Is(err, policies.ErrUntrustedRead) {
		t.Errorf("a reload whose policy read failed returned %v, want the policy not loaded", err)
	}
	if diff := cmp.Diff([]string{"personal", "work"}, ids(s), compare.Options); diff != "" {
		t.Errorf("after the policy's failure the accounts in force (-want +got):\n%s", diff)
	}
	if s.Policy("personal").Composed().RestrictsAll() {
		t.Error("the account read before lost its policy to a reload whose policy read failed")
	}
	if !s.Policy("work").Composed().RestrictsAll() {
		t.Error("an account newly listed was given a policy no load read for it")
	}
	if diff := cmp.Diff([]string{"personal"}, s.AccountsWithPolicy(), compare.Options); diff != "" {
		t.Errorf("the accounts with a policy after the policy's failure (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal", "personal,work"}, o.reloads(), compare.Options); diff != "" {
		t.Errorf("the reloads told (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for the Session's first reload, for the Session. The first reload does not hold
// up the start. While no reload has loaded the list and the policy, the Session reports not ready and
// the reload is tried again within seconds, though the interval is an hour, it reports ready once one
// has, and a later reload whose policy fails leaves it ready (ADR-0090, ADR-0142).
func TestTheSessionIsReadyOnceAReloadHasLoadedEverything(t *testing.T) {
	for _, c := range []struct{ name, privilege string }{
		{"the list", "SELECT ON accounts"},
		{"the policy", "SELECT ON policy_rules"},
	} {
		t.Run(c.name, func(t *testing.T) {
			conn := superuser(t)
			reset(t, conn)
			listed(t, conn, "personal")
			revoke(t, conn, c.privilege)
			r := start(t)
			time.Sleep(time.Second)
			status, body := r.probe(t, "/readyz")
			if status != http.StatusServiceUnavailable || !strings.Contains(body, "session") {
				t.Fatalf("with the first reload failing the probe answered %d %q, want not ready naming the Session", status, body)
			}
			must(t, conn, "GRANT "+c.privilege+" TO "+role)
			r.waitReady(t, 10*time.Second)

			revoke(t, conn, "SELECT ON policy_rules")
			if err := r.session().Reload(t.Context()); !errors.Is(err, sessions.ErrPolicyNotLoaded) {
				t.Fatalf("a later reload with the policy refused returned %v", err)
			}
			if status, body := r.probe(t, "/readyz"); status != http.StatusOK {
				t.Errorf("after a later reload's policy failed the probe answered %d %q, want ready", status, body)
			}
		})
	}
}

// The slot row's part for the Session. Two job kinds of one process that ask for the Session get the
// one the first call built, built from the process, so its reload series carries the program's name
// and no job kind. Its reload is its own upkeep on the process's scheduler, which the scheduler's
// series observe nothing of (ADR-0125, ADR-0142).
func TestTheSessionIsOnePerProcessBuiltFromTheProcess(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	listed(t, conn, "personal")
	r := start(t)
	if r.sessions[0] != r.sessions[1] {
		t.Fatal("two job kinds got two Sessions")
	}
	r.waitReady(t, 10*time.Second)
	_, metrics := r.probe(t, "/metrics")
	if !strings.Contains(metrics, `mediated_mailbox_executioncontext_reload_duration_seconds_count{component="test"}`) {
		t.Errorf("the Session's reload series is not labelled with the program alone:\n%s", grepLines(metrics, "reload_duration"))
	}
	for _, kind := range []string{`job_kind="session"`, `job_kind=""`} {
		if strings.Contains(grepLines(metrics, "mediated_mailbox_job_"), kind) {
			t.Errorf("the scheduler's series observe the Session's reload:\n%s", grepLines(metrics, "mediated_mailbox_job_"))
		}
	}
}

// grepLines returns the lines of text that contain s.
func grepLines(text, s string) string {
	var out []string
	for line := range strings.Lines(text) {
		if strings.Contains(line, s) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "")
}

// timedReloads returns the count of the Session's reload series the process serves.
func timedReloads(t *testing.T, r *running) int {
	t.Helper()
	_, metrics := r.probe(t, "/metrics")
	m := regexp.MustCompile(`mediated_mailbox_executioncontext_reload_duration_seconds_count\{[^}]*\} (\d+)`).FindStringSubmatch(metrics)
	if m == nil {
		t.Fatalf("the Session's reload series is not served:\n%s", grepLines(metrics, "reload_duration"))
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Every reload its caller did not cancel is timed, one that succeeds and one whose read fails, and a
// reload its caller cancelled is not (ADR-0125).
func TestEachReloadItsCallerDidNotCancelIsTimed(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	listed(t, conn, "personal")
	r := start(t)
	r.waitReady(t, 10*time.Second)
	s := r.session()
	before := timedReloads(t, r)
	if err := s.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	revoke(t, conn, "SELECT ON accounts")
	if err := s.Reload(t.Context()); err == nil {
		t.Fatal("a reload with the listing refused succeeded")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.Reload(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled reload returned %v, want context.Canceled", err)
	}
	if got := timedReloads(t, r) - before; got != 2 {
		t.Errorf("reloads timed = %d, want 2, the one that succeeded and the one whose read failed", got)
	}
}

// A process whose Session holds no connections works no account that calls a provider, and works the
// accounts that have a policy in a job kind that calls none (ADR-0090, ADR-0119).
func TestWithoutConnectionsNoAccountIsWorkable(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	listed(t, conn, "personal")
	r := start(t)
	r.waitReady(t, 10*time.Second)
	if got := r.session().Workable(); len(got) != 0 {
		t.Errorf("a Session with no connections holds %v workable", got)
	}
	if diff := cmp.Diff([]string{"personal"}, r.session().AccountsWithPolicy(), compare.Options); diff != "" {
		t.Errorf("the accounts with a policy (-want +got):\n%s", diff)
	}
}

// F9's part of VERIFICATIONS' row for the Session's reload schedule. Once a reload has loaded
// everything, a scheduled reload that fails keeps in force what was, is logged by the Session, and is
// tried again at the next interval rather than after the scheduler's backoff, so the scheduler never
// counts it a failed run (ADR-0090).
func TestOnceLoadedAFailedReloadWaitsForTheNextInterval(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	listed(t, conn, "personal")
	r := startWith(t, "--account_reload_interval=1s")
	r.waitReady(t, 10*time.Second)
	revoke(t, conn, "SELECT ON accounts")
	const logged = "the Session was not reloaded in full, so what it held stays in force"
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(r.logs.String(), logged); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no scheduled reload failed and was logged within ten seconds of a one-second interval:\n%s", r.logs.String())
		}
	}
	if strings.Contains(r.logs.String(), "the run failed") {
		t.Errorf("the scheduler counted a failed reload after the first load as a failed run:\n%s", grepLines(r.logs.String(), "the run failed"))
	}
	if diff := cmp.Diff([]string{"personal"}, ids(r.session()), compare.Options); diff != "" {
		t.Errorf("the accounts in force after a failed scheduled reload (-want +got):\n%s", diff)
	}
}
