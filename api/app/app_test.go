package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/api/app"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// required sets every value the API server's start needs, its token file, database section and key
// files, none of which exists, so a start that gets past its checks stops at the password file,
// before any connection is made.
var required = map[string]string{
	"token_file": "/absent/token", "database.host": "db.example", "database.name": "mailbox",
	"database.user": "mediated_mailbox_app", "database.password_file": "/absent/password",
	"credential.public_key_file": "/absent/public", "credential.private_key_files": "[/absent/private]",
}

// flags returns the flags setting values, in key order, with set in place of their own.
func flags(set map[string]string) []string {
	values := maps.Clone(required)
	maps.Copy(values, set)
	var out []string
	for _, k := range slices.Sorted(maps.Keys(values)) {
		out = append(out, "--"+k+"="+values[k])
	}
	return out
}

// run runs the API server's spec as its main.go does, and returns what it wrote to standard output
// and to standard error, with the run's error.
func run(t *testing.T, args, environ []string) (stdout, stderr string, err error) {
	t.Helper()
	var out, banner bytes.Buffer
	err = process.RunContext(t.Context(), args, environ, &out, &banner, app.Spec())
	return out.String(), banner.String(), err
}

// pastTheChecks is the error a start that passed every check stops with, reading the password file
// that does not exist before it connects.
const pastTheChecks = "reading the password file"

// --version prints the release version and exits before any layer of the configuration is read, so
// a flag the configuration would refuse beside it stops nothing, and no configuration is logged
// (ADR-0078).
func TestVersionPrintsTheReleaseVersionAndReadsNoConfiguration(t *testing.T) {
	stdout, _, err := run(t, []string{"--version", "--no.such=1"}, nil)
	if err != nil {
		t.Fatalf("the run returned %v, want nil", err)
	}
	if want := version.Number + "\n"; stdout != want {
		t.Errorf("the run printed %q and logged nothing else, want %q", stdout, want)
	}
}

// The API server refuses to start while the database driver's password variables are set, an empty
// one included, before its configuration is read, as every process refuses them (ADR-0078).
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		stdout, _, err := run(t, flags(nil), []string{name + "="})
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("with %s set, the run returned %v, want the start refused naming it", name, err)
		}
		if strings.Contains(stdout, `"msg":"configuration"`) {
			t.Errorf("with %s set, the configuration was read before the refusal:\n%s", name, stdout)
		}
	}
}

// The start writes every effective value at info whatever log_level says, sets the level from
// log_level only then, writes a banner naming the API server, its release version and where it serves
// to the writer main.go hands in as standard error, and keeps standard output to JSON records
// (ADR-0122). Under warn, no record at info follows the configuration.
func TestTheStartLogsItsConfigurationThenSetsItsLevelAndWritesItsBanner(t *testing.T) {
	for name, c := range map[string]struct {
		set        map[string]string
		infoBeyond bool
	}{
		"warn": {map[string]string{"log_level": "warn"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := run(t, flags(c.set), nil)
			if err == nil || !strings.Contains(err.Error(), pastTheChecks) {
				t.Fatalf("the run returned %v, want the start to stop at the password file", err)
			}
			if want := "mediated-mailbox-api " + version.Number + "\n  serves  http on :8443\n  probes  http on :8080\n"; stderr != want {
				t.Errorf("the banner is\n%s\nwant\n%s", stderr, want)
			}
			var configuration, infoBeyond int
			for line := range strings.Lines(stdout) {
				var record struct{ Msg, Level string }
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatalf("standard output holds a line that is not a JSON record: %q", line)
				}
				switch {
				case record.Msg == "configuration":
					configuration++
				case record.Level == "INFO":
					infoBeyond++
				}
			}
			if configuration == 0 {
				t.Error("the start wrote no effective value")
			}
			if (infoBeyond > 0) != c.infoBeyond {
				t.Errorf("the start wrote %d info records past the configuration, want some: %v\n%s", infoBeyond, c.infoBeyond, stdout)
			}
		})
	}
}

// The API server's start runs its configuration's check, config.API's, before it reads a key file or
// connects, so each value it cannot serve with refuses the start through the process, the process's
// own listeners among them, before the banner (ADR-0078, ADR-0127, ADR-0142).
func TestAConfigurationTheAPIServerCannotServeWithRefusesTheStart(t *testing.T) {
	both := "and the process serves TLS with both or plain HTTP with neither"
	for _, c := range []struct {
		name string
		set  map[string]string
		want string
	}{
		{"a certificate without its key", map[string]string{"tls_cert": "/tls/cert"}, "tls_cert is set and tls_key is not, " + both},
		{"a key beside a blank certificate path", map[string]string{"tls_cert": " ", "tls_key": "/tls/key"}, "tls_key is set and tls_cert is not, " + both},
		{"an empty probe_listen", map[string]string{"probe_listen": ""}, "probe_listen is empty"},
		{"a blank token file path", map[string]string{"token_file": " "}, "token_file is empty"},
		{"a reload interval of zero", map[string]string{"account_reload_interval": "0s"}, "account_reload_interval is not positive"},
		{"a provider timeout above five minutes", map[string]string{"provider_timeout": "5m1s"}, "provider_timeout is not positive and at most 5m0s"},
		{"an empty database host", map[string]string{"database.host": ""}, "database.host is empty"},
		{"a level naming none", map[string]string{"log_level": "verbose"}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, stderr, err := run(t, flags(c.set), nil)
			if want := "validating the configuration: " + c.want; err == nil || !strings.HasPrefix(err.Error(), want) || (c.want != "" && err.Error() != want) {
				t.Errorf("the run returned %v, want %q", err, want)
			}
			if stderr != "" {
				t.Errorf("the run wrote the banner %q before refusing", stderr)
			}
		})
	}
}

// The API server's defaults are core/config's, in which the token file, the database's host, name,
// user and password file and the key files are required, so a start that sets none of them is
// refused at the first, the token file, before anything is read or connected (ADR-0143).
func TestAStartWithNoRequiredValueIsRefused(t *testing.T) {
	_, _, err := run(t, nil, nil)
	want := "loading the configuration: token_file is required, and neither the file, MEDIATED_MAILBOX_TOKEN_FILE nor --token_file sets it"
	if err == nil || err.Error() != want {
		t.Errorf("the run returned %v, want %q", err, want)
	}
	args := flags(nil)
	args = slices.DeleteFunc(args, func(a string) bool { return strings.HasPrefix(a, "--database.user=") })
	_, _, err = run(t, args, nil)
	want = "loading the configuration: database.user is required, and neither the file, MEDIATED_MAILBOX_DATABASE__USER nor --database.user sets it"
	if err == nil || err.Error() != want {
		t.Errorf("the run returned %v, want %q", err, want)
	}
}

// An argument that is not a configuration flag is refused, so no account reaches the API server from
// its command line (ADR-0080).
func TestAnAccountArgumentIsRefused(t *testing.T) {
	_, _, err := run(t, append(flags(nil), "acct-a"), nil)
	if err == nil || !strings.Contains(err.Error(), "loading the configuration") {
		t.Errorf("the run returned %v, want the argument refused", err)
	}
}

// The start logs every effective value with the layer that set it, the password file's path and
// never its contents. Nothing listens at the database's address, so the start stops at its first
// connection.
func TestTheEffectiveConfigurationIsLogged(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("the-secret-itself\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := flags(map[string]string{"database.host": "127.0.0.1", "database.port": "1", "database.password_file": passwordFile})
	args = slices.DeleteFunc(args, func(a string) bool { return strings.HasPrefix(a, "--database.name=") })
	stdout, _, err := run(t, args, []string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"})
	if err == nil {
		t.Fatal("the run succeeded with nothing listening at the database's address")
	}
	var got []string
	for line := range strings.Lines(stdout) {
		var record struct {
			Msg, Path, Source string
			Value             any
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("standard output holds a line that is not a JSON record: %q", line)
		}
		if record.Msg == "configuration" {
			got = append(got, record.Path+" "+record.Source+" "+fmt.Sprint(record.Value))
		}
	}
	want := []string{
		"account_reload_interval default 1m0s",
		"credential.private_key_files flag --credential.private_key_files [/absent/private]",
		"credential.public_key_file flag --credential.public_key_file /absent/public",
		"database.host flag --database.host 127.0.0.1",
		"database.name environment variable MEDIATED_MAILBOX_DATABASE__NAME mailbox",
		"database.password_file flag --database.password_file " + passwordFile,
		"database.pool_size default " + strconv.Itoa(max(4, runtime.NumCPU())),
		"database.port flag --database.port 1",
		"database.sslmode default verify-full",
		"database.sslrootcert default ",
		"database.user flag --database.user mediated_mailbox_app",
		"listen default :8443",
		"log_level default info",
		"probe_listen default :8080",
		"provider_timeout default 30s",
		"scanner.dense_entropy default 3",
		"scanner.dense_length default 16",
		"scanner.link_params default [token code key auth t otp reset_password_token confirmation_token unlock_token oobcode verification_code confirmation_code ticket signature]",
		"scanner.link_words default [token confirm verify reset magic auth password login unlock]",
		"scanner.subject_threshold default 0.6",
		"scanner.threshold default 0.6",
		"scanner.triggers.en default [code otp verification verify pin passcode password 2fa two-factor two factor 2-factor one-time one time single-use security code login log in log-in sign-in sign in auth authenticate authentication secret access validate validation tan confirmation]",
		"scanner.weights.entropy default 1",
		"scanner.weights.length default 1",
		"scanner.weights.mix default 1",
		"scanner.weights.position default 1",
		"scanner.weights.proximity default 1",
		"scanner.window default 8",
		"tls_cert default ",
		"tls_key default ",
		"token_file flag --token_file /absent/token",
	}
	slices.Sort(got)
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the effective configuration (-want +got):\n%s", diff)
	}
	if strings.Contains(stdout, "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", stdout)
	}
}

// No configuration value holds an account or a credential, so a flag or an environment variable
// naming one refuses the start rather than being read (ADR-0080).
func TestACredentialInTheConfigurationRefusesTheStart(t *testing.T) {
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
	}{
		{"a flag", []string{"--accounts.personal.refresh_token=a-token"}, nil},
		{"an environment variable", nil, []string{"MEDIATED_MAILBOX_ACCOUNTS__PERSONAL__REFRESH_TOKEN=a-token"}},
		{"a credential flag", []string{"--credential.refresh_token=a-token"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := run(t, append(flags(nil), c.args...), c.environ)
			if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
				t.Errorf("the run returned %v, want the configuration library's refusal", err)
			}
		})
	}
}

// D3's part of VERIFICATIONS' row for a configuration value that would disable the gate, skip
// masking or weaken deny-by-default. No such value exists, so every layer naming one refuses the
// start, and the scanner's part, whose pattern tier the serve-time check runs, refuses a value that
// would leave the check nothing to match (ADR-0051, ADR-0002). config.API holds no field a safety
// disposition could be read from, so the gate and the check run as they do without any of these.
func TestNoConfigurationValueWeakensTheGate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "api.yaml")
	if err := os.WriteFile(file, []byte("redaction:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptied := filepath.Join(t.TempDir(), "scanner.yaml")
	if err := os.WriteFile(emptied, []byte("scanner:\n  triggers: {}\n  link_words: []\n  link_params: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
		want    string
	}{
		{"a flag disabling the gate", []string{"--gate.enabled=false"}, nil, "loading the configuration: "},
		{"a flag skipping masking", []string{"--masking=false"}, nil, "loading the configuration: "},
		{"a flag releasing restricted bodies", []string{"--release_restricted=true"}, nil, "loading the configuration: "},
		{"a flag skipping the serve-time check", []string{"--scanner.serve_time_check=false"}, nil, "loading the configuration: "},
		{"an environment variable allowing by default", nil, []string{"MEDIATED_MAILBOX_DEFAULT_ALLOW=true"}, "loading the configuration: "},
		{"a file switching redaction off", []string{"--config-file=" + file}, nil, "loading the configuration: "},
		{"a scanner part with nothing to match", []string{"--config-file=" + emptied}, nil, "validating the configuration: scanner"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := run(t, append(flags(nil), c.args...), c.environ)
			if err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Errorf("the run returned %v, want an error starting %q", err, c.want)
			}
		})
	}
}

// The API server refuses to start while MCPGODEBUG is set, even to an empty value, through its Init,
// which the process runs once the configuration's checks pass and before anything is built, so it
// never reaches the database, because the MCP SDK reads it to switch its behaviour (ADR-0086,
// ADR-0142).
func TestMCPGODEBUGStopsTheStart(t *testing.T) {
	for _, value := range []string{"allowsessionsinstateless=1", ""} {
		_, stderr, err := run(t, flags(nil), []string{"MCPGODEBUG=" + value})
		if err == nil || !strings.Contains(err.Error(), "MCPGODEBUG is set") {
			t.Errorf("with MCPGODEBUG=%q, the run returned %v", value, err)
		}
		if stderr != "" {
			t.Errorf("with MCPGODEBUG=%q, the run wrote the banner %q before refusing", value, stderr)
		}
	}
}
