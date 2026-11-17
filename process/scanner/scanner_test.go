package scanner_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// worker is what the worker's configuration needs on the command line beyond its defaults. No pool
// is opened, so the database and the key files are never read.
var worker = []string{
	"--database.host=db.example", "--database.name=mailbox", "--database.user=mediated_mailbox_app",
	"--database.password_file=/absent/password", "--credential.public_key_file=/absent/public",
	"--credential.private_key_files=[/absent/private]", "--probe_listen=127.0.0.1:0",
}

// scanners runs the worker's configuration type through a process of two job kinds, each of whose
// Build asks for the scanner, with args beyond worker, and returns what each got.
func scanners(t *testing.T, args ...string) [2]scan.Scanner {
	t.Helper()
	var got [2]scan.Scanner
	built := make(chan error, 2)
	job := func(i int, name string) process.Job[config.Worker] {
		return process.Job[config.Worker]{Name: name, Build: func(ctx context.Context, p process.Capabilities, c config.Worker) (schedule.Plan, error) {
			s, err := scanner.Get(ctx, p, c)
			got[i] = s
			built <- err
			return schedule.Plan{
				Keys: func(context.Context) ([]string, error) { return nil, nil },
				Job:  func(string) (schedule.Job, error) { return schedule.Job{}, errors.New("no key is listed") },
			}, err
		}}
	}
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- process.RunContext(ctx, append(append([]string(nil), worker...), args...), nil, io.Discard, io.Discard,
			process.Spec[config.Worker]{Name: "worker", Jobs: []process.Job[config.Worker]{job(0, "one"), job(1, "two")}})
	}()
	for range 2 {
		select {
		case err := <-built:
			if err != nil {
				t.Fatal(err)
			}
		case err := <-done:
			t.Fatalf("the process ended before its builds: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("the builds did not run within ten seconds")
		}
	}
	stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return got
}

// recorded is what a verdict of s records of the scanner.
func recorded(s scan.Scanner) string {
	v := s.Scan("Your code is 419283")
	return fmt.Sprintf("version %d revision %s", v.Version(), v.Revision())
}

// The slot row's part for the scanner, and F9's part of VERIFICATIONS' rows for a section's revision
// and for the default scanner section's known revision. Two job kinds that ask for the scanner get one
// built from the process's scanner section, under the revision the configuration library took of it,
// which for the worker's default section is the value checked in here, never derived by this build
// (ADR-0078, ADR-0120, ADR-0142).
func TestTheScannerIsOnePerProcessUnderItsSectionsRevision(t *testing.T) {
	got := scanners(t)
	const shared = "version 1 revision c9fa3ff13948e4f89ac094b91a27455c"
	for i, s := range got {
		if r := recorded(s); r != shared {
			t.Errorf("job kind %d's scanner records %s, want %s", i, r, shared)
		}
	}
}

// F9's part of VERIFICATIONS' row for a section's revision. The revision a verdict records follows
// the scanner section, which a flag changes, and a change to another section, such as a job kind's
// own, leaves it as it was (ADR-0078, ADR-0120).
func TestTheRevisionFollowsTheScannerSectionAlone(t *testing.T) {
	base := recorded(scanners(t)[0])
	for _, c := range []struct {
		name    string
		args    []string
		changed bool
	}{
		{"a flag changing the scanner", []string{"--scanner.window=9"}, true},
		{"a flag restating the scanner's default", []string{"--scanner.window=8"}, false},
		{"a flag changing a job kind's section", []string{"--sync.decisions_per_tick=50"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := recorded(scanners(t, c.args...)[0]); (got != base) != c.changed {
				t.Errorf("the scanner records %s, from %s, want changed %v", got, base, c.changed)
			}
		})
	}
}

// The body-processing histogram is defined once, under the name and labels each job kind and
// operation that converts and scans registers it with, and a nil one times nothing (ADR-0125).
func TestTheBodyProcessingHistogramIsDefinedOnce(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, err := scanner.NewProcessing(reg)
	if err != nil {
		t.Fatal(err)
	}
	p.Since(scanner.StepConvert, time.Now())
	p.Since(scanner.StepScan, time.Now())
	var nothing *scanner.Processing
	nothing.Since(scanner.StepScan, time.Now())
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 1 || families[0].GetName() != "mediated_mailbox_body_processing_duration_seconds" {
		t.Fatalf("registered %v, want the body-processing histogram alone", families)
	}
	steps := map[string]uint64{}
	for _, m := range families[0].GetMetric() {
		steps[m.GetLabel()[0].GetValue()] = m.GetHistogram().GetSampleCount()
	}
	if steps["convert"] != 1 || steps["scan"] != 1 || len(steps) != 2 {
		t.Errorf("the steps timed %v, want one conversion and one scan", steps)
	}
}
