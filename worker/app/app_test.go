package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/app"
)

// run runs the worker through its spec, as its main.go does, for a start meant to stop before it
// serves, writing its records to stdout and its banner to stderr, and returns the start's error. It
// fails the test when the start serves for ten seconds instead.
func run(t *testing.T, args, environ []string, stdout, stderr io.Writer) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err := process.RunContext(ctx, args, environ, stdout, stderr, app.Spec())
	if ctx.Err() != nil {
		t.Fatalf("the start did not stop within ten seconds, it returned %v", err)
	}
	return err
}

// start is run with nothing written anywhere a test reads.
func start(t *testing.T, args ...string) error {
	t.Helper()
	return run(t, args, nil, io.Discard, io.Discard)
}

// absentKeys names key files that do not exist.
var absentKeys = []string{"--credential.public_key_file=/absent/public", "--credential.private_key_files=[/absent/private]"}

// database names a database whose password file does not exist, so a start that gets as far as
// opening a pool stops there, before any connection is made.
var database = []string{"--database.host=db.example", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent/password"}

// The worker's defaults are core/config's, in which the database's host, name, user and password
// file and the key files are required, so a start that sets none of them is refused at the first,
// the database's host, before anything is read or connected (ADR-0143).
func TestAStartWithNoRequiredValueIsRefused(t *testing.T) {
	err := start(t)
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("the start returned %v, want %q", err, wantErr)
	}
	err = start(t, append([]string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password"}, absentKeys...)...)
	wantErr = "loading the configuration: database.user is required, and neither the file, MEDIATED_MAILBOX_DATABASE__USER nor --database.user sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("the start returned %v, want %q", err, wantErr)
	}
}

// The start validates the merged configuration before it configures any connection.
func TestAnEmptyHostRefusesTheStart(t *testing.T) {
	err := start(t, append([]string{
		"--database.host=", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent",
	}, absentKeys...)...)
	want := "validating the configuration: database.host is empty"
	if err == nil || err.Error() != want {
		t.Errorf("the start returned %v, want %q", err, want)
	}
}

// The pool size, both reload intervals, the sync interval, the first window, the bound on a tick's
// decisions and each job kind's concurrency must be positive, and the start refuses one that is not
// before it reads a key file.
func TestATickThatCannotRunRefusesTheStart(t *testing.T) {
	for _, c := range []struct{ arg, want string }{
		{"--sync.sync_interval=0s", "validating the configuration: sync.sync_interval is not positive"},
		{"--sync.first_window=-1h", "validating the configuration: sync.first_window is not positive"},
		{"--sync.decisions_per_tick=0", "validating the configuration: sync.decisions_per_tick 0 is not positive"},
		{"--sync.concurrency=0", "validating the configuration: sync.concurrency 0 is not positive"},
		{"--backfill.concurrency=0", "validating the configuration: backfill.concurrency 0 is not positive"},
		{"--reload_interval=0s", "validating the configuration: reload_interval is not positive"},
		{"--account_reload_interval=0s", "validating the configuration: account_reload_interval is not positive"},
		{"--database.pool_size=0", "validating the configuration: database.pool_size 0 is not positive"},
	} {
		t.Run(c.arg, func(t *testing.T) {
			err := start(t, slices.Concat(database, absentKeys, []string{c.arg})...)
			if err == nil || err.Error() != c.want {
				t.Errorf("the start returned %v, want %q", err, c.want)
			}
		})
	}
	// The empty password file and the empty user are each refused by their own path.
	for _, key := range []string{"password_file", "user"} {
		args := slices.DeleteFunc(slices.Clone(database), func(a string) bool { return strings.HasPrefix(a, "--database."+key+"=") })
		err := start(t, slices.Concat(args, absentKeys, []string{"--database." + key + "="})...)
		if want := "validating the configuration: database." + key + " is empty"; err == nil || err.Error() != want {
			t.Errorf("the start returned %v, want %q", err, want)
		}
	}
}

// configurationRecord is one record of the effective configuration the start logs.
type configurationRecord struct {
	Level  string `json:"level"`
	Msg    string `json:"msg"`
	Path   string `json:"path"`
	Source string `json:"source"`
	Value  any    `json:"value"`
}

// The start logs every effective value with the layer that set it, the one password file's path and
// never its contents, account_reload_interval among them with its default, which the chart leaves
// as it is.
func TestTheEffectiveConfigurationIsLogged(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("the-secret-itself\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	// Nothing listens on the database's port, so the start stops at the first job kind's build, after
	// the log.
	err := run(t, append([]string{
		"--database.host=127.0.0.1", "--database.port=1", "--database.user=mediated_mailbox_app", "--database.password_file=" + passwordFile,
		"--sync.sync_interval=2m",
	}, absentKeys...), []string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, &out, io.Discard)
	if err == nil || !strings.HasPrefix(err.Error(), "building the job kind backfill: ") {
		t.Fatalf("the start returned %v, want backfill's build to fail", err)
	}
	want := []string{
		`account_reload_interval default 1m0s`,
		`backfill.concurrency default 2`,
		`credential.private_key_files flag --credential.private_key_files [/absent/private]`,
		`credential.public_key_file flag --credential.public_key_file /absent/public`,
		`database.host flag --database.host 127.0.0.1`,
		`database.name environment variable MEDIATED_MAILBOX_DATABASE__NAME mailbox`,
		`database.password_file flag --database.password_file ` + passwordFile,
		`database.pool_size default 4`,
		`database.port flag --database.port 1`,
		`database.sslmode default verify-full`,
		`database.sslrootcert default `,
		`database.user flag --database.user mediated_mailbox_app`,
		`log_level default info`,
		`probe_listen default :8080`,
		`reload_interval default 1m0s`,
		`scanner.window default 8`,
		`sync.concurrency default 4`,
		`sync.decisions_per_tick default 200`,
		`sync.first_window default 168h0m0s`,
		`sync.sync_interval flag --sync.sync_interval 2m0s`,
	}
	// Of the scanner's section, whose every value is logged the same way, the window stands for the
	// rest.
	var got []string
	for line := range strings.Lines(out.String()) {
		var r configurationRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("a line of standard output is not a JSON record: %q", line)
		}
		if r.Msg != "configuration" || strings.HasPrefix(r.Path, "scanner.") && r.Path != "scanner.window" {
			continue
		}
		got = append(got, fmt.Sprintf("%s %s %v", r.Path, r.Source, r.Value))
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the effective configuration logged (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}

// The start validates the credential section before it reads a key file.
func TestAnEmptyListOfPrivateKeysRefusesTheStart(t *testing.T) {
	err := start(t, slices.Concat(database, []string{"--credential.public_key_file=/absent/public", "--credential.private_key_files=[]"})...)
	want := "validating the configuration: credential.private_key_files names no file"
	if err == nil || err.Error() != want {
		t.Errorf("the start returned %v, want %q", err, want)
	}
}

// An account is not an argument. An argument that is not a flag refuses the start, an account and a
// flag written without its dashes alike, so no account can come from the command line (ADR-0078,
// ADR-0080).
func TestAnAccountArgumentRefusesTheStart(t *testing.T) {
	for _, arg := range []string{"one@example.com", "database.port=5433"} {
		err := start(t, slices.Concat(database, []string{arg})...)
		want := fmt.Sprintf("loading the configuration: argument %q: it is not a flag, and flags are written --path=VALUE", arg)
		if err == nil || err.Error() != want {
			t.Errorf("the start returned %v, want %q", err, want)
		}
	}
}

// D1's and D4's parts of VERIFICATIONS' row for a credential supplied outside the account's state
// row. No configuration value holds an account or a credential, so an argument, a flag or an
// environment variable naming one refuses the start rather than being read (ADR-0080).
func TestACredentialInTheConfigurationRefusesTheStart(t *testing.T) {
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
	}{
		{"an account argument", []string{"one@example.com"}, nil},
		{"a flag", []string{"--accounts.personal.refresh_token=a-token"}, nil},
		{"an environment variable", nil, []string{"MEDIATED_MAILBOX_ACCOUNTS__PERSONAL__REFRESH_TOKEN=a-token"}},
		{"a credential flag", []string{"--credential.refresh_token=a-token"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := run(t, slices.Concat(database, absentKeys, c.args), c.environ, io.Discard, io.Discard)
			if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
				t.Errorf("the start returned %v, want the configuration library's refusal", err)
			}
		})
	}
}

// A configuration file holding a key the worker does not read refuses the start, so a typo or a key
// of another program never passes unread (ADR-0078).
func TestAnUnknownKeyInTheFileRefusesTheStart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "worker.yaml")
	if err := os.WriteFile(file, []byte("provider_timeout: 30s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := start(t, slices.Concat(database, absentKeys, []string{"--config-file=" + file})...)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") || !strings.Contains(err.Error(), "provider_timeout") {
		t.Errorf("the start returned %v, want the configuration library's refusal naming the key", err)
	}
}

// The database driver's password variables refuse the start before the configuration is read, since a
// stray one would win over the mounted password file (ADR-0078).
func TestADriversPasswordVariableRefusesTheStart(t *testing.T) {
	for _, v := range []string{"PGPASSWORD=a-password", "PGSSLPASSWORD="} {
		err := run(t, slices.Concat(database, absentKeys), []string{v}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), strings.TrimSuffix(strings.SplitN(v, "=", 2)[0], "=")) {
			t.Errorf("a start with %s returned %v, want it refused naming the variable", v, err)
		}
	}
}

// D1's part of VERIFICATIONS' row for a configuration mistake. The scanner's section is validated by
// the scanner itself before anything is built, so a value outside its designed range refuses the
// start (ADR-0078, ADR-0005).
func TestAnInvalidScannerSectionRefusesTheStart(t *testing.T) {
	for _, c := range []struct{ arg, want string }{
		{"--scanner.window=0", "the window, the dense length and the dense entropy must be positive"},
		{"--scanner.subject_threshold=0.9", "the subject threshold is above the body threshold"},
		{"--scanner.link_words=[]", "no link word or no link parameter is configured"},
	} {
		t.Run(c.arg, func(t *testing.T) {
			err := start(t, slices.Concat(database, absentKeys, []string{c.arg})...)
			if err == nil || !strings.HasPrefix(err.Error(), "validating the configuration: scanner: ") || !strings.Contains(err.Error(), c.want) {
				t.Errorf("the start returned %v, want the scanner's refusal containing %q", err, c.want)
			}
		})
	}
}

// errBuilt stops a start once the scanner the test reads has been built.
var errBuilt = errors.New("the scanner was built")

// scannerOf runs a process over the worker's configuration whose one job kind gets the process's
// scanner, as each of the worker's job kinds gets it, and returns that scanner. The process loads the
// configuration from args and environ as the worker's start does, so the scanner is built from the
// worker's scanner section under the revision the configuration library computed for it.
func scannerOf(t *testing.T, args, environ []string) scan.Scanner {
	t.Helper()
	var got scan.Scanner
	spec := process.Spec[config.Worker]{Name: "worker", Jobs: []process.Job[config.Worker]{{
		Name: "backfill",
		Build: func(ctx context.Context, p process.Capabilities, c config.Worker) (schedule.Plan, error) {
			s, err := scanner.Get(ctx, p, c)
			if err != nil {
				return schedule.Plan{}, err
			}
			got = s
			return schedule.Plan{}, errBuilt
		},
	}}}
	err := process.RunContext(t.Context(), slices.Concat(database, absentKeys, args), environ, io.Discard, io.Discard, spec)
	if !errors.Is(err, errBuilt) {
		t.Fatalf("the start returned %v, want it to stop once the scanner was built", err)
	}
	return got
}

// The scanner the worker's job kinds scan with decides under its section's values, not the defaults
// the application ships, so the section validated at start is the one the scanner runs on. A trigger
// list naming no word of the body leaves the trigger window's rule unfired on a code it fires on by
// default (ADR-0005, ADR-0078).
func TestTheScannerIsBuiltFromItsSection(t *testing.T) {
	rules := func(t *testing.T, args ...string) []string {
		t.Helper()
		return scannerOf(t, args, nil).Scan("Your code is 419283").Rules()
	}
	if got := rules(t); !slices.Contains(got, scan.RuleTriggerWindow) {
		t.Fatalf("under the default section the rules are %v, want %s among them", got, scan.RuleTriggerWindow)
	}
	if got := rules(t, "--scanner.triggers.en=[xyzzy]"); slices.Contains(got, scan.RuleTriggerWindow) {
		t.Errorf("with no trigger word of the body configured the rules are %v, want no %s", got, scan.RuleTriggerWindow)
	}
}

// D1's part of VERIFICATIONS' row for a section's revision. The revision a verdict and a mask record
// follows the worker's scanner section, whichever layer changed it, and a change to another section,
// either job kind's own values and the Session's reload interval included, leaves it as it was
// (ADR-0078, ADR-0005, ADR-0120).
func TestAVerdictsRevisionFollowsTheScannerSection(t *testing.T) {
	file := filepath.Join(t.TempDir(), "work.yaml")
	if err := os.WriteFile(file, []byte("scanner:\n  window: 9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	revision := func(t *testing.T, args, environ []string) string {
		t.Helper()
		return scannerOf(t, args, environ).Scan("Your code is 419283").Revision()
	}
	base := revision(t, nil, nil)
	if base == "" {
		t.Fatal("the default configuration's verdict records no revision")
	}
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
		changed bool
	}{
		{"a flag changing the scanner", []string{"--scanner.window=9"}, nil, true},
		{"an environment variable changing the scanner", nil, []string{"MEDIATED_MAILBOX_SCANNER__TRIGGERS__DE=[code]"}, true},
		{"the file changing the scanner", []string{"--config-file=" + file}, nil, true},
		{"a flag restating the scanner's default", []string{"--scanner.window=8"}, nil, false},
		{"a flag changing another section", []string{"--database.port=5433"}, nil, false},
		{"a flag changing the probes", []string{"--probe_listen=:9090"}, nil, false},
		{"a flag changing the sync interval", []string{"--sync.sync_interval=1m"}, nil, false},
		{"a flag changing a tick's decisions", []string{"--sync.decisions_per_tick=5"}, nil, false},
		{"a flag changing backfill's concurrency", []string{"--backfill.concurrency=3"}, nil, false},
		{"a flag changing the Session's reload interval", []string{"--account_reload_interval=5m"}, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := revision(t, c.args, c.environ); (got != base) != c.changed {
				t.Errorf("the revision is %q against the default's %q, want it changed %v", got, base, c.changed)
			}
		})
	}
}

// D2's and D4's part of the row for the scanner backfill and delta sync share. Both job kinds scan
// with the process's one scanner, built from the worker's one scanner section, and each job kind's own
// test holds its gate to the shared thresholds and its scans to the scanner it is given. The default
// section's scanner records the version and revision written out here, which both job kinds' verdicts
// and masks therefore record (ADR-0120, ADR-0121, ADR-0104).
//
// The revision written out here is also a known answer across releases, computed once by an earlier
// build from the default section loaded as the worker's start loads it, with no environment, and
// never derived by this test. The revision hashes the section's YAML rendering, so a field added
// with a default that keeps today's behaviour, a changed YAML name, a change in how the YAML library
// renders a value this section holds or a change to the configuration library's revision algorithm
// changes it with nothing the scanner does changed (ADR-0078). A change in how the library renders
// only forms this section does not hold is left to review of the library's update. Changing this
// value re-masks and re-scans every stored subject and verdict at the next release, with the bodies
// of the messages returned to pending denied until the second pass scans them again (ADR-0120). So
// it changes only as a decision taken in review, never to make this test pass.
func TestTheScannerSectionBuildsTheSharedScanner(t *testing.T) {
	v := scannerOf(t, nil, nil).Scan("Your code is 419283")
	const shared = "version 1 revision c9fa3ff13948e4f89ac094b91a27455c"
	if got := fmt.Sprintf("version %d revision %s", v.Version(), v.Revision()); got != shared {
		t.Errorf("the scanner records %s, want %s", got, shared)
	}
}

// The start writes a banner naming the worker, its release version and where its probes listen to
// standard error, and standard output carries only JSON records, the banner never among them
// (ADR-0122). The key files do not exist, so the start stops at the first job kind's build, after
// the banner and before any connection is made.
func TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(t, slices.Concat([]string{"--database.host=db", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent"}, absentKeys), nil, &stdout, &stderr)
	if err == nil || !strings.HasPrefix(err.Error(), "building the job kind backfill: ") {
		t.Fatalf("the start returned %v, want it to stop at backfill's build", err)
	}
	if want := "mediated-mailbox-worker " + version.Number + "\n  probes  http on :8080\n"; stderr.String() != want {
		t.Errorf("the banner is\n%s\nwant\n%s", stderr.String(), want)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("the start logged nothing")
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("standard output holds a line that is not a JSON record: %q", line)
		}
	}
}

// --version prints the release version and exits before any layer of the configuration is read, so
// a flag the configuration would refuse beside it stops nothing, and no configuration is logged
// (ADR-0078).
func TestVersionPrintsTheReleaseVersionAndReadsNoConfiguration(t *testing.T) {
	var stdout bytes.Buffer
	if err := run(t, []string{"--version", "--no.such=1"}, nil, &stdout, io.Discard); err != nil {
		t.Fatalf("the start returned %v, want nil", err)
	}
	if want := version.Number + "\n"; stdout.String() != want {
		t.Errorf("the start printed %q, want only %q", stdout.String(), want)
	}
}

// VERIFICATIONS' row for the log level, for the worker on its spec. With log_level set to warn the
// effective configuration is still written, each value with its source, and no other record below
// warn is written (ADR-0122).
func TestALogLevelOfWarnWritesNoInfoRecordBeyondTheConfiguration(t *testing.T) {
	var stdout bytes.Buffer
	err := run(t, slices.Concat(database, absentKeys, []string{"--log_level=warn"}), nil, &stdout, io.Discard)
	if err == nil {
		t.Fatal("the start succeeded")
	}
	var configuration int
	for line := range strings.Lines(stdout.String()) {
		var r configurationRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("a line of standard output is not a JSON record: %q", line)
		}
		switch {
		case r.Msg == "configuration" && r.Source != "":
			configuration++
		case r.Level == "INFO" || r.Level == "DEBUG":
			t.Errorf("a record below warn was written: %s", line)
		}
	}
	if configuration == 0 {
		t.Error("the effective configuration was not written")
	}
}
