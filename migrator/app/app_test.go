package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/migrator/app"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// database is a database section the migrator validates, naming a password file that does not exist,
// so a start that gets past its validation stops at reading the password, before any connection.
var database = []string{"--database.host=db.example", "--database.name=mailbox", "--database.user=mailbox_migrator", "--database.password_file=/absent/password", "--application_user=mailbox_app"}

// --version prints the release version and exits before any layer of the configuration is read, so
// a flag the configuration would refuse beside it stops nothing, and no configuration is logged
// (ADR-0078).
func TestVersionPrintsTheReleaseVersionAndReadsNoConfiguration(t *testing.T) {
	var stdout bytes.Buffer
	err := process.RunContext(t.Context(), []string{"--version", "--no.such=1"}, nil, &stdout, io.Discard, app.Spec())
	if err != nil {
		t.Fatalf("the run returned %v, want nil", err)
	}
	if want := version.Number + "\n"; stdout.String() != want {
		t.Errorf("the run printed %q and logged nothing else, want %q", stdout.String(), want)
	}
}

// The migrator refuses to start while the database driver's password variables are set, before its
// configuration is read, so a stray one never wins over the mounted password file, as every process
// refuses it (ADR-0078).
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		var stdout bytes.Buffer
		err := process.RunContext(t.Context(), database, []string{name + "="}, &stdout, io.Discard, app.Spec())
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("with %s set, the run returned %v, want the start refused naming it", name, err)
		}
		if got := messages(t, stdout.String()); len(got) != 1 || got[0] != "the process failed" {
			t.Errorf("with %s set, the run logged %v, want only the refusal and no configuration before it", name, got)
		}
	}
}

// A start writes the effective configuration at info whatever log_level says, writes a banner naming
// the migrator and its release version to the writer main.go hands in as standard error, and keeps
// standard output to JSON records (ADR-0122). The password file does not exist, so the start stops at
// reading it, after the banner and before any connection is made. The migrator writes no record below
// warn of its own, since goose logs nothing unless it is made verbose, so no run of it shows the level
// holding a record back.
func TestTheStartLogsItsConfigurationThenSetsItsLevelAndWritesItsBanner(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := process.RunContext(t.Context(), append([]string{"--log_level=error"}, database...), nil, &stdout, &stderr, app.Spec())
	if err == nil || !strings.Contains(err.Error(), "reading the password file") {
		t.Fatalf("the run returned %v, want the start to stop at reading the password file", err)
	}
	if want := "mediated-mailbox-migrator " + version.Number + "\n"; stderr.String() != want {
		t.Errorf("the banner is %q, want %q", stderr.String(), want)
	}
	var logged []string
	for line := range strings.Lines(stdout.String()) {
		var record struct{ Msg, Path, Level string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("standard output holds a line that is not a JSON record: %q", line)
		}
		switch {
		case record.Msg == "configuration":
			logged = append(logged, record.Path)
		case record.Level != "ERROR":
			// Past the configuration only the refusal is written, at error.
			t.Errorf("the run wrote %q at %s, below the level log_level names", record.Msg, record.Level)
		}
	}
	if !strings.Contains(strings.Join(logged, " "), "log_level") || !strings.Contains(strings.Join(logged, " "), "database.password_file") {
		t.Errorf("the configuration logged is %v, want every effective value, log_level's included", logged)
	}
}

// A log level that names none, and a database section the driver would fill with its own defaults,
// are refused before the banner and before any connection.
func TestAnUnusableConfigurationIsRefused(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{append([]string{"--log_level=verbose"}, database...), "validating the configuration: "},
		{append([]string{"--database.sslmode=verify_full"}, database...), "validating the configuration: database.sslmode"},
		{[]string{"--database.name=mailbox", "--database.password_file=/absent/password", "--application_user=mailbox_app"}, "loading the configuration: "},
		// The application user's name has no default, since the project names no database user, and an
		// empty one would name no grantee (ADR-0143).
		{[]string{"--database.host=db.example", "--database.name=mailbox", "--database.user=mailbox_migrator", "--database.password_file=/absent/password"}, "loading the configuration: application_user is required"},
		{append([]string{"--application_user="}, database[:4]...), "validating the configuration: application_user is empty"},
		// Nor has the migrator's own user, which the deployment names (ADR-0143).
		{[]string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password", "--application_user=mailbox_app"}, "loading the configuration: database.user is required"},
	} {
		var stderr bytes.Buffer
		err := process.RunContext(t.Context(), c.args, nil, io.Discard, &stderr, app.Spec())
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("%v: the run returned %v, want an error starting %q", c.args, err, c.want)
		}
		if stderr.Len() != 0 {
			t.Errorf("%v: the run wrote the banner %q before refusing", c.args, stderr.String())
		}
	}
}

// messages returns the message of each JSON record in logs.
func messages(t *testing.T, logs string) []string {
	t.Helper()
	var got []string
	for line := range strings.Lines(logs) {
		var record struct{ Msg string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("the log holds a line that is not a JSON record: %q", line)
		}
		got = append(got, record.Msg)
	}
	return got
}
