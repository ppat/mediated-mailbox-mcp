//go:build integration

package deltasync

import (
	"context"
	"errors"
	"io"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// assembled runs the worker's configuration in a process whose one job kind is delta sync, built as
// its Build builds it, over the provider fake, and returns delta sync once the process is ready, so a
// test asks the account's job itself at times it chooses. The job kind's plan lists nothing, so the
// scheduler asks no job of its own. The account personal is connected through an OAuth client, with
// both of its backfill passes ended. The process stops when the test ends.
func assembled(t *testing.T) *DeltaSync {
	t.Helper()
	conn := superuser(t)
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"rate_grants", "rate_state", "account_state", "accounts", "oauth_clients",
	} {
		must(t, conn, "DELETE FROM "+table)
	}
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	public, err := seal.ParsePublicKey(key.PublicKey().Bytes())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, b := range map[string][]byte{"private": seed, "public": key.PublicKey().Bytes()} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sealed := func(plaintext string, c seal.Context) []byte {
		b, err := public.Seal([]byte(plaintext), c)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'household-id', $1)",
		sealed("household-secret", seal.ClientSecret("household")))
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('personal', 'gmail', 'household')")
	must(t, conn, "INSERT INTO account_state (account_id, credential, backfill_pass1_complete, backfill_pass2_complete) VALUES ('personal', $1, true, true)",
		sealed("personal-token", seal.AccountCredential("personal")))

	u, err := neturl.Parse(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	passwordFile := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{
		"--database.host=" + u.Hostname(), "--database.port=" + u.Port(), "--database.name=" + strings.TrimPrefix(u.Path, "/"),
		"--database.user=" + postgres.ApplicationUser, "--database.sslmode=disable", "--database.password_file=" + passwordFile,
		"--credential.public_key_file=" + filepath.Join(dir, "public"), "--credential.private_key_files=[" + filepath.Join(dir, "private") + "]",
		"--probe_listen=127.0.0.1:0", "--account_reload_interval=1h",
	}
	list := fake.Adapters(func(account string, _ *fake.Token) (*fake.Throttled, error) {
		f, err := fake.New(fake.Config{Account: account})
		return fake.Throttle(f, fake.Never(), time.Now), err
	}, fake.Provider{Name: "gmail", UsesClient: true})
	built := make(chan *DeltaSync, 1)
	spec := process.Spec[config.Worker]{Name: "worker", Jobs: []process.Job[config.Worker]{{
		Name: Kind,
		Build: func(ctx context.Context, p process.Capabilities, c config.Worker) (schedule.Plan, error) {
			d, err := assemble(ctx, p, c)
			if err != nil {
				return schedule.Plan{}, err
			}
			built <- d
			return schedule.Plan{
				Keys: func(context.Context) ([]string, error) { return nil, nil },
				Job:  func(string) (schedule.Job, error) { return schedule.Job{}, errors.New("no key is listed") },
			}, nil
		},
	}}}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- process.RunContext(ctx, args, nil, io.Discard, io.Discard, spec, connections.WithAdapters(list))
	}()
	t.Cleanup(func() {
		stop()
		if err := <-done; err != nil {
			t.Errorf("the process ended with %v", err)
		}
	})
	var d *DeltaSync
	select {
	case d = <-built:
	case err := <-done:
		t.Fatalf("the process ended before delta sync was built: %v", err)
	}
	// A unit of work begins only on an account the Session's list in force holds, which its first
	// reload, run once the process serves, puts in force.
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if workable, err := d.session.Workable(); err == nil && len(workable) == 1 {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatal("the Session did not list the account within twenty seconds")
		}
	}
}

// ticks returns how many of the account's ticks were recorded.
func ticks(t *testing.T, account string) int {
	t.Helper()
	var n int
	if err := superuser(t).QueryRow(t.Context(), "SELECT count(*) FROM job_runs WHERE account_id = $1 AND workload = 'sync'", account).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A delta sync job asked before its sync interval has passed since its last tick answers that it is
// not due and ticks nothing, so a wake that comes early, such as the end of a backoff, never ticks
// twice inside one interval, and the job asked once the interval has passed ticks again (ADR-0103,
// ADR-0119).
func TestAJobAskedBeforeItsIntervalDoesNotTick(t *testing.T) {
	job := assembled(t).job("personal")
	t0 := time.UnixMilli(1_790_000_000_000)

	if err := job.Run(t.Context(), schedule.Ask{At: t0, Phase: t0}); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(t.Context(), schedule.Ask{At: t0.Add(time.Minute), Phase: t0}); !errors.Is(err, schedule.ErrNotDue) {
		t.Errorf("the job asked a minute after its tick returned %v, want that it is not due", err)
	}
	if n := ticks(t, "personal"); n != 1 {
		t.Errorf("the account has %d ticks after an early ask, want 1", n)
	}
	if err := job.Run(t.Context(), schedule.Ask{At: t0.Add(5 * time.Minute), Phase: t0.Add(5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if n := ticks(t, "personal"); n != 2 {
		t.Errorf("the account has %d ticks once the interval passed, want 2", n)
	}
}

// A tick asked off the phase, at the end of a backoff, is recorded at the ticker's tick before it, so
// the ticker's next tick is due and the ticks keep the phase the scheduler asks them on. Recorded at
// its own time, the ticker's next tick would come less than an interval after it and read not due,
// and the account would wait one interval more (ADR-0119, ADR-0103).
func TestATickAtTheEndOfABackoffKeepsThePhase(t *testing.T) {
	job := assembled(t).job("personal")
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
	if n := ticks(t, "personal"); n != 3 {
		t.Errorf("the account has %d ticks, want 3, one at the backoff's end and one at the ticker's next tick", n)
	}
}
