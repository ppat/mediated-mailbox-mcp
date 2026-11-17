//go:build integration

package database_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// program is a configuration type composed as a program's that connects, with the process's parts
// inline and the database part.
type program struct {
	config.Process    `yaml:",inline"`
	config.Operations `yaml:",inline"`
	Database          config.Database `yaml:"database"`
}

func (p program) Check() error { return p.Database.Check() }

func (program) Defaults(int) program {
	return program{Process: config.DefaultProcess(), Operations: config.DefaultOperations(), Database: config.DefaultDatabase(2)}
}

func (p program) DatabasePart() config.Database { return p.Database }

// flags returns the flags naming the database url points at, with its password in a mounted file.
func flags(t *testing.T, url string) []string {
	t.Helper()
	u, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	path := filepath.Join(t.TempDir(), "password")
	//nolint:gosec // The path is in the test's own directory.
	if err := os.WriteFile(path, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{
		"--database.host=" + u.Hostname(), "--database.port=" + u.Port(), "--database.name=" + strings.TrimPrefix(u.Path, "/"),
		"--database.user=" + u.User.Username(), "--database.sslmode=disable", "--database.password_file=" + path,
		"--probe_listen=127.0.0.1:0",
	}
}

// lockedBuffer is a destination the process and the test write and read from different goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func idle() schedule.Plan {
	return schedule.Plan{
		Keys: func(context.Context) ([]string, error) { return nil, nil },
		Job:  func(string) (schedule.Job, error) { return schedule.Job{}, errors.New("no key is listed") },
	}
}

// metricsOf asks the running process's metrics endpoint, whose address the serving record names.
func metricsOf(t *testing.T, logs func() string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for line := range strings.Lines(logs()) {
			if !strings.Contains(line, `"msg":"serving"`) {
				continue
			}
			_, rest, _ := strings.Cut(line, `"probe_listen":"`)
			addr, _, _ := strings.Cut(rest, `"`)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/metrics", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			if err := errors.Join(err, resp.Body.Close()); err != nil {
				t.Fatal(err)
			}
			// The metrics endpoint answers a gather error, such as two series of one name and one set
			// of label values, with an error status rather than the series.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("/metrics answered %d:\n%s", resp.StatusCode, body)
			}
			return string(body)
		}
	}
	t.Fatalf("the process wrote no serving record:\n%s", logs())
	return ""
}

// Each job kind gets a pool of its own, opened from the one database part and labelled with its job
// kind, asked for again it is the same pool, and a thing the process shares gets the process's one
// pool, labelled with the program alone, whichever part asks first. In this process, which holds job
// kinds' pools beside the process's pool in one registry, the job kinds' series carry their job_kind
// and the process's pool's carry no job_kind at all, the scrape succeeds, and no series appears
// twice. Every pool is closed once the process has stopped (ADR-0119, ADR-0125, ADR-0142,
// ADR-0143).
func TestEachJobKindHasAPoolOfItsOwnAndTheProcessOneMore(t *testing.T) {
	shared := process.NewOnePerProcess[*pgxpool.Pool]("shared")
	pools := map[string][]*pgxpool.Pool{}
	job := func(name string) process.Job[program] {
		return process.Job[program]{Name: name, Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			for range 2 {
				pool, err := database.Get(ctx, p, c)
				if err != nil {
					return schedule.Plan{}, err
				}
				pools[name] = append(pools[name], pool)
			}
			pool, err := shared.Get(ctx, p, func(ctx context.Context, sp process.Capabilities) (*pgxpool.Pool, func(), error) {
				pool, err := database.Get(ctx, sp, c)
				return pool, nil, err
			})
			pools["shared"] = append(pools["shared"], pool)
			return idle(), err
		}}
	}
	var logs lockedBuffer
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- process.RunContext(ctx, flags(t, postgres.URL(t)), nil, &logs, io.Discard, process.Spec[program]{
			Name: "test", Jobs: []process.Job[program]{job("first"), job("second")},
		})
	}()
	metrics := metricsOf(t, logs.String)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	first, second, process := pools["first"], pools["second"], pools["shared"]
	if first[0] != first[1] || second[0] != second[1] || first[0] == second[0] || process[0] != process[1] || process[0] == first[0] || process[0] == second[0] {
		t.Errorf("the pools are first %p, second %p and the process's %p, want one per job kind, each asked for twice, and one more", first, second, process)
	}
	for _, line := range []string{
		`mediated_mailbox_db_pool_max_connections{component="test",job_kind="first"} 2`,
		`mediated_mailbox_db_pool_max_connections{component="test",job_kind="second"} 2`,
		`mediated_mailbox_db_pool_max_connections{component="test"} 2`,
	} {
		if !strings.Contains(metrics, "\n"+line) {
			t.Errorf("/metrics does not serve %s:\n%s", line, metrics)
		}
	}
	seen := map[string]bool{}
	for line := range strings.Lines(metrics) {
		if strings.HasPrefix(line, "#") {
			continue
		}
		series, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		if seen[series] {
			t.Errorf("/metrics serves %s twice", series)
		}
		seen[series] = true
		if strings.HasPrefix(series, "mediated_mailbox_db_") && strings.Contains(series, `job_kind=""`) {
			t.Errorf("/metrics serves %s, a pool series with an empty job_kind", series)
		}
	}
	for _, pool := range []*pgxpool.Pool{first[0], second[0], process[0]} {
		if err := pool.Ping(t.Context()); err == nil {
			t.Error("a pool answers once the process has stopped, want every pool closed")
		}
	}
}

// Get runs the schema check on a pool before it first returns it, so a schema the chain cannot run on
// fails the build that asked and stops the start before anything serves, and no pool is returned
// (ADR-0140, ADR-0142). The migrator's pool is opened through ForMigration on the same schema with no
// check.
func TestGetChecksTheSchemaAndForMigrationDoesNot(t *testing.T) {
	k := int64(0)
	entries, err := os.ReadDir("../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		var v int64
		if _, err := fmt.Sscanf(e.Name(), "%d_", &v); err == nil {
			k = max(k, v)
		}
	}
	behind := postgres.ChainBefore(t, "../../db/migrations", k)
	var got *pgxpool.Pool
	// A check that let the schema through would leave the process serving, so a deadline ends the run.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err = process.RunContext(ctx, flags(t, behind.URL), nil, io.Discard, io.Discard, process.Spec[program]{
		Name: "test", Jobs: []process.Job[program]{{Name: "kind", Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			pool, err := database.Get(ctx, p, c)
			got = pool
			return idle(), err
		}}},
	})
	if !errors.Is(err, migrations.ErrNotMigrated) || got != nil {
		t.Errorf("on a schema missing the chain's newest migration the run returned %v and the pool %p, want the build refused and no pool", err, got)
	}

	var migrator []string
	var migratorPool *pgxpool.Pool
	err = process.RunContext(t.Context(), append(flags(t, behind.URL)[:6], "--application_user="+postgres.ApplicationUser), nil, io.Discard, io.Discard, process.Spec[config.Migrator]{
		Name: "migrator", Task: &process.Task[config.Migrator]{Build: func(ctx context.Context, p process.Capabilities, c config.Migrator) (func(context.Context) error, error) {
			pool, err := database.ForMigration(ctx, p, c)
			if err != nil {
				return nil, err
			}
			migratorPool = pool
			return func(ctx context.Context) error {
				if err := pool.Ping(ctx); err != nil {
					return err
				}
				migrator = append(migrator, "answered")
				return nil
			}, nil
		}},
	})
	if err != nil || len(migrator) != 1 {
		t.Errorf("ForMigration on the same schema returned %v, want a pool with no check", err)
	}
	if migratorPool == nil || migratorPool.Ping(t.Context()) == nil {
		t.Error("the migrator's pool answers once its run has returned, want it closed by then")
	}
}

// The schema check refuses a schema that has not applied the chain and one whose compatibility floor
// is above it, and accepts the chain's own schema and a newer one whose floor the chain meets, read as
// the application user every program but the migrator connects as (ADR-0140, ADR-0143).
func TestTheSchemaCheckRefusesASchemaTheChainCannotRunOn(t *testing.T) {
	postgres.SchemaChecks(t, "../../db/migrations", database.CheckSchema, postgres.ApplicationUser)
}

// A process that runs no job kind holds no job kind's pool, so its one pool serves its series with
// the program's label alone and no job_kind, as the UI's does (ADR-0125, ADR-0142).
func TestAPoolOfAProcessWithoutJobKindsCarriesNoJobKind(t *testing.T) {
	url := postgres.URL(t)
	var logs lockedBuffer
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		args := append(flags(t, url), "--listen=127.0.0.1:0")
		done <- process.RunContext(ctx, args, nil, &logs, io.Discard, process.Spec[served]{
			Name: "test", Endpoints: []process.Endpoint[served]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, c served) (http.Handler, error) {
				_, err := database.Get(ctx, p, c)
				return http.NotFoundHandler(), err
			}}},
		})
	}()
	metrics := metricsOf(t, logs.String)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metrics, "\n"+`mediated_mailbox_db_pool_max_connections{component="test"} 2`) || strings.Contains(metrics, "job_kind") {
		t.Errorf("an endpoint's pool in a process with no job kind serves its series with a job_kind, or not at all:\n%s", metrics)
	}
}
